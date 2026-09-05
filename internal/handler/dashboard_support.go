package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_support.go — BL-59d: section domain "Support" pada Beranda (Modul 1
// redesain, tasks.md BL-59). Komposisi-per-izin sama dgn Sales/Langganan/CS
// (dashboard_sales.go dst): tiap butir digate kapabilitas modulnya (crm:tickets /
// crm:sla read); role melihat UNION butir modul yang boleh diaksesnya; heading
// tampil hanya bila ≥1 butir tampil (bool balik). Selaras F2/F3.
//
// REUSE agregasi Support Report 8.3 (BL-46, reports_support.go): ReportSLAByPriority
// (tiket per-prioritas), ReportAgentPerformance (beban agen), ReportSupportKPIs
// (kepatuhan SLA + rata waktu penyelesaian) + CountTicketKPIs (header /tickets,
// tiket terbuka & langgar). TAK menulis ulang query.
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
		// Tiket Terbuka & Terlambat/Langgar SLA dari header KPI (CountTicketKPIs,
		// sumber SAMA dgn /tickets).
		tk, err := q.CountTicketKPIs(ctx, db.CountTicketKPIsParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs,
			panel.DashKPI{Label: "Tiket Terbuka", Value: strconv.FormatInt(tk.OpenCount, 10), ValueClass: "text-primary"},
			panel.DashKPI{Label: "Terlambat / Langgar SLA", Value: strconv.FormatInt(tk.BreachedCount, 10), ValueClass: "text-error"},
		)

		// Tiket per Prioritas (bar) = REUSE ReportSLAByPriority (Total per prioritas,
		// semua periode/prioritas → Period nil & Priority nil).
		byPrio, err := q.ReportSLAByPriority(ctx, db.ReportSLAByPriorityParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.Panels = append(d.Panels, panel.DashPanel{
			Title: "Tiket per Prioritas", ChartID: "chart-tickets-priority",
			ChartJSON: h.marshalChart(ticketsByPriorityChartOption(byPrio)),
		})

		// Beban Agen (bar) = REUSE ReportAgentPerformance (jumlah tiket ditangani).
		agents, err := q.ReportAgentPerformance(ctx, db.ReportAgentPerformanceParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.Panels = append(d.Panels, panel.DashPanel{
			Title: "Beban Agen", ChartID: "chart-agent-workload",
			ChartJSON: h.marshalChart(agentWorkloadChartOption(agents)),
		})
	}

	if canViewSLAPolicies(ctx) {
		// Kepatuhan SLA & Rata Waktu Penyelesaian = REUSE ReportSupportKPIs.
		kp, err := q.ReportSupportKPIs(ctx, db.ReportSupportKPIsParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs,
			panel.DashKPI{Label: "Kepatuhan SLA", Value: ratePct(kp.Met, kp.WithSla), ValueClass: "text-success"},
			panel.DashKPI{Label: "Rata Waktu Penyelesaian", Value: hoursStr(kp.AvgResolutionHours), ValueClass: "text-primary"},
		)
	}

	return d, len(d.KPIs) > 0 || len(d.Panels) > 0, nil
}

// dashAgentLabel — nama agen utk sumbu chart Beban Agen; fallback ke email
// TER-MASK (bukan email mentah spt tabel Report yg tergerbang crm:reports)
// karena Beranda ini tergerbang crm:tickets — audiens lebih luas.
func dashAgentLabel(r db.ReportAgentPerformanceRow) string {
	if r.AgentName != nil && *r.AgentName != "" {
		return *r.AgentName
	}
	return maskEmail(r.AgentEmail)
}
