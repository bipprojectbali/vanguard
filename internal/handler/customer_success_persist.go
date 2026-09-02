package handler

import (
	"context"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_persist.go — insert/update baris customer_success dipisah dari
// customer_success_save.go (file health, ambang Route/Handler 150). Nilai kesehatan
// TURUNAN (score/status/trend + jejak previous_*) dihitung di handler save lalu
// dioper lewat csHealthComputed; helper ini hanya menuliskan, tak menghitung.

// csHealthComputed = kumpulan nilai kesehatan yang DIHITUNG handler save (bukan dari
// form): overall_health_score + kapan dihitung + snapshot skor sebelumnya (untuk
// score_trend). Dibundel agar tak jadi enam parameter posisional yang mudah tertukar.
type csHealthComputed struct {
	overallScore         *int16
	healthLastCalculated pgtype.Timestamptz
	previousScore        *int16
	previousCalculatedAt pgtype.Timestamptz
}

// createCustomerSuccessRow menyisipkan baris CS baru (jalur create). CreatedBy = aktor.
func (h *Handler) createCustomerSuccessRow(ctx context.Context, tenantID, accountID, uid int64, form customerSuccessForm, hc csHealthComputed) error {
	_, err := h.q(ctx).CreateCustomerSuccess(ctx, db.CreateCustomerSuccessParams{
		TenantID:                   tenantID,
		AccountID:                  accountID,
		OverallHealthScore:         hc.overallScore,
		HealthStatus:               form.HealthStatus,
		AdoptionScore:              form.AdoptionScore,
		EngagementScore:            form.EngagementScore,
		SupportScore:               form.SupportScore,
		SentimentScore:             form.SentimentScore,
		ScoreTrend:                 form.ScoreTrend,
		HealthLastCalculated:       hc.healthLastCalculated,
		PreviousHealthScore:        hc.previousScore,
		PreviousHealthCalculatedAt: hc.previousCalculatedAt,
		LifecycleStage:             form.LifecycleStage,
		StageEntryDate:             form.StageEntryDate,
		OnboardingStatus:           form.OnboardingStatus,
		KickoffDate:                form.KickoffDate,
		TargetGoLiveDate:           form.TargetGoLiveDate,
		ActualGoLiveDate:           form.ActualGoLiveDate,
		OnboardingProgress:         form.OnboardingProgress,
		LastLoginDate:              form.LastLoginDate,
		ActiveUsers:                form.ActiveUsers,
		LoginFrequency:             form.LoginFrequency,
		FeatureAdoptionRate:        form.FeatureAdoptionRate,
		KeyFeaturesUsed:            form.KeyFeaturesUsed,
		UsageTrend:                 form.UsageTrend,
		CreatedBy:                  &uid,
	})
	return err
}

// updateCustomerSuccessRow memperbarui baris CS existing (jalur update). UpdatedBy =
// aktor; AccountID = kunci baris (satu baris per desa).
func (h *Handler) updateCustomerSuccessRow(ctx context.Context, accountID, uid int64, form customerSuccessForm, hc csHealthComputed) error {
	_, err := h.q(ctx).UpdateCustomerSuccess(ctx, db.UpdateCustomerSuccessParams{
		OverallHealthScore:         hc.overallScore,
		HealthStatus:               form.HealthStatus,
		AdoptionScore:              form.AdoptionScore,
		EngagementScore:            form.EngagementScore,
		SupportScore:               form.SupportScore,
		SentimentScore:             form.SentimentScore,
		ScoreTrend:                 form.ScoreTrend,
		HealthLastCalculated:       hc.healthLastCalculated,
		PreviousHealthScore:        hc.previousScore,
		PreviousHealthCalculatedAt: hc.previousCalculatedAt,
		LifecycleStage:             form.LifecycleStage,
		StageEntryDate:             form.StageEntryDate,
		OnboardingStatus:           form.OnboardingStatus,
		KickoffDate:                form.KickoffDate,
		TargetGoLiveDate:           form.TargetGoLiveDate,
		ActualGoLiveDate:           form.ActualGoLiveDate,
		OnboardingProgress:         form.OnboardingProgress,
		LastLoginDate:              form.LastLoginDate,
		ActiveUsers:                form.ActiveUsers,
		LoginFrequency:             form.LoginFrequency,
		FeatureAdoptionRate:        form.FeatureAdoptionRate,
		KeyFeaturesUsed:            form.KeyFeaturesUsed,
		UsageTrend:                 form.UsageTrend,
		UpdatedBy:                  &uid,
		AccountID:                  accountID,
	})
	return err
}
