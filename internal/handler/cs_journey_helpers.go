package handler

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_journey_helpers.go — enum/label & hitung lama-di-fase untuk Customer
// Journey / Lifecycle (Modul 6, 6.2 — BL-77). Murni Go (tanpa HTTP/DB call)
// agar diuji langsung. Meniru cs_impl_tasks_helpers.go.
//
// Pemetaan DB→view (KPI, funnel fase, baris tabel) & helper terkait di
// cs_journey_view.go.

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
