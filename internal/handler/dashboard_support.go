package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_support.go — BL-59d + BL-98: section domain "Support" pada Beranda
// (Modul 1). Komposisi-per-izin; heading tampil hanya bila ≥1 KPI tampil.
//
// BL-98 (ramping): section kini MAKSIMUM 2 KPI — Tiket Terbuka (crm:tickets) &
// Kepatuhan SLA (crm:sla) — TANPA chart domain. KPI Terlambat/Langgar & Rata
// Waktu Penyelesaian, chart Tiket-per-Prioritas & Beban-Agen dipindah ke Support
// Report (ditautkan "Lihat Laporan →"). REUSE agregasi Report 8.3 (BL-46):
// CountTicketKPIs (tiket terbuka) & ReportSupportKPIs (kepatuhan SLA).
//
// F3: ownership tiket via TicketsListFilterFor(dataScope, canWriteTicketsPerm) —
// SENGAJA canWriteTicketsPerm (F2 mentah, BUKAN archive-aware canWriteTickets)
// karena Beranda GET murni harus lolos gerbang arsip (rasional identik
// reports_support.go). F4: laporan Support tanpa kolom Rp → tanpa masking nilai.
func (h *Handler) dashSupportDomain(ctx context.Context, dataScope string, uid int64) (panel.DashDomain, bool, error) {
	d := panel.DashDomain{Title: "Support"}
	q := h.q(ctx)
	// Filter kepemilikan tiket dipakai KEDUA gate (tickets & sla) — ReportSupportKPIs
	// pun beragregasi atas tiket, jadi ownership-nya sama.
	tf := db.TicketsListFilterFor(dataScope, canWriteTicketsPerm(ctx))

	if canViewTickets(ctx) {
		// Tiket Terbuka dari header KPI (CountTicketKPIs, sumber SAMA dgn /tickets).
		tk, err := q.CountTicketKPIs(ctx, db.CountTicketKPIsParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Tiket Terbuka", Value: strconv.FormatInt(tk.OpenCount, 10), ValueClass: "text-primary",
		})
	}

	if canViewSLAPolicies(ctx) {
		// Kepatuhan SLA = REUSE ReportSupportKPIs (met/with-sla).
		kp, err := q.ReportSupportKPIs(ctx, db.ReportSupportKPIsParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Kepatuhan SLA", Value: ratePct(kp.Met, kp.WithSla), ValueClass: "text-success",
		})
	}

	// BL-98: tautan Support Report — HANYA bila role ber-crm:reports.
	if canViewReports(ctx) {
		d.ReportPath = wsPathOf(ctx, "/reports/support")
	}
	return d, len(d.KPIs) > 0, nil
}
