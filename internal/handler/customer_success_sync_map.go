package handler

import (
	"strconv"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/desaplus"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_sync_map.go — fungsi MURNI (tanpa DB/HTTP) pemetaan
// respons desa-plus → field customer_success, dipisah dari
// customer_success_sync.go agar unit-testable langsung tanpa DB/mock HTTP
// (mirror pola computeHealthSnapshot di customer_success_persist.go).

// formFromCustomerSuccess membangun customerSuccessForm dari baris existing
// APA ADANYA — tipe field customerSuccessForm & db.CustomerSuccess SENGAJA
// identik untuk bagian yang tumpang tindih (jadi salin langsung, tanpa
// konversi lain). Dipakai jalur sync (BL-27) yang TAK PERNAH menyentuh
// Health/Lifecycle/Onboarding/field manual Adoption lain — hanya 3 field
// yang benar-benar berasal dari desa-plus (caller menimpanya setelah ini
// lewat mapDesaPlusSummary).
func formFromCustomerSuccess(cs db.CustomerSuccess) customerSuccessForm {
	return customerSuccessForm{
		HealthStatus:    cs.HealthStatus,
		AdoptionScore:   cs.AdoptionScore,
		EngagementScore: cs.EngagementScore,
		SupportScore:    cs.SupportScore,
		SentimentScore:  cs.SentimentScore,
		ScoreTrend:      cs.ScoreTrend,

		LifecycleStage: cs.LifecycleStage,
		StageEntryDate: cs.StageEntryDate,

		OnboardingStatus:   cs.OnboardingStatus,
		KickoffDate:        cs.KickoffDate,
		TargetGoLiveDate:   cs.TargetGoLiveDate,
		ActualGoLiveDate:   cs.ActualGoLiveDate,
		OnboardingProgress: cs.OnboardingProgress,

		LastLoginDate:       cs.LastLoginDate,
		ActiveUsers:         cs.ActiveUsers,
		LoginFrequency:      cs.LoginFrequency,
		FeatureAdoptionRate: cs.FeatureAdoptionRate,
		KeyFeaturesUsed:     cs.KeyFeaturesUsed,
		UsageTrend:          cs.UsageTrend,
	}
}

// mapDesaPlusSummary memetakan VillageSummary → 3 field Adoption/Usage yang
// punya padanan API (last_login_date/active_users/key_features_used).
// existing dipakai sbg FALLBACK (bukan ditimpa NULL/kosong) saat API tak
// punya data utk field itu di panggilan ini — mis. lastActivity null tak
// mesti berarti "belum pernah ada aktivitas apa pun"; menimpa ke NULL
// berisiko meregresi tanggal yang sudah benar dari sync sebelumnya.
// activeUsers TAK PAKAI fallback — API SELALU mengirim angka (0 = valid,
// "memang nol pengguna aktif", bukan "data hilang").
func mapDesaPlusSummary(existing db.CustomerSuccess, sum *desaplus.VillageSummary) (lastLogin pgtype.Date, activeUsers *int32, keyFeatures *string) {
	lastLogin = existing.LastLoginDate
	if y, m, d, ok := sum.LastActivity.Date(); ok {
		lastLogin = pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
	}

	au := sum.ActiveUsers
	activeUsers = &au

	keyFeatures = existing.KeyFeaturesUsed
	if formatted := formatTopFeatures(sum.TopFeatures); formatted != "" {
		keyFeatures = &formatted
	}
	return lastLogin, activeUsers, keyFeatures
}

// formatTopFeatures memformat topFeatures desa-plus jadi teks ringkas untuk
// key_features_used, mis. "presensi (12x), surat (8x)". String kosong bila
// tak ada data (caller mapDesaPlusSummary lalu fallback ke nilai existing).
func formatTopFeatures(features []desaplus.TopFeature) string {
	if len(features) == 0 {
		return ""
	}
	parts := make([]string, 0, len(features))
	for _, f := range features {
		parts = append(parts, f.Feature+" ("+strconv.Itoa(int(f.Count))+"x)")
	}
	return strings.Join(parts, ", ")
}
