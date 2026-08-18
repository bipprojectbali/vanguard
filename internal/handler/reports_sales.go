package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// reports_sales.go — Sales Report (Modul 8 M8-1, wireframe 8.1): pipeline
// per-stage TERMASUK Closed Won/Lost + kartu win-rate. Reuse DealPipelineStats
// (queries/deals.sql, sudah ada) untuk ringkasan, ReportPipelineByStage
// (queries/reports.sql, baru) untuk tabel per-stage. F2 gate canViewReports
// (reports_view.go); F3 ownership via DealsListFilterFor (sumber sama dgn
// dealsPipeline) — laporan tak boleh bocor lintas-pemilik bagi sales biasa.
//
// F4: PipelineValue & Value per-stage lewat maskARR (fls.go) — SAMA dgn
// semua permukaan nilai deal lain (sales_deals_page.go dkk). Gate Reports
// SATU objek crm:reports untuk semua preset (reports_view.go), dan Support
// TERMASUK pemegangnya (business_policy.csv) — tanpa masking di sini, Support
// bisa buka /reports/sales langsung dan melihat nilai komersial mentah,
// melanggar skema.md §9 ("MRR/ARR/amount disembunyikan dari Support ...
// deals"). Diperbaiki audit FLS M9; sebelumnya sengaja tanpa masking.

// reportsSalesData menjalankan agregasi & merakit view-model; dipakai
// ReportsSales (HTML) & ReportsSalesExport (CSV) agar keduanya selalu
// konsisten (satu sumber angka, bukan dua jalur hitung terpisah).
func (h *Handler) reportsSalesData(ctx context.Context) (panel.ReportsSalesView, error) {
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	q := h.q(ctx)

	stats, err := q.DealPipelineStats(ctx, db.DealPipelineStatsParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	stages, err := q.ReportPipelineByStage(ctx, db.ReportPipelineByStageParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}

	total := stats.WonCount + stats.LostCount
	winRate := "—"
	if total > 0 {
		winRate = strconv.FormatFloat(float64(stats.WonCount)*100/float64(total), 'f', 1, 64) + "%"
	}

	rows := make([]panel.ReportStageRow, 0, len(stages))
	for _, s := range stages {
		rows = append(rows, panel.ReportStageRow{
			Stage: s.Stage,
			Count: s.DealCount,
			Value: maskARR(formatRupiah(s.StageValue), br),
		})
	}

	return panel.ReportsSalesView{
		OpenCount:     stats.OpenCount,
		PipelineValue: maskARR(formatRupiah(stats.PipelineValue), br),
		WinRate:       winRate,
		Stages:        rows,
	}, nil
}

// ReportsSales — GET /reports/sales. Bukan pemegang izin crm:reports read →
// 403 + penjelasan (pola sama dgn plans_page.go/subscriptions_page.go).
func (h *Handler) ReportsSales(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Sales Report", "/reports/sales")
		return
	}
	view, err := h.reportsSalesData(ctx)
	if err != nil {
		h.Log.Error("reports: sales data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Sales Report", "/reports/sales", panel.ReportsSalesBody(view))
}

// ReportsSalesExport — GET /reports/sales/export. Data SAMA (reportsSalesData)
// diserialisasi CSV via writeCSV (csv_export.go). Nilai numerik APA ADANYA
// (bukan formatRupiah) — angka mentah lebih berguna untuk spreadsheet. Kolom
// Nilai tetap lewat maskARR (F4) — CSV bukan celah untuk melewati masking HTML.
func (h *Handler) ReportsSalesExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Sales Report", "/reports/sales")
		return
	}
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	stages, err := h.q(ctx).ReportPipelineByStage(ctx, db.ReportPipelineByStageParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		h.Log.Error("reports: sales export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows := make([][]string, 0, len(stages))
	for _, s := range stages {
		rows = append(rows, []string{s.Stage, strconv.FormatInt(s.DealCount, 10), maskARR(numericStr(s.StageValue), br)})
	}
	if err := writeCSV(w, "sales-report", []string{"Stage", "Jumlah Deal", "Nilai"}, rows); err != nil {
		h.Log.Error("reports: sales export write", "err", err)
	}
}

// renderReportsForbidden — 403 + penjelasan; dipakai Sales & Subscription
// Report (slice 3). Path dioper karena Reports punya dua halaman berbeda.
func (h *Handler) renderReportsForbidden(w http.ResponseWriter, r *http.Request, title, path string) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, title, path, panel.SalesForbidden(title))
}
