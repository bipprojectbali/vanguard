package panel

import (
	"strings"
	"testing"
)

// plans_kpi_test.go — BL-93: poles header katalog Plans & Pricing. View murni;
// KPI sudah dihitung handler. Jaga: subtitle & label tombol baru, badge status
// "Nonaktif" (bukan "Pensiun"), 4 kartu KPI ter-render (label + nilai + sub),
// dan katalog tanpa harga menampilkan "—".

func TestPlanList_HeaderPolish(t *testing.T) {
	out := renderLeads(t, PlanList(PlanListView{
		Base:     "/w/desa",
		CanWrite: true,
		KPI: PlanKPIView{
			ActiveCount: 3, InactiveCount: 1,
			HasPrice:    true,
			LowestPrice: "Rp 750.000", LowestSub: "Basic / thn",
			HighestPrice: "Rp 1.800.000", HighestSub: "Desa+ Pro / thn",
		},
	}))
	wants := []string{
		"Katalog paket Desa+", // subtitle baru
		"+ Paket Baru",        // label tombol baru
		"Paket Aktif", "Paket Nonaktif", "Harga Terendah", "Harga Tertinggi",
		"Rp 750.000", "Basic / thn",
		"Rp 1.800.000", "Desa+ Pro / thn",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("header/KPI harus memuat %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "Plan Baru\"") || strings.Contains(out, ">Plan Baru<") {
		t.Errorf("label tombol lama \"Plan Baru\" tak boleh muncul")
	}
}

func TestPlanStatusBadge_NonaktifBukanPensiun(t *testing.T) {
	out := renderLeads(t, planStatusBadge(false))
	if !strings.Contains(out, "Nonaktif") {
		t.Errorf("status nonaktif → badge \"Nonaktif\":\n%s", out)
	}
	if strings.Contains(out, "Pensiun") {
		t.Errorf("label lama \"Pensiun\" tak boleh muncul")
	}
	if !strings.Contains(renderLeads(t, planStatusBadge(true)), "Aktif") {
		t.Errorf("status aktif → badge \"Aktif\"")
	}
}

// Katalog tanpa harga dikenali (HasPrice=false) → kedua kartu harga "—",
// count tetap tampil.
func TestPlanKPICards_NoPriceFallback(t *testing.T) {
	out := renderLeads(t, planKPICards(PlanKPIView{ActiveCount: 0, InactiveCount: 0}))
	if !strings.Contains(out, "—") {
		t.Errorf("tanpa harga → kartu tampilkan \"—\":\n%s", out)
	}
	if !strings.Contains(out, "belum ada harga") {
		t.Errorf("sub kartu harga kosong = \"belum ada harga\"")
	}
}

// Grid KPI mobile-first: 1 kolom → md 2 → lg 4 (kelas literal penuh, gotcha #4).
func TestPlanKPICards_MobileFirstGrid(t *testing.T) {
	out := renderLeads(t, planKPICards(PlanKPIView{}))
	if !strings.Contains(out, "grid-cols-1 md:grid-cols-2 lg:grid-cols-4") {
		t.Errorf("grid KPI harus mobile-first 1→2→4:\n%s", out)
	}
}
