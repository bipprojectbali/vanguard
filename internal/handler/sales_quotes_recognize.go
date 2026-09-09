package handler

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_quotes_recognize.go — BL-88: saat quote di-Accept, quote jadi SUMBER
// KEBENARAN komersial → grand_total disalin ke deal.amount (nilai DIAKUI
// menggantikan perkiraan manual). Dipisah dari sales_quotes_update.go demi file
// health (handler ≤150 baris). (Backfill plan_requested_id single-plan PR1 sudah
// di-retire PR2b — paket kini diturunkan langsung dari quote_items saat Closed Won.)

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
