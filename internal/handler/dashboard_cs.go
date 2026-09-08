package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_cs.go — BL-59c + BL-98: section domain "Customer Success" pada
// Beranda (Modul 1). Komposisi-per-izin; heading tampil hanya bila ≥1 KPI tampil.
//
// BL-98 (ramping): section kini MAKSIMUM 2 KPI — Desa Berisiko (crm:health) &
// Adoption Rate (crm:adoption) — TANPA chart domain. KPI Engagement Jatuh Tempo &
// chart Progres Onboarding dipindah ke CS Report (ditautkan "Lihat Laporan →").
// REUSE agregasi Report 8.2 (BL-45): ReportCSHealth (band berisiko) & ReportCSAdoption
// (rata adopsi). F3 via AccountsListFilterFor. F4: murni skor/persen (tanpa Rp).
//
// §5 (project CLAUDE.md): Distribusi Health SENGAJA global/ungated (dashboard.go,
// dashboardGlobalCharts) — terbuka untuk SEMUA role. Section ini TAK menduplikasi
// donut itu; crm:health di sini hanya menambah angka "Desa Berisiko".
func (h *Handler) dashCSDomain(ctx context.Context, dataScope string, uid int64) (panel.DashDomain, bool, error) {
	d := panel.DashDomain{Title: "Customer Success"}
	q := h.q(ctx)
	acc := db.AccountsListFilterFor(dataScope)

	if canReadCSHealth(ctx) {
		// Desa Berisiko = band Berisiko (40–59) + Kritis (<40). Distribusi penuh
		// tetap di donut GLOBAL (§5) — di sini hanya angka agar tak menduplikasi.
		hr, err := q.ReportCSHealth(ctx, db.ReportCSHealthParams{
			ScopeAll: acc.ScopeAll, IsCsm: acc.IsOwn, IsSales: acc.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Desa Berisiko", Value: strconv.FormatInt(hr.BandAtRisk+hr.BandCritical, 10),
			ValueClass: "text-error",
		})
	}

	if canReadCSAdoption(ctx) {
		ar, err := q.ReportCSAdoption(ctx, db.ReportCSAdoptionParams{
			ScopeAll: acc.ScopeAll, IsCsm: acc.IsOwn, IsSales: acc.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		// csPctStr → "—" bila belum ada desa berskor (reuse reports_cs_panels.go).
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Adoption Rate", Value: csPctStr(ar.AvgAdoption, ar.Scored),
			ValueClass: "text-primary",
		})
	}

	// BL-98: tautan CS Report — HANYA bila role ber-crm:reports.
	if canViewReports(ctx) {
		d.ReportPath = wsPathOf(ctx, "/reports/customer-success")
	}
	return d, len(d.KPIs) > 0, nil
}
