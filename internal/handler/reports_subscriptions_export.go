package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/ui/pages/panel"
)

// reports_subscriptions_export.go — CSV per-panel Subscription Report 8.4
// (BL-47). Satu route /reports/subscriptions/export baca ?panel= (default
// "mrr"). Serial dari view yang SAMA (reportsSubscriptionsData) → angka CSV
// identik HTML, bukan jalur hitung kedua. F2 & F3 & F4 identik jalur HTML
// (query, argumen scope, & masking sama). Panel tak dikenal → mrr (default aman).

// ReportsSubscriptionsExport — GET /reports/subscriptions/export?panel=…
func (h *Handler) ReportsSubscriptionsExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Subscription Report", "/reports/subscriptions")
		return
	}
	view, err := h.reportsSubscriptionsData(ctx)
	if err != nil {
		h.Log.Error("reports: subscriptions export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	name, headers, rows := reportsSubscriptionsCSV(r.URL.Query().Get("panel"), view)
	if err := writeCSV(w, name, headers, rows); err != nil {
		h.Log.Error("reports: subscriptions export write", "err", err)
	}
}

// reportsSubscriptionsCSV memilih panel & merakit baris CSV dari view. Nilai Rp
// SUDAH ter-mask F4 di view (builder) — CSV tak membocorkan apa yang HTML tutup.
func reportsSubscriptionsCSV(panelKey string, v panel.ReportsSubscriptionsView) (string, []string, [][]string) {
	switch panelKey {
	case "renewal":
		rows := make([][]string, 0, len(v.RenewalMonths))
		for _, r := range v.RenewalMonths {
			rows = append(rows, []string{
				r.Period,
				strconv.FormatInt(r.Due, 10),
				strconv.FormatInt(r.Renewed, 10),
				r.Rate,
			})
		}
		return "subscription-renewal", []string{"Periode", "Jatuh Tempo", "Diperpanjang", "Rate"}, rows

	case "churn":
		rows := make([][]string, 0, len(v.ChurnReasons))
		for _, r := range v.ChurnReasons {
			rows = append(rows, []string{
				r.Reason,
				strconv.FormatInt(r.Count, 10),
				r.LostValue,
				r.Porsi,
			})
		}
		return "subscription-churn", []string{"Alasan", "Desa", "Nilai Hilang", "Porsi"}, rows

	case "plan":
		rows := make([][]string, 0, len(v.RevenueByPlan))
		for _, r := range v.RevenueByPlan {
			rows = append(rows, []string{
				r.Plan,
				strconv.FormatInt(r.Count, 10),
				r.MRR,
				r.AvgPer,
			})
		}
		return "subscription-plan", []string{"Paket", "Desa", "MRR", "Rata per Desa"}, rows

	case "aging":
		rows := make([][]string, 0, len(v.AgingRows))
		for _, r := range v.AgingRows {
			rows = append(rows, []string{
				r.Bucket,
				strconv.FormatInt(r.Villages, 10),
				r.MRR,
				r.AvgHealth,
				r.RenewalRate,
				r.ChurnRate,
				r.Note,
			})
		}
		return "subscription-aging", []string{"Kelompok Umur", "Desa", "MRR", "Rata Health", "Renewal Rate", "Churn Rate", "Catatan"}, rows

	default: // "" atau "mrr" — default aman.
		rows := make([][]string, 0, len(v.MRRComponents))
		for _, r := range v.MRRComponents {
			rows = append(rows, []string{
				r.Component,
				r.Value,
				strconv.FormatInt(r.Count, 10),
				r.Porsi,
			})
		}
		return "subscription-mrr", []string{"Komponen MRR", "Nilai", "Desa", "Porsi"}, rows
	}
}
