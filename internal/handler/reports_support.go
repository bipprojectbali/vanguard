package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// reports_support.go — Support Report (Modul 8, wireframe 8.3): Ticket
// Volume / SLA / Resolution, breakdown per status tiket (tickets +
// sla_policies, Modul 6 slice B2). F2 gate canViewReports (reports_view.go,
// SATU objek crm:reports read untuk semua preset).
//
// F3 ownership via TicketsListFilterFor(dataScope, canWriteTicketsPerm(ctx))
// — SENGAJA pakai canWriteTicketsPerm (F2 mentah tulis tiket, TANPA cek
// arsip/IsReadOnly), BUKAN canWriteTickets (dipakai halaman /tickets untuk
// gating tombol tulis). Laporan ini GET murni: memakai varian archive-aware
// akan membuat Support (data_scope='none') kehilangan cakupan ScopeAll-nya
// begitu workspace diarsipkan — padahal request GET seharusnya tetap lolos
// gerbang arsip (gateLifecycle, CLAUDE.md §Siklus hidup).

// reportsSupportData menjalankan agregasi & merakit view-model; dipakai
// ReportsSupport (HTML) & ReportsSupportExport (CSV) agar keduanya selalu
// konsisten (satu sumber angka, bukan dua jalur hitung terpisah).
func (h *Handler) reportsSupportData(ctx context.Context) (panel.ReportsSupportView, error) {
	filter := db.TicketsListFilterFor(session.BusinessDataScope(ctx), canWriteTicketsPerm(ctx))
	uid := session.UserID(ctx)
	q := h.q(ctx)

	kpis, err := q.CountTicketKPIs(ctx, db.CountTicketKPIsParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSupportView{}, err
	}
	breakdown, err := q.ReportTicketsByStatus(ctx, db.ReportTicketsByStatusParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSupportView{}, err
	}

	rows := make([]panel.ReportTicketStatusRow, 0, len(breakdown))
	for _, s := range breakdown {
		rows = append(rows, panel.ReportTicketStatusRow{
			Status:            ticketStatusLabel(s.Status),
			Count:             s.TicketCount,
			Breached:          s.BreachedCount,
			AvgHoursToResolve: numericStr(s.AvgResolutionHours),
		})
	}

	return panel.ReportsSupportView{
		KPIs: ticketKPIView(kpis),
		Rows: rows,
	}, nil
}

// ticketStatusLabel menerjemahkan status tiket mentah ('baru', 'ditugaskan',
// 'eskalasi', 'selesai') ke label Indonesia. Meniru ticketStatusBadge
// (tickets_list.go) tapi mengembalikan teks polos — tabel breakdown report
// murni teks, tanpa badge.
func ticketStatusLabel(status string) string {
	switch status {
	case "baru":
		return "Baru"
	case "ditugaskan":
		return "Ditugaskan"
	case "eskalasi":
		return "Eskalasi"
	case "selesai":
		return "Selesai"
	default:
		return status
	}
}

// ReportsSupport — GET /reports/support. Bukan pemegang izin crm:reports read
// → 403 + penjelasan (pola sama dgn reports_sales.go).
func (h *Handler) ReportsSupport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Support Report", "/reports/support")
		return
	}
	view, err := h.reportsSupportData(ctx)
	if err != nil {
		h.Log.Error("reports: support data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Support Report", "/reports/support", panel.ReportsSupportBody(view))
}

// ReportsSupportExport — GET /reports/support/export. Data SAMA
// (reportsSupportData) diserialisasi CSV via writeCSV (csv_export.go).
func (h *Handler) ReportsSupportExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Support Report", "/reports/support")
		return
	}
	view, err := h.reportsSupportData(ctx)
	if err != nil {
		h.Log.Error("reports: support export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows := make([][]string, 0, len(view.Rows))
	for _, s := range view.Rows {
		rows = append(rows, []string{
			s.Status,
			strconv.FormatInt(s.Count, 10),
			strconv.FormatInt(s.Breached, 10),
			s.AvgHoursToResolve,
		})
	}
	header := []string{"Status", "Jumlah Tiket", "SLA Terlanggar", "Rata-rata Resolusi (jam)"}
	if err := writeCSV(w, "support-report", header, rows); err != nil {
		h.Log.Error("reports: support export write", "err", err)
	}
}
