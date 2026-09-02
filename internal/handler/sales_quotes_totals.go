package handler

import (
	"context"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_totals.go — recomputeTotals dipisah dari sales_quotes_update.go
// (file health, ambang tipe Route/Handler 150). Satu paket: dipanggil dari aksi
// item Quote & QuoteTax; logika total = SATU sumber.

// recomputeTotals: hitung ulang snapshot total quote dari subtotal item + konfigurasi
// pajak (BL-14). Dipanggil tiap item berubah (mode/rate/amount = milik quote saat ini)
// & tiap pajak diubah lewat QuoteTax (mode/rate/amount baru).
//
//   - mode 'percent' → tax_amount = subtotal × rate/100, MENGIKUTI subtotal; tax_rate
//     disimpan agar bisa dihitung ulang saat item berubah.
//   - mode 'amount'  → tax_amount = nilai tetap; tax_rate NULL (tak relevan).
//
// grand_total = subtotal + tax_amount di kedua mode.
func (h *Handler) recomputeTotals(ctx context.Context, quoteID int64, mode string, rate, amount pgtype.Numeric, actorID int64) error {
	sub, err := h.q(ctx).QuoteItemsSubtotal(ctx, quoteID)
	if err != nil {
		return err
	}
	var tax, storeRate pgtype.Numeric
	if mode == taxModePercent {
		tax = percentOfNumeric(sub, rate)
		storeRate = rate
	} else {
		mode = taxModeAmount // normalkan nilai tak dikenal → amount (mencerminkan CHECK DB)
		tax = amount
		storeRate = pgtype.Numeric{Valid: false} // NULL: rate tak dipakai di mode amount
	}
	return h.q(ctx).UpdateQuoteTotals(ctx, db.UpdateQuoteTotalsParams{
		TaxMode:    mode,
		TaxRate:    storeRate,
		TaxAmount:  tax,
		GrandTotal: addNumeric(sub, tax),
		UpdatedBy:  &actorID,
		ID:         quoteID,
	})
}
