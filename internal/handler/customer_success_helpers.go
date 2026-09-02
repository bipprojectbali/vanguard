package handler

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_helpers.go — opsi enum (cermin CHECK migrasi 00019), tipe
// form tervalidasi (customerSuccessForm), & pemetaan kode → pesan (ok/err).
// Parsing form ada di customer_success_form.go; skoring & pemetaan view di
// customer_success_view.go — dipisah agar tiap file di bawah ambang tipe
// Route/Handler (150). Ketiganya satu paket: aturan "kosong = NULL / terisi =
// wajib sah" tetap punya SATU sumber enum di sini.

// Nilai enum sah — CERMIN persis CHECK constraint migrasi 00019. Urutan untuk
// dropdown di slice terpisah (map tak berurutan) + cek sinkron compile-time.
// validHealthStatuses DIHAPUS (BL-24): health_status turunan overall_health_score
// (deriveHealthStatus). validScoreTrends DIHAPUS (BL-25): score_trend turunan
// riwayat skor (deriveScoreTrend) — keduanya tak lagi diparse dari form; nilai
// sah tetap ditegakkan CHECK cs_health_status_chk / cs_score_trend_chk (00019).
var (
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

// healthStatusOptions DIHAPUS (BL-24) & scoreTrendOptions DIHAPUS (BL-25):
// kedua dropdown diganti badge read-only (nilai turunan, bukan input operator).
var (
	lifecycleStageOptions   = []string{"Onboarding", "Adoption", "Retention", "Renewal", "Advocacy"}
	onboardingStatusOptions = []string{"Not Started", "In Progress", "Completed", "Stalled"}
	loginFrequencyOptions   = []string{"Daily", "Weekly", "Monthly", "Rarely", "Inactive"}
	usageTrendOptions       = []string{"Increasing", "Stable", "Decreasing"}
)

// compile-time: pastikan opsi & map validasi sepakat (panjang sama). Berbeda =
// dropdown menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(lifecycleStageOptions) != len(validLifecycleStages) ||
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
	case "score":
		return "Skor harus bilangan bulat 0–100."
	case "lifecycle_stage":
		return "Tahap siklus hidup tidak dikenal."
	case "onboarding_status":
		return "Status onboarding tidak dikenal."
	case "login_frequency":
		return "Frekuensi login tidak dikenal."
	case "usage_trend":
		return "Tren penggunaan tidak dikenal."
	case errOnboardingLifecycleMismatch:
		return "Tahap siklus hidup sudah melewati Onboarding, tetapi status onboarding belum \"Completed\". Selesaikan onboarding dulu atau kembalikan tahap ke Onboarding."
	case errOnboardingGoLiveMismatch:
		return "Status onboarding \"Not Started\", tetapi Tanggal Go-Live Aktual sudah terisi. Perbarui status onboarding atau kosongkan tanggal go-live."
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
