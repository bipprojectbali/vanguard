package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// reports_cs.go — Customer Success Report (Modul 8, wireframe 8.2): breakdown
// Health/Adoption per status (customer_success, sudah ada sejak Modul 6).
//
// SCOPE SENGAJA DIPERSEMPIT: skema.md §8 memetakan 8.2 ke DUA sumber —
// Health/Adoption (di sini) DAN NPS/CSAT (tabel `surveys`). Tabel `surveys`
// BELUM ADA di migrasi mana pun (dicek juga di semua branch feature/crm-cs-*).
// "Report bukan objek data, nol tabel baru" (skema.md §8) — NPS/CSAT ditunda
// sampai Modul 6 menambah tabel survei, bukan dibangun sebagai tabel baru di
// luar scope slice ini.
//
// F2 gate canViewReports (reports_view.go, SATU objek crm:reports read untuk
// semua preset). F3 ownership via AccountsListFilterFor (sumber sama dgn
// healthScoreListParams, health_score_view.go) — CSM/Sales hanya lihat
// breakdown desa binaannya, bukan lintas-workspace.

// reportsCSData menjalankan agregasi & merakit view-model; dipakai ReportsCS
// (HTML) & ReportsCSExport (CSV) agar keduanya selalu konsisten (satu sumber
// angka, bukan dua jalur hitung terpisah).
func (h *Handler) reportsCSData(ctx context.Context) (panel.ReportsCSView, error) {
	af := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	isOwn := af.IsOwn
	uid := session.UserID(ctx)
	q := h.q(ctx)

	kpis, err := q.CountHealthScoreKPIs(ctx, db.CountHealthScoreKPIsParams{
		ScopeAll: af.ScopeAll, IsCsm: isOwn, IsSales: isOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}
	breakdown, err := q.ReportHealthByStatus(ctx, db.ReportHealthByStatusParams{
		ScopeAll: af.ScopeAll, IsCsm: isOwn, IsSales: isOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsCSView{}, err
	}

	rows := make([]panel.ReportHealthRow, 0, len(breakdown))
	for _, s := range breakdown {
		rows = append(rows, panel.ReportHealthRow{
			Status:   reportHealthStatusLabel(s.HealthStatus),
			Count:    s.AccountCount,
			AvgScore: numericStr(s.AvgHealthScore),
		})
	}

	return panel.ReportsCSView{
		KPIs: panel.HealthScoreKPIs{
			Total:    kpis.Total,
			Healthy:  kpis.Healthy,
			AtRisk:   kpis.AtRisk,
			Critical: kpis.Critical,
		},
		Rows: rows,
	}, nil
}

// reportHealthStatusLabel menerjemahkan health_status mentah ('Healthy',
// 'At-Risk', 'Critical', 'Belum Dinilai' — sudah di-COALESCE di SQL) ke label
// Indonesia. Meniru healthScoreStatus (health_score.go) tapi tanpa badge —
// tabel breakdown report murni teks.
func reportHealthStatusLabel(status string) string {
	switch status {
	case "Healthy":
		return "Sehat"
	case "At-Risk":
		return "Berisiko"
	case "Critical":
		return "Kritis"
	default:
		return status // "Belum Dinilai"
	}
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

// ReportsCSExport — GET /reports/customer-success/export. Data SAMA
// (reportsCSData) diserialisasi CSV via writeCSV (csv_export.go).
func (h *Handler) ReportsCSExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Customer Success Report", "/reports/customer-success")
		return
	}
	view, err := h.reportsCSData(ctx)
	if err != nil {
		h.Log.Error("reports: cs export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows := make([][]string, 0, len(view.Rows))
	for _, s := range view.Rows {
		rows = append(rows, []string{s.Status, strconv.FormatInt(s.Count, 10), s.AvgScore})
	}
	if err := writeCSV(w, "customer-success-report", []string{"Status", "Jumlah Desa", "Rata-rata Skor"}, rows); err != nil {
		h.Log.Error("reports: cs export write", "err", err)
	}
}
