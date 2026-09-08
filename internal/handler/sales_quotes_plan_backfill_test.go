package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_quotes_plan_backfill_test.go — BL-100 (Fix B): backfill deals.plan_requested_id
// saat quote di-Accept. Membuktikan Deal Won→Langganan (BL-21) tak lagi ditolak
// plan_required saat quote berisi paket. Empat kasus aturan pilih-paket + gerbang
// status non-Accept.

// acceptQuote menjalankan QuoteStatus dengan status tertentu sebagai admin owner dan
// mengembalikan Location redirect (assert ok/err).
func (e *testEnv) acceptQuote(t *testing.T, uid, dealID, quoteID int64, status string) string {
	t.Helper()
	form := url.Values{"quote_status": {status}}
	req := quotesReq(http.MethodPost, quoteSub(dealID, quoteID)+"/status", form,
		itoa(dealID), itoa(quoteID), "")
	rec := e.runAccount(uid, "owner", "admin", req, e.h.QuoteStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=status") {
		t.Fatalf("status %q harus ok=status, got %q (code %d)", status, loc, rec.Code)
	}
	return status
}

// dealPlan membaca deals.plan_requested_id (nil bila tak diset).
func (e *testEnv) dealPlan(t *testing.T, dealID int64) *int64 {
	t.Helper()
	d, err := e.q.GetDeal(t.Context(), dealID)
	if err != nil {
		t.Fatalf("get deal %d: %v", dealID, err)
	}
	return d.PlanRequestedID
}

func TestQuoteAccept_BackfillsSinglePlanToDeal(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Backfill", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: jendela quoting
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	plan := env.seedPlan(t, "Paket A", "PKA", "1000000")
	env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1")

	env.acceptQuote(t, uid, deal.ID, q.ID, "Accepted")

	got := env.dealPlan(t, deal.ID)
	if got == nil || *got != plan {
		t.Fatalf("plan_requested_id harus %d, got %v", plan, got)
	}
	env.assertAudited(t, "deal.plan.backfilled")
}

func TestQuoteAccept_NoItems_LeavesDealPlanNil(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kosong", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0") // tanpa item

	env.acceptQuote(t, uid, deal.ID, q.ID, "Accepted")

	if got := env.dealPlan(t, deal.ID); got != nil {
		t.Fatalf("tanpa item, plan_requested_id harus nil, got %v", *got)
	}
}

func TestQuoteAccept_MultipleDistinctPlans_LeavesDealPlanNil(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Ambigu", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	planA := env.seedPlan(t, "Paket A", "PKA", "1000000")
	planB := env.seedPlan(t, "Paket B", "PKB", "2000000")
	env.addQuoteItem(t, uid, deal.ID, q.ID, planA, "1")
	env.addQuoteItem(t, uid, deal.ID, q.ID, planB, "1")

	env.acceptQuote(t, uid, deal.ID, q.ID, "Accepted")

	if got := env.dealPlan(t, deal.ID); got != nil {
		t.Fatalf(">1 paket berbeda harus skip (nil), got %v", *got)
	}
}

func TestQuoteStatus_NonAccept_DoesNotBackfill(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sent", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	plan := env.seedPlan(t, "Paket A", "PKA", "1000000")
	env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1")

	env.acceptQuote(t, uid, deal.ID, q.ID, "Sent") // bukan Accepted

	if got := env.dealPlan(t, deal.ID); got != nil {
		t.Fatalf("status non-Accept tak boleh backfill, got %v", *got)
	}
}
