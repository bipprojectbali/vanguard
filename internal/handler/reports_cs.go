package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_cs.go — Customer Success Report (Modul 8, wireframe 8.2, BL-45): 6
// panel spec 8.2, DIBANGUN hanya yang datanya SUDAH ADA. Orkestrasi di sini;
// transformasi baris→sub-view di reports_cs_panels.go; serialisasi CSV per-panel
// di reports_cs_export.go. "Report bukan objek data" (skema.md §8): nol tabel
// baru, agregasi murni atas customer_success/subscriptions/engagements.
//
// DILEWATKAN (tak dirender, tak ada tabelnya — bukan bug): NPS/CSAT (tak ada
// `surveys`), tren health bulanan (tak ada snapshot), adopsi per-fitur (tak ada
// model pemakaian), onboarding per-tahap (tak ada timestamp antara), net
// retention (tak ada delta ekspansi MRR). Ini keputusan sadar BL-45 (docs/crm/
// tasks.md): "data yang belum ada DILEWATKAN dulu, tak bikin tabel baru".
//
// Filter interaktif Periode + Segmen (disebut di teks 8.2) DITUNDA ke BL
// terpisah — pola sama BL-43→BL-49 di Sales Report (jangan borong di sini).
//
// TIGA sumbu keamanan:
//   - F2 gate canViewReports (reports_view.go, SATU objek crm:reports read).
//   - F3 ownership per-sumber via {Accounts,Subscriptions,Engagements}ListFilterFor
//     (session.BusinessDataScope) — sumber SATU dgn modul asal, tak duplikasi
//     logic scope. CSM (own) hanya lihat desa binaannya lintas ketiga sumber.
//   - F4 maskARR pada kolom Rp (Nilai Hilang di tabel churn). Support pemegang
//     crm:reports tapi DI LUAR allow-list canSeeARR (skema.md §9) → tersamar.
//     Panel lain murni skor/persen/hari → tanpa masking.

// reportsCSData menjalankan agregasi & merakit view-model 6 panel; dipakai
// ReportsCS (HTML) & ReportsCSExport (CSV) agar keduanya konsisten (satu sumber
// angka). Tiga bentuk filter ownership karena tiga sumber tabel berbeda (lihat
// doc queries/reports.sql).
func (h *Handler) reportsCSData(ctx context.Context) (panel.ReportsCSView, error) {
	scope := session.BusinessDataScope(ctx)
	acc := db.AccountsListFilterFor(scope)
	sub := db.SubscriptionsListFilterFor(scope)
	eng := db.EngagementsListFilterFor(scope)
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	q := h.q(ctx)

	health, err := q.ReportCSHealth(ctx, db.ReportCSHealthParams{
		ScopeAll: acc.ScopeAll, IsCsm: acc.IsOwn, IsSales: acc.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}
	adoption, err := q.ReportCSAdoption(ctx, db.ReportCSAdoptionParams{
		ScopeAll: acc.ScopeAll, IsCsm: acc.IsOwn, IsSales: acc.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}
	retention, err := q.ReportRetention(ctx, db.ReportRetentionParams{
		ScopeAll: sub.ScopeAll, IsOwn: sub.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}
	churn, err := q.ReportChurnReasons(ctx, db.ReportChurnReasonsParams{
		ScopeAll: sub.ScopeAll, IsOwn: sub.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}
	onboarding, err := q.ReportOnboarding(ctx, db.ReportOnboardingParams{
		Today:    pgtype.Date{Time: todayInAppTZ(), Valid: true},
		ScopeAll: acc.ScopeAll, IsCsm: acc.IsOwn, IsSales: acc.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}
	compliance, err := q.ReportEngagementCompliance(ctx, db.ReportEngagementComplianceParams{
		ScopeAll: eng.ScopeAll, IsOwn: eng.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}
	byCSM, err := q.ReportEngagementByCSM(ctx, db.ReportEngagementByCSMParams{
		ScopeAll: eng.ScopeAll, IsOwn: eng.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}

	v := panel.ReportsCSView{
		AvgHealth:     csAvgStr(health.AvgHealth, health.Scored),
		AvgAdoption:   csPctStr(adoption.AvgAdoption, adoption.Scored),
		RetentionRate: ratePct(retention.Active, retention.Active+retention.Churned),

		HealthScored: health.Scored,
		HealthBands:  buildHealthBands(health),

		AdoptionScored: adoption.Scored,
		AdoptionBands:  buildAdoptionBands(adoption),

		RetentionPct: ratePct(retention.Active, retention.Active+retention.Churned),
		ChurnPct:     ratePct(retention.Churned, retention.Active+retention.Churned),
		ActiveCount:  retention.Active,
		ChurnedCount: retention.Churned,
		ChurnReasons: buildChurnRows(churn, br),

		OnboardAvgDuration: daysStr(onboarding.AvgDurationDays, onboarding.CompletedWithDates),
		OnboardCompleted:   onboarding.Completed,
		OnboardLate:        onboarding.Late,
		OnboardStatus:      buildOnboardingBands(onboarding),

		EngagementTypes: buildComplianceRows(compliance),
		EngagementCSMs:  buildEngagementCSMRows(byCSM),
	}
	return v, nil
}

// ReportsCS — GET /reports/customer-success. Bukan pemegang izin crm:reports
// read → 403 + penjelasan (pola sama dgn reports_sales.go).
func (h *Handler) ReportsCS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Customer Success Report", "/reports/customer-success")
		return
	}
	view, err := h.reportsCSData(ctx)
	if err != nil {
		h.Log.Error("reports: cs data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Customer Success Report", "/reports/customer-success", panel.ReportsCSBody(view))
}
