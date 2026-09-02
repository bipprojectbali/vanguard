package handler

import (
	"strconv"
	"strings"
)

// customer_success_form.go — parsing & validasi form Customer Success
// (parseCustomerSuccessForm + optScore), dipisah dari customer_success_helpers.go
// agar tiap file di bawah ambang tipe Route/Handler (150). Definisi enum &
// customerSuccessForm tetap di customer_success_helpers.go (paket sama).

// parseCustomerSuccessForm membaca & memvalidasi form. Mengembalikan (form, "")
// bila sah, atau (zero, kode) yang dipetakan customerSuccessErrMsg. SEMUA field
// opsional (kosong = NULL) — tak ada satu pun kolom wajib di v1, section yang
// aktor tak berhak isi memang harus boleh dikosongkan sepenuhnya.
func parseCustomerSuccessForm(fv func(string) string) (customerSuccessForm, string) {
	var f customerSuccessForm

	// health_status TIDAK diparse dari form (BL-24): status kini turunan
	// overall_health_score, di-set server-side di customer_success_save.go
	// (deriveHealthStatus). Field form apa pun diabaikan — badge read-only.
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
	// score_trend TAK diparse (BL-25): kini turunan riwayat skor (deriveScoreTrend,
	// customer_success_save.go), bukan input operator — nilai form apa pun diabaikan.

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
