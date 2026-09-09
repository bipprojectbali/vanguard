package handler

import (
	"math/big"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sales_deals_won_items_test.go — BL-88 PR2a: Closed Won menyalin quote_items →
// subscription_items (tulis ganda). Membuktikan tiap baris quote yang di-Accept
// tertulis sebagai baris langganan dengan snapshot komersial (plan/qty/subtotal)
// ikut, dan Σ item.mrr cocok parent.mrr (nilai per-item diturunkan dari subtotal).

// subscriptionItems mendaftar baris item satu langganan langsung (assert tulis-ganda).
func (e *testEnv) subscriptionItems(t *testing.T, subID int64) []db.SubscriptionItem {
	t.Helper()
	items, err := e.q.ListSubscriptionItems(t.Context(), subID)
	if err != nil {
		t.Fatalf("list subscription items: %v", err)
	}
	return items
}

// TestWonCreatesSubscriptionItems: quote MULTI-plan (2 paket berbeda) → Accept
// (grand_total → deal.Amount) → Closed Won → langganan lahir dengan 2 subscription_items
// yang mencerminkan quote_items, parent plan_id NULL (identitas di item, BL-88 PR2b), &
// Σ item.mrr == parent.mrr. Dua paket harus DISTINCT: idx_subscription_items_one_active
// menolak 2 item Active ber-paket sama pada satu desa (invarian pindah ke item PR2b).
func TestWonCreatesSubscriptionItems(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Multiline", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: jendela quoting
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	planA := env.seedPlan(t, "Paket Inti", "PLN-ITM-A", "1000000")
	planB := env.seedPlan(t, "Paket Tambahan", "PLN-ITM-B", "1000000")
	// Dua baris paket BERBEDA → singleQuotePlan >1 distinct → parent plan_id NULL
	// (identitas di item); grand_total = 2×1jt = 2jt.
	env.addQuoteItem(t, uid, deal.ID, q.ID, planA, "1")
	env.addQuoteItem(t, uid, deal.ID, q.ID, planB, "1")

	env.acceptQuote(t, uid, deal.ID, q.ID, "Accepted") // salin grand_total → deal.Amount

	// Termin Annual (12 bln) di quote agar Won punya termin sah.
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE quotes SET subscription_term='Annual', contract_term_months=12 WHERE id=$1`,
		q.ID); err != nil {
		t.Fatalf("set quote term: %v", err)
	}

	rec := env.postDealStage(uid, deal.ID, wonForm("Active"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Won harus sukses (?ok=staged), got %q (code %d)\n%s",
			loc, rec.Code, rec.Body.String())
	}

	deal2 := env.freshDeal(t, deal.ID)
	if deal2.CreatedSubscriptionID == nil {
		t.Fatalf("deal.created_subscription_id harus terisi")
	}
	subID := *deal2.CreatedSubscriptionID

	items := env.subscriptionItems(t, subID)
	if len(items) != 2 {
		t.Fatalf("subscription_items = %d, want 2 (mirror quote_items)", len(items))
	}

	sumMRR := new(big.Rat)
	seenPlans := map[int64]bool{}
	for _, it := range items {
		if it.PlanID == nil {
			t.Errorf("item.plan_id harus terisi (paket per-baris)")
		} else {
			seenPlans[*it.PlanID] = true
		}
		if it.Quantity != 1 {
			t.Errorf("item.quantity = %d, want 1", it.Quantity)
		}
		if ratFromNumeric(it.Subtotal).Cmp(ratFromNumeric(numFrom(t, "1000000"))) != 0 {
			t.Errorf("item.subtotal = %s, want 1000000 (snapshot dari quote)", formatRupiah(it.Subtotal))
		}
		sumMRR.Add(sumMRR, ratFromNumeric(it.Mrr))
	}
	if !seenPlans[planA] || !seenPlans[planB] {
		t.Errorf("kedua paket (%d, %d) harus tercermin di item, got %v", planA, planB, seenPlans)
	}

	// Σ item.mrr ≈ parent.mrr (2jt/12): item mrr diturunkan per-baris dari subtotal/
	// bulan-termin, parent dari grand_total/bulan-termin. Σ subtotal == grand_total,
	// jadi keduanya sama SEBELUM pembulatan; sesudah pembulatan ke moneyScale tiap baris
	// bisa selisih ≤ 0.01/baris (di sini 83.333,33×2 = 166.666,66 vs parent 166.666,67).
	// Toleransi = jumlah_baris × 0.01 (batas akumulasi pembulatan per-item).
	sub, err := env.q.GetSubscription(t.Context(), subID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	// Multi-plan → parent plan_id NULL (identitas paket sepenuhnya di item, BL-88 PR2b).
	if sub.PlanID != nil {
		t.Errorf("parent plan_id harus NULL untuk quote multi-paket, got %d", *sub.PlanID)
	}
	diff := new(big.Rat).Abs(new(big.Rat).Sub(sumMRR, ratFromNumeric(sub.Mrr)))
	tol := new(big.Rat).SetFrac64(int64(len(items)), 100) // len × 0.01
	if diff.Cmp(tol) > 0 {
		t.Errorf("Σ item.mrr = %s vs parent.mrr %s: selisih %s > toleransi %s",
			sumMRR.FloatString(2), formatRupiah(sub.Mrr), diff.FloatString(2), tol.FloatString(2))
	}
}
