package handler

import (
	"testing"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// plans_kpi_test.go — BL-93: KPI header katalog Plans & Pricing. planCatalogKPIView
// murni (tanpa DB) → uji langsung: count diteruskan apa adanya, harga min/max
// DINORMALISASI ke tahunan (Monthly x12) sebelum dibanding, plan tanpa harga/siklus
// dikenali dilewati, katalog kosong → HasPrice=false.

func numFor(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("scan numeric %q: %v", s, err)
	}
	return n
}

func planFor(t *testing.T, name, billing, basePrice string) db.Plan {
	t.Helper()
	p := db.Plan{PlanName: name}
	if billing != "" {
		b := billing
		p.BillingFrequency = &b
	}
	if basePrice != "" {
		p.BasePrice = numFor(t, basePrice)
	}
	return p
}

func TestPlanCatalogKPIView_CountsPassThrough(t *testing.T) {
	got := planCatalogKPIView(db.PlanCatalogStatsRow{ActiveCount: 7, InactiveCount: 3}, nil)
	if got.ActiveCount != 7 || got.InactiveCount != 3 {
		t.Fatalf("count: got aktif=%d nonaktif=%d, want 7/3", got.ActiveCount, got.InactiveCount)
	}
	if got.HasPrice {
		t.Errorf("tanpa plan berharga → HasPrice harus false")
	}
}

// Inti keputusan BL-93: Monthly Rp 150rb (setara Rp 1,8jt/thn) TAK boleh jadi
// terendah dibanding Annual Rp 750rb — normalisasi tahunan membalik urutan mentah.
func TestPlanCatalogKPIView_NormalizesMonthlyToAnnual(t *testing.T) {
	active := []db.Plan{
		planFor(t, "Basic", billingAnnual, "750000"),      // 750.000/thn
		planFor(t, "Desa+ Pro", billingMonthly, "150000"), // 150.000 x12 = 1.800.000/thn
	}
	got := planCatalogKPIView(db.PlanCatalogStatsRow{ActiveCount: 2}, active)
	if !got.HasPrice {
		t.Fatal("HasPrice harus true")
	}
	if got.LowestPrice != "Rp 750.000" {
		t.Errorf("terendah = %q, want %q (Basic annual)", got.LowestPrice, "Rp 750.000")
	}
	if got.LowestSub != "Basic / thn" {
		t.Errorf("sub terendah = %q, want %q", got.LowestSub, "Basic / thn")
	}
	if got.HighestPrice != "Rp 1.800.000" {
		t.Errorf("tertinggi = %q, want %q (Desa+ Pro annualized)", got.HighestPrice, "Rp 1.800.000")
	}
	if got.HighestSub != "Desa+ Pro / thn" {
		t.Errorf("sub tertinggi = %q, want %q", got.HighestSub, "Desa+ Pro / thn")
	}
}

// Plan tanpa base_price atau siklus tak dikenal (NULL/asing) tak bisa dinormalisasi
// → dilewati dari min/max, tak mengacaukan rentang.
func TestPlanCatalogKPIView_SkipsUnpriceable(t *testing.T) {
	active := []db.Plan{
		planFor(t, "Ada Harga", billingAnnual, "500000"),
		planFor(t, "Tanpa Harga", billingAnnual, ""), // base_price NULL → dilewati
		planFor(t, "Tanpa Siklus", "", "999999"),     // billing NULL → dilewati
		planFor(t, "Siklus Asing", "Weekly", "1"),    // siklus tak dikenal → dilewati
	}
	got := planCatalogKPIView(db.PlanCatalogStatsRow{ActiveCount: 4}, active)
	if !got.HasPrice {
		t.Fatal("plan berharga tunggal → HasPrice true")
	}
	if got.LowestPrice != "Rp 500.000" || got.HighestPrice != "Rp 500.000" {
		t.Errorf("min/max harus = satu-satunya plan sah (Rp 500.000); got lo=%q hi=%q",
			got.LowestPrice, got.HighestPrice)
	}
}

func TestPlanCatalogKPIView_EmptyCatalog(t *testing.T) {
	got := planCatalogKPIView(db.PlanCatalogStatsRow{}, nil)
	if got.HasPrice {
		t.Errorf("katalog kosong → HasPrice false (view tampilkan \"—\")")
	}
}
