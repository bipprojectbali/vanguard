package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// reports_support.go — Support Report (Modul 8, wireframe 8.3, BL-46): 5 panel
// spec 8.3, DIBANGUN hanya yang datanya SUDAH ADA. Orkestrasi di sini;
// transformasi baris→sub-view di reports_support_panels.go; serialisasi CSV
// per-panel di reports_support_export.go. "Report bukan objek data": nol tabel
// baru, agregasi murni atas tickets/sla_policies/kb_articles/users.
//
// DILEWATKAN (tak dirender, tak ada tabel/kolomnya — bukan bug, keputusan sadar
// BL-46 "data belum ada DILEWATKAN dulu"): CSAT (KPI ke-4, tak ada survei),
// volume per-kategori & Kanal (tak ada kolom), Respons pertama & FCR (tak ada
// first_response_at/reopen), KB rating/deflection/membantu (tak ada kolomnya).
//
// F3 ownership via TicketsListFilterFor(dataScope, canWriteTicketsPerm(ctx))
// — SENGAJA pakai canWriteTicketsPerm (F2 mentah tulis tiket, TANPA cek
// arsip/IsReadOnly), BUKAN canWriteTickets (dipakai /tickets untuk gating
// tombol). Laporan ini GET murni: varian archive-aware akan membuat Support
// (data_scope='none') kehilangan cakupan ScopeAll saat workspace diarsipkan —
// padahal GET seharusnya tetap lolos gerbang arsip (CLAUDE.md §Siklus hidup).
// KB (panel 4) tenant-scoped via RLS (h.q) — tak ber-owner, tanpa filter F3.
//
// F2 gate canViewReports (reports_view.go, SATU objek crm:reports read). F4:
// laporan ini tanpa kolom Rp → tanpa masking.

// reportsSupportData menjalankan agregasi & merakit view-model 5 panel; dipakai
// ReportsSupport (HTML) & ReportsSupportExport (CSV) agar keduanya konsisten
// (satu sumber angka, bukan dua jalur hitung terpisah).
func (h *Handler) reportsSupportData(ctx context.Context, f supportReportFilter) (panel.ReportsSupportView, error) {
	filter := db.TicketsListFilterFor(session.BusinessDataScope(ctx), canWriteTicketsPerm(ctx))
	uid := session.UserID(ctx)
	prio := f.priorityArg() // *string prioritas (nil = semua)
	// scope = argumen bersama 5 query utama (bentuk field identik → type-convert).
	// Periode & Prioritas ikut di sini; ReportResolutionMonthCompare (tanpa
	// Periode) & ReportKBPublished (tanpa params) dirakit terpisah di bawah.
	scope := db.ReportTicketVolumeByMonthParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, Priority: prio,
	}
	q := h.q(ctx)

	volume, err := q.ReportTicketVolumeByMonth(ctx, scope)
	if err != nil {
		return panel.ReportsSupportView{}, err
	}
	kpis, err := q.ReportSupportKPIs(ctx, db.ReportSupportKPIsParams(scope))
	if err != nil {
		return panel.ReportsSupportView{}, err
	}
	sla, err := q.ReportSLAByPriority(ctx, db.ReportSLAByPriorityParams(scope))
	if err != nil {
		return panel.ReportsSupportView{}, err
	}
	resolution, err := q.ReportResolutionByPriority(ctx, db.ReportResolutionByPriorityParams(scope))
	if err != nil {
		return panel.ReportsSupportView{}, err
	}
	// MonthCompare: "bulan ini vs lalu" relatif now() → Periode TAK berlaku,
	// hanya Prioritas menyaring (BL-51).
	compare, err := q.ReportResolutionMonthCompare(ctx, db.ReportResolutionMonthCompareParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid, Priority: prio,
	})
	if err != nil {
		return panel.ReportsSupportView{}, err
	}
	agents, err := q.ReportAgentPerformance(ctx, db.ReportAgentPerformanceParams(scope))
	if err != nil {
		return panel.ReportsSupportView{}, err
	}
	kb, err := q.ReportKBPublished(ctx)
	if err != nil {
		return panel.ReportsSupportView{}, err
	}

	thisLabel, prevLabel := resolutionMonthLabels(todayInAppTZ())
	v := panel.ReportsSupportView{
		Filter: h.buildSupportFilterView(ctx, f),

		TotalTickets:  formatInt(kpis.Total),
		SLACompliance: ratePct(kpis.Met, kpis.WithSla),
		AvgResolution: hoursStr(kpis.AvgResolutionHours),

		VolumeRows: buildVolumeRows(volume),

		SLAMetPct:    ratePct(kpis.Met, kpis.WithSla),
		SLABreachPct: ratePct(kpis.Breached, kpis.WithSla),
		SLAAtRisk:    kpis.AtRisk,
		SLARows:      buildSLARows(sla),

		ResolutionRows: buildResolutionRows(resolution),
		ResThisLabel:   thisLabel,
		ResThisMonth:   hoursStr(compare.ThisMonthHours),
		ResPrevLabel:   prevLabel,
		ResPrevMonth:   hoursStr(compare.PrevMonthHours),
		ResChange:      resolutionChange(compare.ThisMonthHours, compare.PrevMonthHours),

		KBRows:    buildKBRows(kb),
		AgentRows: buildAgentRows(agents),
	}
	return v, nil
}

// ReportsSupport — GET /reports/support. Bukan pemegang izin crm:reports read
// → 403 + penjelasan (pola sama dgn reports_sales.go).
func (h *Handler) ReportsSupport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Support Report", "/reports/support")
		return
	}
	view, err := h.reportsSupportData(ctx, parseSupportReportFilter(r))
	if err != nil {
		h.Log.Error("reports: support data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Support Report", "/reports/support", panel.ReportsSupportBody(view))
}
