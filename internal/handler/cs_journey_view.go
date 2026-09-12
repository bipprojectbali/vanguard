package handler

import (
	"math"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_journey_view.go — pemetaan DB→view (KPI, funnel fase, baris tabel) untuk
// Customer Journey / Lifecycle (Modul 6, 6.2 — BL-77), plus helper href/keyset/
// filter terkait. Label/enum murni & hitung lama-di-fase di cs_journey_helpers.go.

// csJourneyKPIView memetakan CountCSJourneyKPIs → CSJourneyKPIs view (float rata
// dibulatkan ke hari terdekat).
func csJourneyKPIView(k db.CountCSJourneyKPIsRow) panel.CSJourneyKPIs {
	return panel.CSJourneyKPIs{
		OnboardingCount:   int(k.OnboardingCount),
		OnboardingAvgDays: int(math.Round(k.OnboardingAvgDays)),
		AdoptionCount:     int(k.AdoptionCount),
		AdoptionMaxDays:   int(k.AdoptionMaxDays),
		RetentionCount:    int(k.RetentionCount),
		RenewalCount:      int(k.RenewalCount),
	}
}

// csJourneyPhaseViews melengkapi hasil agregat per-fase (yang hanya memuat fase
// ber-baris) menjadi 5 fase KANONIK terurut lifecycleStageOptions — fase kosong
// diisi nol. Pct = lebar bar relatif count MAKSIMUM antar-fase (funnel visual).
func csJourneyPhaseViews(rows []db.ListCSJourneyPhasesRow) []panel.CSJourneyPhaseRow {
	byStage := make(map[string]db.ListCSJourneyPhasesRow, len(rows))
	var maxCount int64
	for _, r := range rows {
		if r.Stage == nil {
			continue
		}
		byStage[*r.Stage] = r
		if r.VillageCount > maxCount {
			maxCount = r.VillageCount
		}
	}
	out := make([]panel.CSJourneyPhaseRow, 0, len(lifecycleStageOptions))
	for _, stage := range lifecycleStageOptions {
		r := byStage[stage] // zero value bila fase tak punya baris
		pct := 0
		if maxCount > 0 {
			pct = int(r.VillageCount * 100 / maxCount)
		}
		out = append(out, panel.CSJourneyPhaseRow{
			Stage:   stage,
			Count:   int(r.VillageCount),
			AvgDays: int(math.Round(r.AvgDays)),
			Stalled: int(r.StalledCount),
			Pct:     pct,
		})
	}
	return out
}

// csJourneyOnboardRowView memetakan satu baris ListCSJourneyOnboarding → view.
// Peringatan target lampau dihitung di Go (target_go_live_date < hari ini).
func csJourneyOnboardRowView(r db.ListCSJourneyOnboardingRow, slug string, today time.Time) panel.CSJourneyOnboardRow {
	label, badge := csJourneyOnboardBadge(r.OnboardingStatus)
	targetPassed := r.TargetGoLiveDate.Valid && dateOnlyUTC(r.TargetGoLiveDate.Time).Before(dateOnlyUTC(today))
	return panel.CSJourneyOnboardRow{
		AccountName:  r.AccountName,
		Progress:     csJourneyProgressInt(r.OnboardingProgress),
		StatusLabel:  label,
		StatusBadge:  badge,
		TargetGoLive: csJourneyDate(r.TargetGoLiveDate),
		TargetPassed: targetPassed,
		CSMName:      strFromPtr(r.CsmName),
		HrefDetail:   csJourneyDetailHref(slug, r.AccountID),
	}
}

// csJourneyAccountRowView memetakan satu baris ListCSJourneyAccounts → view
// tabel utama. Health dari health_status (turunan skor, BL-24) via
// healthScoreStatus. Lama-di-fase & tanda macet dihitung di Go.
func csJourneyAccountRowView(r db.ListCSJourneyAccountsRow, slug string, today time.Time) panel.CSJourneyAccountRow {
	stageLabel, stageBadge := csJourneyStageBadge(r.LifecycleStage)
	days, known := daysInStage(r.StageEntryDate, today)
	healthLabel, healthBadge := healthScoreStatus(r.HealthStatus)
	healthScore := ""
	if r.HealthStatus != nil && *r.HealthStatus != "" {
		healthScore = healthLabel
	}
	onbLabel, onbBadge := csJourneyOnboardBadge(r.OnboardingStatus)
	return panel.CSJourneyAccountRow{
		AccountName:        r.AccountName,
		StageLabel:         stageLabel,
		StageBadge:         stageBadge,
		StageEntry:         csJourneyDate(r.StageEntryDate),
		DaysInStage:        days,
		DaysKnown:          known,
		Stalled:            known && days > csJourneyStalledDays,
		HealthScore:        healthScore,
		HealthBadge:        healthBadge,
		Progress:           csJourneyProgressInt(r.OnboardingProgress),
		HasProgress:        r.OnboardingProgress != nil,
		OnboardStatusLabel: onbLabel,
		OnboardStatusBadge: onbBadge,
		CSMName:            strFromPtr(r.CsmName),
		HrefDetail:         csJourneyDetailHref(slug, r.AccountID),
	}
}

// csJourneyDetailHref = tautan aksi baris → detail Customer Success desa. View
// TAK merakit path sendiri (base dari handler); helper ini di handler.
func csJourneyDetailHref(slug string, accountID int64) string {
	return wsPath(slug, "/accounts/"+strconv.FormatInt(accountID, 10)+"/customer-success")
}

// csJourneyKeyOf = kunci keyset ASC untuk splitPage, mencerminkan PERSIS ORDER
// BY query: COALESCE(stage_entry_date @ UTC, created_at) lalu account_id. Selalu
// finit (stage_entry_date kosong → jatuh ke created_at) agar formatCursor tak
// kehilangan baris.
func csJourneyKeyOf(r db.ListCSJourneyAccountsRow) (pgtype.Timestamptz, int64) {
	at := r.CreatedAt
	if r.StageEntryDate.Valid {
		at = pgtype.Timestamptz{Time: dateOnlyUTC(r.StageEntryDate.Time), Valid: true}
	}
	return at, r.AccountID
}

// csJourneyFilterStage membaca & memvalidasi ?stage= terhadap lifecycleStageOptions.
// Nilai liar → "" (semua fase) — masukan URL yang bisa disunting tak boleh
// menggagalkan render, sama filosofi pageCursor.
func csJourneyFilterStage(raw string) string {
	if raw == "" {
		return ""
	}
	for _, s := range lifecycleStageOptions {
		if raw == s {
			return raw
		}
	}
	return ""
}
