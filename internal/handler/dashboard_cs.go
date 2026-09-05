package handler

import (
	"context"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_cs.go — BL-59c: section domain "Customer Success" pada Beranda
// (Modul 1 redesain, tasks.md BL-59). Komposisi-per-izin sama dgn Sales/Langganan
// (dashboard_sales.go, dashboard_subscription.go): tiap butir digate kapabilitas
// modulnya (crm:health / crm:adoption / crm:engagements / crm:journey read); role
// melihat UNION butir modul yang boleh diaksesnya; heading tampil hanya bila ≥1
// butir tampil (bool balik). Selaras F2/F3 → role kustom otomatis dapat section
// sesuai modulnya tanpa kode baru.
//
// REUSE agregasi Customer Success Report 8.2 (BL-45, reports_cs.go): ReportCSHealth
// (band berisiko), ReportCSAdoption (rata adopsi), ReportOnboarding (distribusi
// onboarding) + CountEngagementKPIs (header /engagements, jatuh tempo 7 hari).
// TAK menulis ulang query.
//
// F3: query CS pakai AccountsListFilterFor (health/adoption/onboarding ownership
// via kolom accounts assigned_csm/backup_csm/account_owner) & EngagementsListFilterFor
// (engagement ikut desa induk) — sumber SATU dgn modul asal.
//
// §5 (project CLAUDE.md): Distribusi Health SENGAJA global/ungated (dashboard.go,
// dashboardGlobalCharts) — terbuka untuk SEMUA role. Section ini TAK menduplikasi
// donut itu; crm:health di sini hanya menambah angka "Desa Berisiko". F4: semua
// butir CS murni skor/persen/count (tanpa Rp) → tak butuh masking.
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

	if canViewEngagements(ctx) {
		eng := db.EngagementsListFilterFor(dataScope)
		ek, err := q.CountEngagementKPIs(ctx, db.CountEngagementKPIsParams{
			ScopeAll: eng.ScopeAll, IsOwn: eng.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		// Jatuh tempo 7 hari = next_due_date <= hari+7 & status planned (query).
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Engagement Jatuh Tempo (7 hari)", Value: strconv.FormatInt(ek.DueSoonCount, 10),
			ValueClass: "text-warning",
		})
	}

	if canReadCSJourney(ctx) {
		// Periode NULL (zero Timestamptz) = semua kohort; Segmen nil = semua band.
		ob, err := q.ReportOnboarding(ctx, db.ReportOnboardingParams{
			Today:    reportTodayDate(time.Now().In(appTZ)),
			ScopeAll: acc.ScopeAll, IsCsm: acc.IsOwn, IsSales: acc.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.Panels = append(d.Panels, panel.DashPanel{
			Title: "Progres Onboarding", ChartID: "chart-onboarding",
			ChartJSON: h.marshalChart(onboardingChartOption(ob)),
		})
	}

	return d, len(d.KPIs) > 0 || len(d.Panels) > 0, nil
}
