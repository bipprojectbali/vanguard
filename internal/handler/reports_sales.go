package handler

import (
	"net/http"

	"go_starter/internal/ui/pages/panel"
)

// reports_sales.go — Sales Report (Modul 8, wireframe 8.1, BL-43): 5 panel
// (Pipeline · Forecast · Win/Loss · Lead Conversion · Sales Activity) + KPI
// ringkas. Orkestrasi di sini; transformasi baris→sub-view di
// reports_sales_panels.go; serialisasi CSV per-panel di reports_sales_export.go.
// "Report bukan objek data" (skema.md §8): nol tabel baru, agregasi murni.
//
// TIGA sumbu keamanan:
//   - F2 gate canViewReports (reports_view.go, SATU objek crm:reports read).
//   - F3 ownership per-sumber via {Deals,Leads,Activities}ListFilterFor
//     (session.BusinessDataScope) — sumber SATU dgn modul asal, tak duplikasi
//     logic scope. Support data_scope='none' atas deals → panel deal kosong
//     (F2 tetap 200).
//   - F4 maskARR pada SEMUA nilai Rp (Pipeline Value/Weighted, Forecast, KPI
//     Nilai Pipeline). Support pemegang crm:reports tapi DI LUAR allow-list
//     canSeeARR (skema.md §9) → nilai tersamar. Panel Win/Loss, Lead
//     Conversion, Sales Activity murni angka/persen/hari → tanpa masking.
//
// Filter interaktif Periode + Tim(owner) (BL-49) memotong SEMUA panel + KPI +
// CSV serentak via salesReportFilter (reports_sales_filter.go). Empat gap
// butuh-schema (Target/Batal/picklist Alasan/qualified_at) → BL-44.

// ReportsSales — GET /reports/sales. Bukan pemegang izin crm:reports read →
// 403 + penjelasan (pola sama dgn plans_page.go/subscriptions_page.go).
func (h *Handler) ReportsSales(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Sales Report", "/reports/sales")
		return
	}
	view, err := h.reportsSalesData(ctx, parseSalesReportFilter(r))
	if err != nil {
		h.Log.Error("reports: sales data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Sales Report", "/reports/sales", panel.ReportsSalesBody(view))
}

// renderReportsForbidden — 403 + penjelasan; dipakai Sales & Subscription
// Report (slice 3). Path dioper karena Reports punya dua halaman berbeda.
func (h *Handler) renderReportsForbidden(w http.ResponseWriter, r *http.Request, title, path string) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, title, path, panel.SalesForbidden(title))
}
