package handler

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_quotes_plan_backfill.go — BL-100 (Fix B): saat quote di-Accept, salin paket
// dari item quote ke deal induk (deals.plan_requested_id). Alasan: auto-buat langganan
// Closed Won (BL-21, subscriptionFromWonDeal) MEMBACA deals.plan_requested_id yang TAK
// pernah diisi jalur UI mana pun — paket sebenarnya hidup di quote_items.plan_id. Tanpa
// backfill ini, Deal Won selalu ditolak plan_required walau quote berisi paket (DEAL-156).
//
// Dipisah dari sales_quotes_update.go demi file health (handler ≤150 baris).

// backfillDealPlanFromQuote menurunkan paket tunggal dari item quote lalu menyetelnya di
// deal induk. FAIL-SOFT: semua kegagalan hanya di-Log, TAK menggagalkan Accept (transisi
// status quote tetap sukses). Berjalan di tx Scope yang sama (h.q(ctx)).
//
// Aturan pilih paket (keputusan Fix B — jangan menebak):
//   - TEPAT 1 paket distinct non-null di item → pakai (timpa nilai lama, accept terakhir menang)
//   - 0 paket (item tanpa plan / quote tanpa item) → skip
//   - >1 paket berbeda → skip ('paket utama' = BL terpisah)
func (h *Handler) backfillDealPlanFromQuote(ctx context.Context, dealID, quoteID, uid int64) {
	items, err := h.q(ctx).ListQuoteItems(ctx, quoteID)
	if err != nil {
		h.Log.Error("quotes: backfill deal plan — list items", "quote_id", quoteID, "err", err)
		return
	}
	planID, ok := singlePlanID(items)
	if !ok {
		return // 0 atau >1 paket berbeda → tak ada yang di-backfill
	}
	if err := h.q(ctx).SetDealRequestedPlan(ctx, db.SetDealRequestedPlanParams{
		PlanRequestedID: &planID,
		UpdatedBy:       &uid,
		ID:              dealID,
	}); err != nil {
		h.Log.Error("quotes: backfill deal plan — set", "deal_id", dealID, "quote_id", quoteID, "err", err)
		return
	}
	h.auditWorkspace(ctx, uid, "deal.plan.backfilled", session.TenantID(ctx), map[string]string{
		"deal_id":  strconv.FormatInt(dealID, 10),
		"quote_id": strconv.FormatInt(quoteID, 10),
		"plan_id":  strconv.FormatInt(planID, 10),
	})
}

// recognizeDealValueFromQuote menyalin grand_total quote yang BARU di-Accept ke
// deal.amount (BL-88): nilai DIAKUI menggantikan perkiraan manual. FAIL-SOFT: kegagalan
// hanya di-Log, TAK menggagalkan Accept. Berjalan di tx Scope yang sama (h.q(ctx)).
func (h *Handler) recognizeDealValueFromQuote(ctx context.Context, dealID, quoteID int64, grandTotal pgtype.Numeric, uid int64) {
	if err := h.q(ctx).SetDealRecognizedValue(ctx, db.SetDealRecognizedValueParams{
		Amount:    grandTotal,
		UpdatedBy: &uid,
		ID:        dealID,
	}); err != nil {
		h.Log.Error("quotes: recognize deal value", "deal_id", dealID, "quote_id", quoteID, "err", err)
		return
	}
	h.auditWorkspace(ctx, uid, "deal.value.recognized", session.TenantID(ctx), map[string]string{
		"deal_id":  strconv.FormatInt(dealID, 10),
		"quote_id": strconv.FormatInt(quoteID, 10),
	})
}

// singlePlanID mengembalikan (paket, true) bila item quote menunjuk TEPAT SATU paket
// distinct non-null; else (0, false). Item tanpa plan_id (mis. baris manual) diabaikan.
func singlePlanID(items []db.QuoteItem) (int64, bool) {
	var found int64
	seen := false
	for _, it := range items {
		if it.PlanID == nil {
			continue
		}
		if !seen {
			found, seen = *it.PlanID, true
			continue
		}
		if *it.PlanID != found {
			return 0, false // >1 paket berbeda → ambigu
		}
	}
	return found, seen
}
