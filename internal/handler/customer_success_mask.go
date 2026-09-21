package handler

import "go_starter/internal/db"

// customer_success_mask.go — F2 masking per-section, DIPISAH agar bisa diuji
// LANGSUNG (unit test) tanpa HTTP. TAK ADA business_role bawaan yang punya akses
// tulis SEBAGIAN dari 3 section CS (business_policy.csv: tiap role tulis
// semua-atau-tiada — admin/manager/csm penuh, sales/support nol) — jalur masking
// karena itu tak bisa dilatih lewat peran nyata via HTTP; fungsi murni ini yang
// mengunci kontraknya (dipanggil CustomerSuccessSave DAN test).

// applyCustomerSuccessMasking mempertahankan field section yang TAK berhak
// ditulis aktor (writeHealth/writeJourney/writeAdoption false) dari baris
// existing, bukan menimpanya dengan form baru — form kosong pada section itu
// berarti "kartunya tak dirender" (view menyembunyikannya via CanWriteX), bukan
// "kosongkan section ini".
func applyCustomerSuccessMasking(
	form customerSuccessForm, existing db.CustomerSuccess,
	writeHealth, writeJourney, writeAdoption bool,
) customerSuccessForm {
	if !writeHealth {
		form.HealthStatus = existing.HealthStatus
		form.AdoptionScore = existing.AdoptionScore
		form.EngagementScore = existing.EngagementScore
		form.SupportScore = existing.SupportScore
		form.SentimentScore = existing.SentimentScore
		form.ScoreTrend = existing.ScoreTrend
	}
	if !writeJourney {
		form.LifecycleStage = existing.LifecycleStage
		form.StageEntryDate = existing.StageEntryDate
		form.OnboardingStatus = existing.OnboardingStatus
		form.KickoffDate = existing.KickoffDate
		form.TargetGoLiveDate = existing.TargetGoLiveDate
		form.ActualGoLiveDate = existing.ActualGoLiveDate
		form.OnboardingProgress = existing.OnboardingProgress
	}
	if !writeAdoption {
		form.LastLoginDate = existing.LastLoginDate
		form.ActiveUsers = existing.ActiveUsers
		form.LoginFrequency = existing.LoginFrequency
		form.FeatureAdoptionRate = existing.FeatureAdoptionRate
		form.KeyFeaturesUsed = existing.KeyFeaturesUsed
		form.UsageTrend = existing.UsageTrend
	} else if existing.UsageDataSource == "Product Telemetry" {
		// BL-27: last_login_date/active_users/key_features_used berasal dari sync
		// Desa+ (bukan section-write biasa) — form edit merendernya READ-ONLY
		// (customer_success_form.go), tapi form klien cuma jaring UX; masking di
		// sini adalah wasit sesungguhnya, cermin pola health_status/score_trend
		// (BL-24/25) yang juga diabaikan dari input & selalu server-derived.
		// login_frequency/feature_adoption_rate/usage_trend TETAP field manual
		// biasa (API desa-plus tak punya padanan untuk ketiganya).
		form.LastLoginDate = existing.LastLoginDate
		form.ActiveUsers = existing.ActiveUsers
		form.KeyFeaturesUsed = existing.KeyFeaturesUsed
	}
	return form
}
