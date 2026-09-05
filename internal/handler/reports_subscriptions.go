package handler

import (
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_subscriptions.go — Subscription Report (Modul 8, wireframe 8.4,
// BL-47): 5 panel agregasi (MRR/ARR · Renewal · Churn · Revenue by Plan ·
// Aging) + 4 KPI. Orkestrasi di sini; transformasi baris→sub-view di
// reports_subscriptions_panels.go; serialisasi CSV per-panel di
// reports_subscriptions_export.go. "Report bukan objek data" (skema.md §8):
// NOL tabel baru, agregasi murni atas subscriptions (+ plans, customer_success).
// Drill-down per-desa renewal/churn TETAP di modul Subscriptions
// (/subscriptions/renewals · /churn) — laporan ini agregasi, bukan daftar.
//
// DILEWATKAN (keputusan sadar BL-47, "data belum ada DILEWATKAN dulu"): delta
// "vs bulan lalu" kartu MRR & tren MRR bulanan historis (tak ada snapshot MRR
// lampau). Filter interaktif Periode+Paket ditunda ke BL lanjutan (pola
// BL-49/50/51).
//
// F2 gate canViewReports (SATU objek crm:reports read). F3 ownership via
// SubscriptionsListFilterFor (subscription_owner — sumber SAMA modul asal). F4
// masking Rp via maskARR di builder (Support/role tanpa akses → "•••").

// reportTodayDate = "hari ini" appTZ sebagai pgtype.Date (tengah malam UTC —
// jendela query date_trunc/end_date berbasis date murni). Dioper ke query
// ber-Today (hindari AT TIME ZONE di SELECT list sqlc, gotcha #14).
func reportTodayDate(now time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

// ReportsSubscriptions — GET /reports/subscriptions. Bukan pemegang izin
// crm:reports read → 403 + penjelasan (pola sama reports_support.go).
func (h *Handler) ReportsSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Subscription Report", "/reports/subscriptions")
		return
	}
	f := parseSubscriptionReportFilter(r)
	view, err := h.reportsSubscriptionsData(ctx, f)
	if err != nil {
		h.Log.Error("reports: subscriptions data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sf := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	filterView, err := h.buildSubscriptionFilterView(ctx, f, sf, session.UserID(ctx))
	if err != nil {
		h.Log.Error("reports: subscriptions filter", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Filter = filterView
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Subscription Report", "/reports/subscriptions",
		panel.ReportsSubscriptionsBody(view))
}
