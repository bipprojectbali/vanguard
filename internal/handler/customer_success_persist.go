package handler

import (
	"context"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_persist.go — hitung snapshot kesehatan (computeHealthSnapshot) +
// insert/update baris customer_success, dipisah dari customer_success_save.go (file
// health, ambang Route/Handler 150).

// csHealthComputed = kumpulan nilai kesehatan yang DIHITUNG (bukan dari form):
// overall_health_score + kapan dihitung + snapshot skor sebelumnya (untuk
// score_trend). Dibundel agar tak jadi enam parameter posisional yang mudah tertukar.
type csHealthComputed struct {
	overallScore         *int16
	healthLastCalculated pgtype.Timestamptz
	previousScore        *int16
	previousCalculatedAt pgtype.Timestamptz
}

// computeHealthSnapshot menghitung overall_health_score (rata-rata komponen
// non-NULL), health_status TURUNAN skor (BL-24, bukan input operator — form
// diabaikan, ikut nilai lama otomatis bila komponen di-mask ke nilai lama), dan
// score_trend TURUNAN riwayat skor (BL-25): geser skor LAMA → previous_* SEBELUM
// menyimpan nilai baru, HANYA saat section Health benar-benar ditulis kali ini
// (writeHealth) — bila di-mask ke nilai lama, pertahankan trend & previous_*
// existing (tak ada perubahan skor untuk dibandingkan). Snapshot pertama
// (existing.OverallHealthScore NULL) → trend NULL ("—", belum ada dasar).
func computeHealthSnapshot(form customerSuccessForm, existing db.CustomerSuccess, writeHealth bool) (customerSuccessForm, csHealthComputed) {
	overallScore := computeOverallHealthScore(form.AdoptionScore, form.EngagementScore, form.SupportScore, form.SentimentScore)
	form.HealthStatus = deriveHealthStatus(overallScore)

	healthLastCalculated := existing.HealthLastCalculated
	previousScore := existing.PreviousHealthScore
	previousCalculatedAt := existing.PreviousHealthCalculatedAt
	if writeHealth {
		form.ScoreTrend = deriveScoreTrend(existing.OverallHealthScore, overallScore, healthTrendDeadband)
		previousScore = existing.OverallHealthScore
		previousCalculatedAt = existing.HealthLastCalculated
		healthLastCalculated = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	} else {
		form.ScoreTrend = existing.ScoreTrend
	}

	return form, csHealthComputed{
		overallScore:         overallScore,
		healthLastCalculated: healthLastCalculated,
		previousScore:        previousScore,
		previousCalculatedAt: previousCalculatedAt,
	}
}

// createCustomerSuccessRow menyisipkan baris CS baru (jalur create). CreatedBy = aktor.
// usageDataSource DIOPER EKSPLISIT oleh caller (BL-27) — BUKAN bagian
// customerSuccessForm, karena form manual (customer_success_save.go) tak
// pernah mengubahnya; jalur sync (customer_success_sync.go) mengoper
// "Product Telemetry" secara sadar.
func (h *Handler) createCustomerSuccessRow(ctx context.Context, tenantID, accountID, uid int64, form customerSuccessForm, hc csHealthComputed, usageDataSource string) error {
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
		UsageDataSource:            usageDataSource,
		CreatedBy:                  &uid,
	})
	return err
}

// updateCustomerSuccessRow memperbarui baris CS existing (jalur update). UpdatedBy =
// aktor; AccountID = kunci baris (satu baris per desa). usageDataSource lihat
// komentar createCustomerSuccessRow.
func (h *Handler) updateCustomerSuccessRow(ctx context.Context, accountID, uid int64, form customerSuccessForm, hc csHealthComputed, usageDataSource string) error {
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
		UsageDataSource:            usageDataSource,
		UpdatedBy:                  &uid,
		AccountID:                  accountID,
	})
	return err
}
