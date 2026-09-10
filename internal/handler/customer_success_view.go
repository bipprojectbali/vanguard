package handler

import (
	"context"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// customer_success_view.go — pemetaan baris CS → view (customerSuccessDetailView,
// customerSuccessFormFields), dipisah dari customer_success_scoring.go (skoring &
// derivasi) agar tiap file di bawah ambang tipe Route/Handler (150). Definisi enum
// & form tetap di customer_success_helpers.go.

// customerSuccessDetailView merakit data BACA lengkap + terapkan F2 per-section
// (section yang tak berhak dibaca disembunyikan di view lewat flag CanReadX, nilai
// mentahnya TETAP dioper — bukan PII per-baris seperti F4 phone, jadi tak perlu
// disamar di sini, cukup tak dirender). exists=false → seluruh field kosong ("—").
func customerSuccessDetailView(
	ctx context.Context, base string, a db.Account, cs db.CustomerSuccess, exists, hasLiveSub bool,
) panel.CustomerSuccessDetailView {
	// BL-26: peringatan keselarasan onboarding↔lifecycle (K2/K4) HANYA bila
	// aktor berhak membaca section Journey — jangan bocorkan keadaan section
	// yang disembunyikan F2. Banner tak relevan sebelum baris ada (exists=false).
	var warnings []string
	if exists && canReadCSJourney(ctx) {
		warnings = onboardingConsistencyWarnings(cs)
	}
	// BL-114: Customer Success (Health/Journey/Adoption) hanya dikelola untuk
	// PELANGGAN — desa dgn ≥1 langganan hidup (Trial/Active/Suspended). Prospek
	// (tanpa langganan) & churned (langganan mati semua) → penyuntingan dikunci
	// walau F2 memberi izin tulis. WriteLockNote menjelaskan penguncian HANYA saat
	// aktor sebenarnya berhak menulis (jangan bocorkan "kau bisa edit" ke read-only).
	canWriteBase := canWriteCS(ctx) && !IsReadOnly(ctx)
	var writeLockNote string
	if canWriteBase && !hasLiveSub {
		writeLockNote = "Desa ini belum menjadi pelanggan aktif (tidak ada langganan hidup). Health Score & Customer Success hanya dikelola untuk pelanggan — mulai langganan dulu untuk membuka penyuntingan."
	}
	return panel.CustomerSuccessDetailView{
		Base:          base,
		ID:            a.ID,
		AccountName:   a.VillageName,
		CanWrite:      canWriteBase && hasLiveSub,
		WriteLockNote: writeLockNote,
		Exists:        exists,
		Warnings:      warnings,

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
		AdoptionScore:   probabilityStr(cs.AdoptionScore),
		EngagementScore: probabilityStr(cs.EngagementScore),
		SupportScore:    probabilityStr(cs.SupportScore),
		SentimentScore:  probabilityStr(cs.SentimentScore),

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
