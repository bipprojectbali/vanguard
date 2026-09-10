package panel

import (
	"strings"
	"testing"
)

// sales_quotes_tax_format_test.go — BL-147: modal Pajak merender input Nominal
// sebagai moneyFieldRp (afiks "Rp" + data-numgroup) selaras Nilai Deal/Estimasi
// Lead, BUKAN type="number" telanjang; halaman memuat numgroup.js agar tampilan
// terkelompok ribuan & ternormalkan jadi digit polos saat submit.

// TestQuoteDetail_TaxAmountUsesMoneyField: input tax_amount = data-numgroup (kait
// numgroup.js) + afiks "Rp", dan /static/numgroup.js dimuat. Field pajak masih
// terkirim (name="tax_amount") agar backend tetap penerima.
func TestQuoteDetail_TaxAmountUsesMoneyField(t *testing.T) {
	v := QuoteDetailView{
		Base: "/w/desa", DealID: 7, ID: 3, QuoteName: "Q-1",
		Status: "Draft", Statuses: []string{"Draft", "Sent"},
		TaxMode: "amount", TaxRateInput: "11", TaxAmountInput: "5000000",
		Items:    []QuoteItemRow{{ID: 9, PlanLabel: "Plan A", Quantity: "1", UnitPrice: "Rp 1", Subtotal: "Rp 1"}},
		Plans:    []QuotePlanOption{{ID: 1, Label: "Plan A"}},
		CanWrite: true, Quotable: true,
	}
	out := renderQuoteNode(t, QuoteDetail(v))

	for _, want := range []string{
		`name="tax_amount"`,     // field pajak tetap terkirim
		`data-numgroup`,         // kait pengelompokan ribuan (moneyFieldRp)
		`/static/numgroup.js`,   // skrip format+normalisasi dimuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BL-147: modal Pajak harus memuat %q:\n%s", want, out)
		}
	}

	// Input Nominal TIDAK boleh type="number" (menolak "5.000.000" terkelompok).
	if strings.Contains(out, `id="f-tax_amount" name="tax_amount" type="number"`) {
		t.Errorf("BL-147: tax_amount tak boleh type=number (moneyField pakai type=text):\n%s", out)
	}
}
