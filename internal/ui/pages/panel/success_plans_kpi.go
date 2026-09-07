package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// success_plans_kpi.go — kartu KPI header dasbor Success Plans (BL-97).
// View murni-data: semua angka & sub-teks dihitung di handler
// (successPlanKPIsToView). Empat kartu selaras mockup Image #24 (KPI langkah
// "Target Tercapai" disubstitusi rata capaian % karena model langkah belum ada).

// SuccessPlanKPIs — angka + sub-teks empat kartu KPI header.
type SuccessPlanKPIs struct {
	ActiveCount    string // "RENCANA AKTIF"
	ActiveSub      string // "di N desa"
	Overdue        string // "TERLAMBAT"
	OverdueSub     string // "melewati tenggat"
	DueSoon        string // "JATUH TEMPO 14 HARI"
	DueSoonSub     string // "perlu dikejar"
	AvgProgress    string // "RATA CAPAIAN" (substitusi Target Tercapai) mis. "68%"
	AvgProgressSub string // "rata capaian"
}

// successPlanKPICards — 4 kartu KPI: Aktif / Terlambat / Jatuh Tempo 14 Hari /
// Rata Capaian. Mobile-first: grid-cols-2 md:grid-cols-4.
func successPlanKPICards(k SuccessPlanKPIs) g.Node {
	return h.Div(h.Class("grid grid-cols-2 md:grid-cols-4 gap-3"),
		successPlanKPICard("Rencana Aktif", k.ActiveCount, "text-base-content", k.ActiveSub),
		successPlanKPICard("Terlambat", k.Overdue, "text-error", k.OverdueSub),
		successPlanKPICard("Jatuh Tempo 14 Hari", k.DueSoon, "text-warning", k.DueSoonSub),
		successPlanKPICard("Rata Capaian", k.AvgProgress, "text-primary", k.AvgProgressSub),
	)
}

// successPlanKPICard — satu kartu KPI dengan sub-teks. sub kosong → baris sub
// tetap dirender kosong agar tinggi kartu seragam antar-kolom (grid stretch).
func successPlanKPICard(label, value, valueClass, sub string) g.Node {
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body p-4"),
			h.P(h.Class("text-sm text-base-content/60"), g.Text(label)),
			h.P(h.Class("text-2xl font-bold "+valueClass), g.Text(value)),
			h.P(h.Class("text-xs text-base-content/50"), g.Text(sub)),
		),
	)
}
