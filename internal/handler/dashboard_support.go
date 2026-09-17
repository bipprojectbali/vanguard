package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_support.go — BL-59d + BL-98 + BL-144: section domain "Support" pada
// Beranda (Modul 1). Komposisi-per-izin; heading tampil hanya bila ≥1 KPI/chart
// tampil.
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
//
// BL-144 (membalik BL-98 KHUSUS Beranda): tambah 2 chart inline, digate PERSIS
// sama dgn KPI kembar di section ini (bukan crm:reports) — sumber query & F3
// identik Support Report 8.3 (reports_support.go), nol query baru.
//   - Volume Tiket per Bulan (bar, Tiket Masuk): REUSE ReportTicketVolumeByMonth
//     (panel 1 Report) → barOption(months, masuk) — SATU seri (Masuk) sesuai
//     ticket ("barOption(months, counts)"); Selesai/Backlog tetap eksklusif
//     tabel Report. Digate crm:tickets (SAMA gate KPI Tiket Terbuka).
//   - SLA per Prioritas (bar, % Terpenuhi): REUSE ReportSLAByPriority (panel 2
//     Report) → barOption(priorities, metPct) — memecah KPI agregat "Kepatuhan
//     SLA" di atas per prioritas (pola sama BL-142 Revenue-per-Paket vs KPI
//     MRR). Digate crm:sla (SAMA gate KPI Kepatuhan SLA). Label bulan/prioritas
//     REUSE forecastPeriodLabel/ticketPriorityLabelID (string SAMA dgn tabel
//     Report). Chart kosong (nol baris) di-skip, bukan pai/bar telanjang (pola
//     BL-141 Win/Loss). Period/Priority filter kosong (unbounded/semua) —
//     konsisten KPI section ini yg juga tak difilter periode.
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

		// Chart: Volume Tiket per Bulan (Masuk) — gate SAMA KPI di atas.
		vol, err := q.ReportTicketVolumeByMonth(ctx, db.ReportTicketVolumeByMonthParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		if len(vol) > 0 {
			periods := make([]string, len(vol))
			masuk := make([]int64, len(vol))
			for i, r := range vol {
				periods[i] = forecastPeriodLabel(r.Period)
				masuk[i] = r.Masuk
			}
			d.Charts = append(d.Charts, panel.DashChart{
				Title: "Volume Tiket per Bulan", ChartID: "chart-support-volume",
				ChartJSON: h.marshalChart(barOption(periods, masuk)),
			})
		}
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

		// Chart: SLA per Prioritas (% Terpenuhi) — gate SAMA KPI di atas; memecah
		// agregat itu per prioritas (pola sama Revenue-per-Paket/BL-142).
		sla, err := q.ReportSLAByPriority(ctx, db.ReportSLAByPriorityParams{
			ScopeAll: tf.ScopeAll, IsOwn: tf.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		if len(sla) > 0 {
			priorities := make([]string, len(sla))
			metPct := make([]int64, len(sla))
			for i, r := range sla {
				priorities[i] = ticketPriorityLabelID(r.Priority)
				if r.WithSla > 0 {
					metPct[i] = r.Met * 100 / r.WithSla
				}
			}
			d.Charts = append(d.Charts, panel.DashChart{
				Title: "SLA per Prioritas", ChartID: "chart-support-sla",
				ChartJSON: h.marshalChart(barOption(priorities, metPct)),
			})
		}
	}

	// BL-98: tautan Support Report — HANYA bila role ber-crm:reports_support
	// (BL-169: dulu crm:reports).
	if canViewSupportReports(ctx) {
		d.ReportPath = wsPathOf(ctx, "/reports/support")
	}
	// BL-144: cek Charts JUGA (bukan cuma KPIs) — sama alasan BL-143 (dashboard_cs.go).
	return d, len(d.KPIs) > 0 || len(d.Charts) > 0, nil
}
