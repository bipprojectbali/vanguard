package handler

import (
	"net/http"
)

// reports_sales_export.go — CSV per-panel Sales Report 8.1 (BL-43). Satu route
// /reports/sales/export baca ?panel= (default "pipeline", KOMPATIBEL MUNDUR
// dgn M8-1 yang cuma punya pipeline). Nilai numerik APA ADANYA (numericStr,
// bukan formatRupiah) — angka mentah lebih berguna untuk spreadsheet; kolom Rp
// TETAP lewat maskARR (F4) agar CSV bukan celah melewati masking HTML. F2 & F3
// identik jalur HTML (query yang sama, argumen scope yang sama).

// ReportsSalesExport — GET /reports/sales/export?panel=…
func (h *Handler) ReportsSalesExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Sales Report", "/reports/sales")
		return
	}
	name, headers, rows, err := h.reportsSalesCSV(ctx, r.URL.Query().Get("panel"), parseSalesReportFilter(r))
	if err != nil {
		h.Log.Error("reports: sales export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := writeCSV(w, name, headers, rows); err != nil {
		h.Log.Error("reports: sales export write", "err", err)
	}
}
