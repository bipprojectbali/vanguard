package handler

import "testing"

// TestDealTermMonths: BL-87 opsi c — dealTermMonths() (dioper ke view untuk
// preview MRR/ARR) HARUS mencerminkan termContractMonths (sumber tunggal yang
// dipakai Won→Langganan). Bila keduanya bergeser, preview klien menyesatkan.
func TestDealTermMonths(t *testing.T) {
	got := dealTermMonths()
	if len(got) != len(termContractMonths) {
		t.Fatalf("dealTermMonths panjang %d, termContractMonths %d — harus sama",
			len(got), len(termContractMonths))
	}
	for term, months := range termContractMonths {
		if got[term] != int(months) {
			t.Errorf("Termin %q: dealTermMonths=%d, termContractMonths=%d",
				term, got[term], months)
		}
	}
	// Nilai konkret (kunci regresi vs. rumus Won): Monthly=1, Annual=12, Multi-year=36.
	for term, want := range map[string]int{"Monthly": 1, "Annual": 12, "Multi-year": 36} {
		if got[term] != want {
			t.Errorf("Termin %q harus %d bulan, dapat %d", term, want, got[term])
		}
	}
}
