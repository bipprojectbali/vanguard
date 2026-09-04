package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/ui/pages/panel"
)

// reports_support_export.go — CSV per-panel Support Report 8.3 (BL-46). Satu
// route /reports/support/export baca ?panel= (default "volume"). Serial dari
// view yang SAMA (reportsSupportData) → angka CSV identik HTML, bukan jalur
// hitung kedua. F2 & F3 identik jalur HTML (query & argumen scope sama). Panel
// tak dikenal → volume (default aman).

// ReportsSupportExport — GET /reports/support/export?panel=…
func (h *Handler) ReportsSupportExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Support Report", "/reports/support")
		return
	}
	view, err := h.reportsSupportData(ctx, parseSupportReportFilter(r))
	if err != nil {
		h.Log.Error("reports: support export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	name, headers, rows := reportsSupportCSV(r.URL.Query().Get("panel"), view)
	if err := writeCSV(w, name, headers, rows); err != nil {
		h.Log.Error("reports: support export write", "err", err)
	}
}

// reportsSupportCSV memilih panel & merakit baris CSV dari view.
func reportsSupportCSV(panelKey string, v panel.ReportsSupportView) (string, []string, [][]string) {
	switch panelKey {
	case "sla":
		rows := make([][]string, 0, len(v.SLARows))
		for _, r := range v.SLARows {
			rows = append(rows, []string{r.Priority, r.Target, r.MetPct, strconv.FormatInt(r.Breached, 10)})
		}
		return "support-sla", []string{"Prioritas", "Target SLA", "Terpenuhi", "Terlanggar"}, rows

	case "resolution":
		rows := make([][]string, 0, len(v.ResolutionRows))
		for _, r := range v.ResolutionRows {
			rows = append(rows, []string{r.Priority, r.Avg, strconv.FormatInt(r.Count, 10)})
		}
		return "support-resolution", []string{"Prioritas", "Rata Penyelesaian", "Selesai"}, rows

	case "kb":
		rows := make([][]string, 0, len(v.KBRows))
		for _, r := range v.KBRows {
			rows = append(rows, []string{r.Title, strconv.FormatInt(r.Views, 10), r.Status})
		}
		return "support-kb", []string{"Judul", "Dilihat", "Status"}, rows

	case "agent":
		rows := make([][]string, 0, len(v.AgentRows))
		for _, r := range v.AgentRows {
			rows = append(rows, []string{
				r.Agent,
				strconv.FormatInt(r.Handled, 10),
				strconv.FormatInt(r.Resolved, 10),
				r.AvgResolution,
				r.SLACompliance,
				r.StatusLabel,
			})
		}
		return "support-agent", []string{"Agen", "Tiket Ditangani", "Selesai", "Rata Penyelesaian", "Kepatuhan SLA", "Status"}, rows

	default: // "" atau "volume" — default aman.
		rows := make([][]string, 0, len(v.VolumeRows))
		for _, r := range v.VolumeRows {
			rows = append(rows, []string{
				r.Period,
				strconv.FormatInt(r.Masuk, 10),
				strconv.FormatInt(r.Selesai, 10),
				strconv.FormatInt(r.Backlog, 10),
			})
		}
		return "support-report", []string{"Periode", "Tiket Masuk", "Selesai", "Backlog"}, rows
	}
}
