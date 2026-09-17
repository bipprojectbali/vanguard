package handler

import (
	"context"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_cs.go — BL-59c + BL-98 + BL-143: section domain "Customer Success"
// pada Beranda (Modul 1). Komposisi-per-izin; heading tampil hanya bila ≥1
// KPI/chart tampil.
//
// BL-98 (ramping): section kini MAKSIMUM 2 KPI — Desa Berisiko (crm:health) &
// Adoption Rate (crm:adoption) — TANPA chart domain. KPI Engagement Jatuh Tempo &
// chart Progres Onboarding dipindah ke CS Report (ditautkan "Lihat Laporan →").
// REUSE agregasi Report 8.2 (BL-45): ReportCSHealth (band berisiko) & ReportCSAdoption
// (rata adopsi). F3 via AccountsListFilterFor. F4: murni skor/persen (tanpa Rp).
//
// §5 (project CLAUDE.md): Distribusi Health SENGAJA global/ungated (dashboard.go,
// dashboardGlobalCharts) — terbuka untuk SEMUA role. Section ini TAK menduplikasi
// donut itu; crm:health di sini hanya menambah angka "Desa Berisiko". Kedua chart
// BL-143 di bawah TAK menyentuh distribusi health sama sekali (beda sumbu:
// onboarding & beban-kerja CSM), jadi tak melanggar aturan ini.
//
// BL-143 (membalik BL-98 KHUSUS Beranda): tambah 2 chart inline.
//   - Progres Onboarding (pie, distribusi status): REUSE ReportOnboarding persis
//     query BL-59c pre-BL-98 (F3 identik ReportCSHealth: AccountsListFilterFor).
//     Digate canReadCSJourney(ctx) — MENGIKUTI PRESEDEN pre-BL-98 (bukan
//     crm:adoption yg disebut longgar di teks ticket) krn query & KPI-nya sejak
//     awal (BL-59c) memang di bawah objek Casbin "crm:journey", bukan
//     "crm:adoption"; menyamakan gate persis sumber query (keputusan sama pola
//     dgn BL-142: ikuti sumber F3/F2 aktual saat teks ticket ambigu).
//   - Engagement per CSM (bar, Total touch point per CSM): REUSE ReportEngagementByCSM
//     (Report 8.2 panel 6) + EngagementsListFilterFor (F3 identik
//     ReportEngagementCompliance, BUKAN AccountsListFilterFor — sumber datanya
//     tabel engagements, bukan accounts). Digate canViewEngagements(ctx)
//     (crm:engagements read) — SAMA gate dgn KPI "Engagement Jatuh Tempo" pre-
//     BL-98 di section ini (CountEngagementKPIs), bukan crm:health/crm:adoption
//     spt disebut longgar di teks ticket. Kategori nama CSM via ownerDisplay
//     (reuse reports_cs_panels.go, sama label "— (Tak ditugaskan)" dgn tabel
//     Report). Period/Segment kosong (unbounded) — konsisten dgn KPI Desa
//     Berisiko/Adoption Rate di section ini yg juga tak difilter periode.
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

	if canReadCSJourney(ctx) {
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

	// Chart: Progres Onboarding (pie, distribusi status) — gate crm:journey
	// (preseden pre-BL-98, sumber query SAMA; lihat komentar header file).
	if canReadCSJourney(ctx) {
		ob, err := q.ReportOnboarding(ctx, db.ReportOnboardingParams{
			Today:    reportTodayDate(time.Now().In(appTZ)),
			ScopeAll: acc.ScopeAll, IsCsm: acc.IsOwn, IsSales: acc.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.Charts = append(d.Charts, panel.DashChart{
			Title: "Progres Onboarding", ChartID: "chart-cs-onboarding",
			ChartJSON: h.marshalChart(pieOption("Onboarding",
				[]string{"Belum Mulai", "Berjalan", "Terhambat", "Selesai"},
				[]int64{ob.NotStarted, ob.InProgress, ob.Stalled, ob.Completed})),
		})
	}

	// Chart: Engagement per CSM (bar, Total touch point) — gate crm:engagements
	// (sumber F3 SAMA, EngagementsListFilterFor; lihat komentar header file).
	if canViewEngagements(ctx) {
		eng := db.EngagementsListFilterFor(dataScope)
		byCSM, err := q.ReportEngagementByCSM(ctx, db.ReportEngagementByCSMParams{
			ScopeAll: eng.ScopeAll, IsOwn: eng.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		names := make([]string, len(byCSM))
		totals := make([]int64, len(byCSM))
		for i, r := range byCSM {
			names[i] = ownerDisplay(r.OwnerID, r.OwnerName, r.OwnerEmail)
			totals[i] = r.Total
		}
		d.Charts = append(d.Charts, panel.DashChart{
			Title: "Engagement per CSM", ChartID: "chart-cs-engagement",
			ChartJSON: h.marshalChart(barOption(names, totals)),
		})
	}

	// BL-98: tautan CS Report — HANYA bila role ber-crm:reports_cs
	// (BL-169: dulu crm:reports).
	if canViewCSReports(ctx) {
		d.ReportPath = wsPathOf(ctx, "/reports/customer-success")
	}
	// BL-143: cek Charts JUGA (bukan cuma KPIs) — role custom crm:journey/
	// crm:engagements-only (tanpa crm:health/crm:adoption) hanya punya chart,
	// nol KPI; tanpa ini section itu tersembunyi walau punya isi (bug BL-59c
	// asli pakai `|| len(d.Panels) > 0` pun sudah menjaga hal ini).
	return d, len(d.KPIs) > 0 || len(d.Charts) > 0, nil
}
