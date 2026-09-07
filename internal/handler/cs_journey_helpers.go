package handler

import (
	"math"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_journey_helpers.go — enum/label, hitung lama-di-fase, dan pemetaan DB→view
// untuk Customer Journey / Lifecycle (Modul 6, 6.2 — BL-77). Murni Go (tanpa
// HTTP/DB call) agar diuji langsung. Meniru cs_impl_tasks_helpers.go.

const (
	// csJourneyStalledDays = ambang "macet": desa yang berada di fase lebih lama
	// dari ini ditandai (⚠️) di funnel & tabel utama. Ambang bisnis bernama,
	// bukan angka telanjang tersebar (aturan hardcode #15).
	csJourneyStalledDays = 90

	// csJourneyOnboardingLimit = batas baris panel "Onboarding Aktif" (guardrail
	// no full-scan; panel ringkas, bukan daftar penuh).
	csJourneyOnboardingLimit = 100

	// csJourneyExportLimit = batas baris ekspor CSV tabel utama. Pagar atas agar
	// ekspor tak memindai seluruh tabel tanpa batas; ruang kerja nyata jauh di
	// bawah ini.
	csJourneyExportLimit = 5000
)

// csJourneyDate memformat tanggal untuk tampilan; kosong → "—".
func csJourneyDate(d pgtype.Date) string {
	if !d.Valid {
		return "—"
	}
	return d.Time.Format("2 Jan 2006")
}

// daysInStage menghitung lama-di-fase (hari kalender) dari stage_entry_date
// relatif today. Dihitung di Go SENGAJA (gotcha #14: ekspresi tanggal di SELECT
// list bikin sqlc emit interface{}). (0,false) bila tanggal kosong → view
// menampilkan "—". Tanggal masa depan (negatif) dipangkas ke 0.
func daysInStage(entry pgtype.Date, today time.Time) (int, bool) {
	if !entry.Valid {
		return 0, false
	}
	e := dateOnlyUTC(entry.Time)
	t := dateOnlyUTC(today)
	days := int(t.Sub(e).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days, true
}

// dateOnlyUTC menormalkan sebuah waktu ke tengah malam UTC pada tanggal
// kalendernya — dasar seragam pembanding hari (dan kunci keyset ASC yang
// mencerminkan COALESCE(stage_entry_date::timestamp AT TIME ZONE 'UTC', ...)).
func dateOnlyUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// csJourneyStageBadge memetakan lifecycle_stage → label + kelas badge daisyUI.
// Token semantik selaras arah perjalanan (Onboarding=info … Renewal=warning).
func csJourneyStageBadge(stage *string) (label, badge string) {
	if stage == nil || *stage == "" {
		return "—", "badge-ghost"
	}
	switch *stage {
	case "Onboarding":
		return *stage, "badge-info"
	case "Adoption":
		return *stage, "badge-primary"
	case "Retention":
		return *stage, "badge-success"
	case "Renewal":
		return *stage, "badge-warning"
	case "Advocacy":
		return *stage, "badge-secondary"
	default:
		return *stage, "badge-ghost"
	}
}

// csJourneyOnboardBadge memetakan onboarding_status → label + kelas badge.
// Kosong/nil → ("","badge-ghost"): view memperlakukan label kosong sebagai
// "belum ada onboarding" dan menampilkan "—".
func csJourneyOnboardBadge(status *string) (label, badge string) {
	if status == nil || *status == "" {
		return "", "badge-ghost"
	}
	switch *status {
	case "Not Started":
		return *status, "badge-ghost"
	case "In Progress":
		return *status, "badge-info"
	case "Completed":
		return *status, "badge-success"
	case "Stalled":
		return *status, "badge-error"
	default:
		return *status, "badge-ghost"
	}
}

// csJourneyProgressInt mengambil onboarding_progress (*int16) → int; nil → 0.
func csJourneyProgressInt(p *int16) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

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
