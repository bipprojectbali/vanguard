package panel

import (
	"strings"
	"testing"
)

// dashboard_fill_width_test.go — regresi: strip KPI & grid panel Beranda mengisi
// penuh-lebar saat jumlah kartu < kolom (tak ada sel kosong menggantung). Kolom
// dipilih dashKPICols/dashPanelCols dari jumlah kartu; kelas WAJIB literal penuh
// (gotcha #4 — Tailwind tree-shake string terpotong).

func TestDashKPICols_FillsRow(t *testing.T) {
	cases := map[int]string{
		1: "grid grid-cols-1 gap-3 mb-4",
		2: "grid grid-cols-2 gap-3 mb-4",
		3: "grid grid-cols-2 md:grid-cols-3 gap-3 mb-4", // 3 KPI → 3 kolom, bukan 4 (sel kosong)
		4: "grid grid-cols-2 md:grid-cols-4 gap-3 mb-4",
		6: "grid grid-cols-2 md:grid-cols-3 gap-3 mb-4", // 6 → 2 baris rapi 3-kolom
	}
	for n, want := range cases {
		if got := dashKPICols(n); got != want {
			t.Errorf("dashKPICols(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDashPanelCols_LonePanelFullWidth(t *testing.T) {
	if got := dashPanelCols(1); got != "grid grid-cols-1 gap-4" {
		t.Errorf("dashPanelCols(1) = %q, want full-width (tanpa md:grid-cols-2)", got)
	}
	if got := dashPanelCols(2); got != "grid grid-cols-1 md:grid-cols-2 gap-4" {
		t.Errorf("dashPanelCols(2) = %q, want 2 kolom desktop", got)
	}
}

// TestDashboardBody_NoDanglingCells: view dgn 1 domain 3-KPI + 1-panel + chart
// global tunggal TAK boleh menyisakan grid 4/2-kolom yg setengah kosong —
// md:grid-cols-2 tak muncul (semua baris parsial diisi penuh).
func TestDashboardBody_NoDanglingCells(t *testing.T) {
	out := renderLeads(t, DashboardBody(DashboardView{
		HealthChart: "{}",
		Domains: []DashDomain{{
			Title: "Sales",
			KPIs: []DashKPI{
				{Label: "Win Rate", Value: "72%"},
				{Label: "Deal Tutup", Value: "8"},
				{Label: "Aktivitas", Value: "45"},
			},
			Panels: []DashPanel{{Title: "Pipeline", ChartID: "chart-x", ChartJSON: "{}"}},
		}},
	}))
	// Strip 3-KPI → 3 kolom desktop (mengisi baris penuh).
	if !strings.Contains(out, "md:grid-cols-3") {
		t.Errorf("strip 3-KPI harus md:grid-cols-3 (baris penuh):\n%s", out)
	}
	// Chart global tunggal + panel domain tunggal → penuh-lebar, tak ada
	// grid 2-kolom yg separuh kosong.
	if strings.Contains(out, "md:grid-cols-2") {
		t.Errorf("tak boleh ada md:grid-cols-2 saat semua baris chart berisi 1 kartu:\n%s", out)
	}
}
