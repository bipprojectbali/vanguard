package handler

import (
	"net/http"
	"testing"
)

// sales_quotes_fls_test.go — F4 (field-level) nilai komersial Quote (Subtotal/
// Tax/GrandTotal/UnitPrice per-item): kebijakan umum canSeeARR/maskARR (semua
// role KECUALI Support). Diperbaiki audit FLS M9-1 — sebelumnya angka dirender
// mentah ke SEMUA viewer.
//
// Diuji LANGSUNG atas quoteDetailView (bukan lewat HTTP QuoteDetail): Support
// tak punya crm:deals sama sekali (business_policy.csv) → canViewDeals selalu
// menolak Support SEBELUM quoteDetailView pernah dipanggil (F2-blocked total,
// lihat comment sales_quotes_detail.go). Jadi tak ada jalur HTTP nyata utk
// body "Support melihat Subtotal/Tax/GrandTotal". Unit test langsung atas
// mapper murni-data ini memverifikasi WIRING pemanggilan maskARR di titik
// render (predikat canSeeARR/maskARR itu sendiri sudah diuji tuntas di
// fls_test.go) — pertahanan berlapis bila F2 berubah di masa depan, pola sama
// dgn subscriptions_fls_test.go / accounts_fls_test.go.
func TestQuoteDetailView_AmountMasked(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	quote := env.seedQuote(t, deal.ID, acc.ID, "0")
	planID := env.seedPlan(t, "Paket Quote", "PLAN-Q", "1000000")
	if rec := env.addQuoteItem(t, uid, deal.ID, quote.ID, planID, "2"); rec.Code != http.StatusSeeOther {
		t.Fatalf("add item gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	q := env.mustGetQuote(t, quote.ID)
	const wantAmount = "Rp 2.000.000" // 2 x Rp1.000.000, tax 0

	view := func(role string) (subtotal, tax, grand, itemUnit, itemSub string) {
		req := quotesReq(http.MethodGet, quoteSub(deal.ID, quote.ID), nil, itoa(deal.ID), itoa(quote.ID), "")
		env.runAccount(uid, "owner", role, req, func(w http.ResponseWriter, r *http.Request) {
			v := env.h.quoteDetailView(r.Context(), "", deal.ID, q)
			subtotal, tax, grand = v.Subtotal, v.Tax, v.GrandTotal
			if len(v.Items) > 0 {
				itemUnit, itemSub = v.Items[0].UnitPrice, v.Items[0].Subtotal
			}
		})
		return
	}

	subtotal, tax, grand, itemUnit, itemSub := view("support")
	if subtotal != flsHidden || tax != flsHidden || grand != flsHidden {
		t.Errorf("support: Subtotal/Tax/GrandTotal harus tersamar (%s), got %q/%q/%q", flsHidden, subtotal, tax, grand)
	}
	if itemUnit != flsHidden || itemSub != flsHidden {
		t.Errorf("support: UnitPrice/Subtotal item harus tersamar (%s), got %q/%q", flsHidden, itemUnit, itemSub)
	}

	for _, role := range []string{"sales", "csm", "admin", "manager"} {
		subtotal, tax, grand, itemUnit, itemSub := view(role)
		if subtotal != wantAmount {
			t.Errorf("role %q: Subtotal harus %q, got %q", role, wantAmount, subtotal)
		}
		if grand != wantAmount {
			t.Errorf("role %q: GrandTotal harus %q, got %q", role, wantAmount, grand)
		}
		if tax != "Rp 0" {
			t.Errorf("role %q: Tax harus %q, got %q", role, "Rp 0", tax)
		}
		if itemUnit != "Rp 1.000.000" {
			t.Errorf("role %q: UnitPrice item harus %q, got %q", role, "Rp 1.000.000", itemUnit)
		}
		if itemSub != wantAmount {
			t.Errorf("role %q: Subtotal item harus %q, got %q", role, wantAmount, itemSub)
		}
	}
}
