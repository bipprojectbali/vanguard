package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_helpers.go — parsing form, opsi enum (cermin CHECK migrasi
// 00019), skor 0–100, label "N hari di tahap ini", & pemetaan model DB → view
// (BACA + SUNTING). Dipisah dari customer_success.go/customer_success_save.go
// agar aturan "kosong = NULL / terisi = wajib sah" & pemetaan tampilan punya
// SATU tempat — create & update (get-then-branch) tak boleh menerima nilai
// yang berbeda sahnya untuk kolom yang sama.

// Nilai enum sah — CERMIN persis CHECK constraint migrasi 00019. Urutan untuk
// dropdown di slice terpisah (map tak berurutan) + cek sinkron compile-time.
var (
	validHealthStatuses = map[string]struct{}{
		"Healthy": {}, "At-Risk": {}, "Critical": {},
	}
	validScoreTrends = map[string]struct{}{
		"Improving": {}, "Stable": {}, "Declining": {},
	}
	validLifecycleStages = map[string]struct{}{
		"Onboarding": {}, "Adoption": {}, "Retention": {}, "Renewal": {}, "Advocacy": {},
	}
	validOnboardingStatuses = map[string]struct{}{
		"Not Started": {}, "In Progress": {}, "Completed": {}, "Stalled": {},
	}
	validLoginFrequencies = map[string]struct{}{
		"Daily": {}, "Weekly": {}, "Monthly": {}, "Rarely": {}, "Inactive": {},
	}
	validUsageTrends = map[string]struct{}{
		"Increasing": {}, "Stable": {}, "Decreasing": {},
	}
)

var (
	healthStatusOptions     = []string{"Healthy", "At-Risk", "Critical"}
	scoreTrendOptions       = []string{"Improving", "Stable", "Declining"}
	lifecycleStageOptions   = []string{"Onboarding", "Adoption", "Retention", "Renewal", "Advocacy"}
	onboardingStatusOptions = []string{"Not Started", "In Progress", "Completed", "Stalled"}
	loginFrequencyOptions   = []string{"Daily", "Weekly", "Monthly", "Rarely", "Inactive"}
	usageTrendOptions       = []string{"Increasing", "Stable", "Decreasing"}
)

// compile-time: pastikan opsi & map validasi sepakat (panjang sama). Berbeda =
// dropdown menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(healthStatusOptions) != len(validHealthStatuses) ||
		len(scoreTrendOptions) != len(validScoreTrends) ||
		len(lifecycleStageOptions) != len(validLifecycleStages) ||
		len(onboardingStatusOptions) != len(validOnboardingStatuses) ||
		len(loginFrequencyOptions) != len(validLoginFrequencies) ||
		len(usageTrendOptions) != len(validUsageTrends) {
		panic("customer_success: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// customerSuccessForm = nilai form YANG SUDAH divalidasi, siap dipetakan ke
// Create/UpdateCustomerSuccessParams — SEBELUM masking F2 per-section (masking
// terjadi di customer_success_save.go, membaca baris existing). overall_health_score
// & health_last_calculated SENGAJA TAK ADA DI SINI: keduanya dihitung/di-set
// server-side (computeOverallHealthScore), form tak pernah mengirimnya.
type customerSuccessForm struct {
	// Health (6.1)
	HealthStatus    *string
	AdoptionScore   *int16
	EngagementScore *int16
	SupportScore    *int16
	SentimentScore  *int16
	ScoreTrend      *string

	// Lifecycle (6.2)
	LifecycleStage *string
	StageEntryDate pgtype.Date

	// Onboarding (6.2.1)
	OnboardingStatus   *string
	KickoffDate        pgtype.Date
	TargetGoLiveDate   pgtype.Date
	ActualGoLiveDate   pgtype.Date
	OnboardingProgress *int16

	// Adoption / Usage (6.4)
	LastLoginDate       pgtype.Date
	ActiveUsers         *int32
	LoginFrequency      *string
	FeatureAdoptionRate pgtype.Numeric
	KeyFeaturesUsed     *string
	UsageTrend          *string
}

// parseCustomerSuccessForm membaca & memvalidasi form. Mengembalikan (form, "")
// bila sah, atau (zero, kode) yang dipetakan customerSuccessErrMsg. SEMUA field
// opsional (kosong = NULL) — tak ada satu pun kolom wajib di v1, section yang
// aktor tak berhak isi memang harus boleh dikosongkan sepenuhnya.
func parseCustomerSuccessForm(fv func(string) string) (customerSuccessForm, string) {
	var f customerSuccessForm

	if s := strings.TrimSpace(fv("health_status")); s != "" {
		if _, ok := validHealthStatuses[s]; !ok {
			return customerSuccessForm{}, "health_status"
		}
		f.HealthStatus = &s
	}
	adoption, code := optScore(fv("adoption_score"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.AdoptionScore = adoption
	engagement, code := optScore(fv("engagement_score"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.EngagementScore = engagement
	support, code := optScore(fv("support_score"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.SupportScore = support
	sentiment, code := optScore(fv("sentiment_score"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.SentimentScore = sentiment
	if s := strings.TrimSpace(fv("score_trend")); s != "" {
		if _, ok := validScoreTrends[s]; !ok {
			return customerSuccessForm{}, "score_trend"
		}
		f.ScoreTrend = &s
	}

	if s := strings.TrimSpace(fv("lifecycle_stage")); s != "" {
		if _, ok := validLifecycleStages[s]; !ok {
			return customerSuccessForm{}, "lifecycle_stage"
		}
		f.LifecycleStage = &s
	}
	stageEntry, code := optDate(fv("stage_entry_date"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.StageEntryDate = stageEntry

	if s := strings.TrimSpace(fv("onboarding_status")); s != "" {
		if _, ok := validOnboardingStatuses[s]; !ok {
			return customerSuccessForm{}, "onboarding_status"
		}
		f.OnboardingStatus = &s
	}
	kickoff, code := optDate(fv("kickoff_date"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.KickoffDate = kickoff
	targetGoLive, code := optDate(fv("target_go_live_date"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.TargetGoLiveDate = targetGoLive
	actualGoLive, code := optDate(fv("actual_go_live_date"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.ActualGoLiveDate = actualGoLive
	onboardingProgress, code := optScore(fv("onboarding_progress"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.OnboardingProgress = onboardingProgress

	lastLogin, code := optDate(fv("last_login_date"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.LastLoginDate = lastLogin
	activeUsers, code := optInt32(fv("active_users"))
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.ActiveUsers = activeUsers
	if s := strings.TrimSpace(fv("login_frequency")); s != "" {
		if _, ok := validLoginFrequencies[s]; !ok {
			return customerSuccessForm{}, "login_frequency"
		}
		f.LoginFrequency = &s
	}
	rate, code := optNumeric(fv("feature_adoption_rate"), "feature_adoption_rate")
	if code != "" {
		return customerSuccessForm{}, code
	}
	f.FeatureAdoptionRate = rate
	f.KeyFeaturesUsed = optTrim(fv("key_features_used"))
	if s := strings.TrimSpace(fv("usage_trend")); s != "" {
		if _, ok := validUsageTrends[s]; !ok {
			return customerSuccessForm{}, "usage_trend"
		}
		f.UsageTrend = &s
	}

	return f, ""
}

// optScore mengurai skor 0–100 opsional (adoption/engagement/support/sentiment/
// onboarding_progress): kosong → (nil, ""); terisi wajib bilangan bulat DALAM
// 0–100 → (&v, ""); di luar itu → (nil, "score"). Batas dipaksa di sini SEBELUM
// DB (cermin CHECK cs_*_score_chk/cs_onboarding_progress_chk migrasi 00019).
// Lokal ke modul ini (bukan form_optional.go, yang sengaja netral-domain) karena
// khas skor CS — mirip optProbability (sales_format.go) tapi pesan galat sendiri.
func optScore(s string) (*int16, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	n, err := strconv.ParseInt(s, 10, 16)
	if err != nil || n < 0 || n > 100 {
		return nil, "score"
	}
	v := int16(n)
	return &v, ""
}

// computeOverallHealthScore = rata-rata KOMPONEN non-NULL (adoption/engagement/
// support/sentiment), dibulatkan. nil bila SEMUA komponen kosong (belum ada
// dasar hitung) — kolom tetap NULL, bukan 0 yang menyesatkan (0 berarti "skor
// terendah", bukan "belum dihitung"). Dipanggil handler SAJA (customer_success_
// save.go); form tak pernah mengirim overall_health_score sendiri (lihat komentar
// migrasi 00019: kolom disengaja app-computed, bukan generated, agar rumus bisa
// berubah tanpa DDL).
func computeOverallHealthScore(adoption, engagement, support, sentiment *int16) *int16 {
	var sum, n int
	for _, p := range []*int16{adoption, engagement, support, sentiment} {
		if p != nil {
			sum += int(*p)
			n++
		}
	}
	if n == 0 {
		return nil
	}
	avg := int16((sum + n/2) / n) // pembulatan biasa, bukan selalu ke bawah
	return &avg
}

// daysInStageLabel = selisih hari (kalender) HARI INI terhadap stage_entry_date,
// diformat "N hari di tahap ini". Kebalikan arah daysLeftLabel (subscriptions_
// renewals.go: hitung SISA hari ke depan) — di sini menghitung MUNDUR sejak
// masuk tahap. Kedua tanggal dinormalkan ke tanggal sipil (UTC midnight) agar
// bebas jam/zona.
func daysInStageLabel(now time.Time, entry pgtype.Date) string {
	if !entry.Valid {
		return "—"
	}
	a := time.Date(entry.Time.Year(), entry.Time.Month(), entry.Time.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	d := int(b.Sub(a).Hours() / 24)
	if d < 0 {
		d = 0 // stage_entry_date di masa depan (data janggal) — tampilkan 0, bukan negatif
	}
	return strconv.Itoa(d) + " hari di tahap ini"
}

// customerSuccessDetailView merakit data BACA lengkap + terapkan F2 per-section
// (section yang tak berhak dibaca disembunyikan di view lewat flag CanReadX, nilai
// mentahnya TETAP dioper — bukan PII per-baris seperti F4 phone, jadi tak perlu
// disamar di sini, cukup tak dirender). exists=false → seluruh field kosong ("—").
func customerSuccessDetailView(
	ctx context.Context, base string, a db.Account, cs db.CustomerSuccess, exists bool,
) panel.CustomerSuccessDetailView {
	return panel.CustomerSuccessDetailView{
		Base:        base,
		ID:          a.ID,
		AccountName: a.VillageName,
		CanWrite:    canWriteCS(ctx) && !IsReadOnly(ctx),
		Exists:      exists,

		CanReadHealth:   canReadCSHealth(ctx),
		CanReadJourney:  canReadCSJourney(ctx),
		CanReadAdoption: canReadCSAdoption(ctx),

		OverallHealthScore:   probabilityStr(cs.OverallHealthScore),
		HealthStatus:         deref(cs.HealthStatus),
		AdoptionScore:        probabilityStr(cs.AdoptionScore),
		EngagementScore:      probabilityStr(cs.EngagementScore),
		SupportScore:         probabilityStr(cs.SupportScore),
		SentimentScore:       probabilityStr(cs.SentimentScore),
		ScoreTrend:           deref(cs.ScoreTrend),
		HealthLastCalculated: dateTimeStr(cs.HealthLastCalculated),

		LifecycleStage:     deref(cs.LifecycleStage),
		StageEntryDate:     dateStr(cs.StageEntryDate),
		DaysInStage:        daysInStageLabel(time.Now(), cs.StageEntryDate),
		OnboardingStatus:   deref(cs.OnboardingStatus),
		KickoffDate:        dateStr(cs.KickoffDate),
		TargetGoLiveDate:   dateStr(cs.TargetGoLiveDate),
		ActualGoLiveDate:   dateStr(cs.ActualGoLiveDate),
		OnboardingProgress: probabilityStr(cs.OnboardingProgress),

		LastLoginDate:       dateStr(cs.LastLoginDate),
		ActiveUsers:         int32Str(cs.ActiveUsers),
		LoginFrequency:      deref(cs.LoginFrequency),
		FeatureAdoptionRate: numericStr(cs.FeatureAdoptionRate),
		KeyFeaturesUsed:     deref(cs.KeyFeaturesUsed),
		UsageTrend:          deref(cs.UsageTrend),
		UsageDataSource:     cs.UsageDataSource,
	}
}

// customerSuccessFormFields memetakan baris existing (atau zero-value, jalur
// create) → nilai prefill form. Section yang aktor tak berhak TULIS tetap
// diisi (view menyembunyikan kartunya via CanWriteX, sama pola dgn detail).
func customerSuccessFormFields(cs db.CustomerSuccess) panel.CustomerSuccessFormFields {
	return panel.CustomerSuccessFormFields{
		HealthStatus:    deref(cs.HealthStatus),
		AdoptionScore:   probabilityStr(cs.AdoptionScore),
		EngagementScore: probabilityStr(cs.EngagementScore),
		SupportScore:    probabilityStr(cs.SupportScore),
		SentimentScore:  probabilityStr(cs.SentimentScore),
		ScoreTrend:      deref(cs.ScoreTrend),

		LifecycleStage: deref(cs.LifecycleStage),
		StageEntryDate: dateStr(cs.StageEntryDate),

		OnboardingStatus:   deref(cs.OnboardingStatus),
		KickoffDate:        dateStr(cs.KickoffDate),
		TargetGoLiveDate:   dateStr(cs.TargetGoLiveDate),
		ActualGoLiveDate:   dateStr(cs.ActualGoLiveDate),
		OnboardingProgress: probabilityStr(cs.OnboardingProgress),

		LastLoginDate:       dateStr(cs.LastLoginDate),
		ActiveUsers:         int32Str(cs.ActiveUsers),
		LoginFrequency:      deref(cs.LoginFrequency),
		FeatureAdoptionRate: numericStr(cs.FeatureAdoptionRate),
		KeyFeaturesUsed:     deref(cs.KeyFeaturesUsed),
		UsageTrend:          deref(cs.UsageTrend),
	}
}

// customerSuccessMsg memetakan ?ok= → pesan sukses. Dipisah dari
// customerSuccessErrMsg agar alert sukses & galat tak pernah tertukar variannya.
func customerSuccessMsg(code string) string {
	switch code {
	case "created":
		return "Customer Success desa ini disimpan pertama kali."
	case "saved":
		return "Perubahan Customer Success disimpan."
	default:
		return ""
	}
}

// customerSuccessErrMsg memetakan ?err= → pesan galat form Customer Success.
// Lokal ke modul (tak menumpang wsErrMsg bersama) agar kode galat khas skor/
// enum CS terkumpul di satu tempat.
func customerSuccessErrMsg(code string) string {
	switch code {
	case "health_status":
		return "Status kesehatan harus salah satu: Healthy, At-Risk, atau Critical."
	case "score":
		return "Skor harus bilangan bulat 0–100."
	case "score_trend":
		return "Tren skor harus salah satu: Improving, Stable, atau Declining."
	case "lifecycle_stage":
		return "Tahap siklus hidup tidak dikenal."
	case "onboarding_status":
		return "Status onboarding tidak dikenal."
	case "login_frequency":
		return "Frekuensi login tidak dikenal."
	case "usage_trend":
		return "Tren penggunaan tidak dikenal."
	case "date":
		return "Format tanggal tidak sah."
	case "number":
		return "Jumlah pengguna aktif harus bilangan bulat non-negatif."
	case "feature_adoption_rate":
		return "Tingkat adopsi fitur harus berupa angka 0–100."
	case "failed":
		return "Gagal menyimpan Customer Success. Coba lagi."
	default:
		return ""
	}
}
