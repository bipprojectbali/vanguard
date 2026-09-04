package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/ui/pages/panel"
)

// reports_cs_export.go — CSV per-panel Customer Success Report 8.2 (BL-45). Satu
// route /reports/customer-success/export baca ?panel= (default "health"). Serial
// dari view yang SAMA (reportsCSData) → angka & masking (F4 Nilai Hilang) CSV
// identik HTML, bukan jalur hitung kedua. F2 & F3 identik jalur HTML (query &
// argumen scope sama). Panel tak dikenal → health (default aman).

// ReportsCSExport — GET /reports/customer-success/export?panel=…
func (h *Handler) ReportsCSExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Customer Success Report", "/reports/customer-success")
		return
	}
	view, err := h.reportsCSData(ctx, parseCSReportFilter(r))
	if err != nil {
		h.Log.Error("reports: cs export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	name, headers, rows := reportsCSCSV(r.URL.Query().Get("panel"), view)
	if err := writeCSV(w, name, headers, rows); err != nil {
		h.Log.Error("reports: cs export write", "err", err)
	}
}

// reportsCSCSV memilih panel & merakit baris CSV dari view. Distribusi band
// (health/adoption/onboarding) berbentuk sama → bandCSV. Churn & engagement
// punya kolom sendiri.
func reportsCSCSV(panelKey string, v panel.ReportsCSView) (string, []string, [][]string) {
	switch panelKey {
	case "adoption":
		return "cs-adoption", []string{"Band Adopsi", "Jumlah Desa", "Porsi"}, bandCSV(v.AdoptionBands)

	case "retention", "churn":
		rows := make([][]string, 0, len(v.ChurnReasons))
		for _, c := range v.ChurnReasons {
			rows = append(rows, []string{c.Reason, strconv.FormatInt(c.Count, 10), c.Porsi, c.LostValue})
		}
		return "cs-churn", []string{"Alasan Churn", "Jumlah Desa", "Porsi", "Nilai Hilang"}, rows

	case "onboarding":
		return "cs-onboarding", []string{"Status", "Jumlah Desa", "Porsi"}, bandCSV(v.OnboardStatus)

	case "engagement":
		rows := make([][]string, 0, len(v.EngagementTypes))
		for _, e := range v.EngagementTypes {
			rows = append(rows, []string{e.Type, e.TouchPoint, e.Compliance})
		}
		return "cs-engagement", []string{"Tipe", "Touch Point", "Kepatuhan"}, rows

	case "engagement-csm":
		rows := make([][]string, 0, len(v.EngagementCSMs))
		for _, e := range v.EngagementCSMs {
			rows = append(rows, []string{e.CSM, strconv.FormatInt(e.Villages, 10), e.TouchPoint, e.Compliance})
		}
		return "cs-engagement-csm", []string{"CSM", "Desa Dipegang", "Touch Point", "Kepatuhan"}, rows

	default: // "" atau "health" — default aman.
		return "customer-success-report", []string{"Band Kesehatan", "Jumlah Desa", "Porsi"}, bandCSV(v.HealthBands)
	}
}

// bandCSV serialisasi baris distribusi band (health/adoption/onboarding) — tiga
// panel berbagi bentuk (Label · Count · Porsi).
func bandCSV(bands []panel.ReportBandRow) [][]string {
	rows := make([][]string, 0, len(bands))
	for _, b := range bands {
		rows = append(rows, []string{b.Label, strconv.FormatInt(b.Count, 10), b.Pct})
	}
	return rows
}
