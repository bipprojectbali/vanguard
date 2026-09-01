package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// quotes.go — ~15 quotes tersebar ke SEMUA 6 status, 1-4 quote_items per
// quote merujuk plan nyata. grand_total/tax_amount = SNAPSHOT (komentar
// quotes.sql.go): dihitung di sini dari item lalu ditulis via
// UpdateQuoteTotals — meniru cara app menghitung ulang, bukan agregat live.

var quoteStatuses = []string{"Draft", "Sent", "Under Review", "Accepted", "Rejected", "Expired"}

const taxRatePct = 11 // PPN 11% — konstanta bisnis lokal, bukan hardcode env-dependent.

func seedQuotes(ctx context.Context, q *db.Queries, tenantID int64, tag string, rng *rand.Rand, owner *int64, accounts []accountInfo, deals []int64, plans []int64) (int, int, error) {
	const total = 15
	quoteCount, itemCount := 0, 0

	for i := 0; i < total; i++ {
		code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityQuote)
		if err != nil {
			return quoteCount, itemCount, fmt.Errorf("kode quote: %w", err)
		}
		acc := accounts[i%len(accounts)]
		status := quoteStatuses[i%len(quoteStatuses)]
		var dealID *int64
		if i%2 == 0 && len(deals) > 0 {
			d := deals[i%len(deals)]
			dealID = &d
		}
		name := fmt.Sprintf("Penawaran %s-%03d", tag, i+1)
		terms := pick(rng, []string{"Net 14", "Net 30", "Dibayar di muka"})
		notes := "Harga sudah termasuk instalasi & pelatihan awal."
		expiry := pgDate(time.Now().AddDate(0, 0, 14+rng.Intn(30)))

		quote, err := q.CreateQuote(ctx, db.CreateQuoteParams{
			TenantID:       tenantID,
			EntityCode:     &code,
			DealID:         dealID,
			AccountID:      acc.ID,
			QuoteName:      &name,
			QuoteStatus:    status,
			ExpirationDate: expiry,
			PaymentTerms:   &terms,
			NotesTerms:     &notes,
			PreparedBy:     owner,
			GrandTotal:     mustNum("0"),
			TaxAmount:      mustNum("0"),
			CreatedBy:      owner,
		})
		if err != nil {
			return quoteCount, itemCount, fmt.Errorf("quote #%d: %w", i+1, err)
		}
		quoteCount++

		nItems := 1 + rng.Intn(4)
		subtotalSum := 0.0
		for line := 0; line < nItems; line++ {
			planID := plans[(i+line)%len(plans)]
			qty := int32(1 + rng.Intn(3))
			unitPrice := 1_000_000 + rng.Intn(3_000_000)
			discount := float64(rng.Intn(3)) * 5 // 0/5/10%
			subtotal := float64(qty) * float64(unitPrice) * (1 - discount/100)
			subtotalSum += subtotal
			lineNo := int16(line + 1)

			unitPriceNum, err := num(fmt.Sprintf("%d.00", unitPrice))
			if err != nil {
				return quoteCount, itemCount, err
			}
			discountNum, err := num(fmt.Sprintf("%.2f", discount))
			if err != nil {
				return quoteCount, itemCount, err
			}
			subtotalNum, err := num(fmt.Sprintf("%.2f", subtotal))
			if err != nil {
				return quoteCount, itemCount, err
			}

			if _, err := q.AddQuoteItem(ctx, db.AddQuoteItemParams{
				QuoteID:     quote.ID,
				TenantID:    tenantID,
				PlanID:      &planID,
				Quantity:    qty,
				UnitPrice:   unitPriceNum,
				DiscountPct: discountNum,
				Subtotal:    subtotalNum,
				LineNo:      &lineNo,
			}); err != nil {
				return quoteCount, itemCount, fmt.Errorf("item quote #%d baris %d: %w", i+1, line+1, err)
			}
			itemCount++
		}

		taxAmount := subtotalSum * taxRatePct / 100
		grandTotal := subtotalSum + taxAmount
		taxNum, err := numFromFloat(taxAmount)
		if err != nil {
			return quoteCount, itemCount, err
		}
		grandNum, err := numFromFloat(grandTotal)
		if err != nil {
			return quoteCount, itemCount, err
		}
		rateNum, err := numFromFloat(taxRatePct) // BL-14: PPN = mode persen
		if err != nil {
			return quoteCount, itemCount, err
		}
		if err := q.UpdateQuoteTotals(ctx, db.UpdateQuoteTotalsParams{
			TaxMode:    "percent",
			TaxRate:    rateNum,
			GrandTotal: grandNum,
			TaxAmount:  taxNum,
			UpdatedBy:  owner,
			ID:         quote.ID,
		}); err != nil {
			return quoteCount, itemCount, fmt.Errorf("update total quote #%d: %w", i+1, err)
		}
	}
	return quoteCount, itemCount, nil
}

// numFromFloat = num() dgn pembulatan 2 desimal — dipakai utk total hasil
// perhitungan (subtotal/tax) yang berupa float64, bukan literal string tetap.
func numFromFloat(v float64) (pgtype.Numeric, error) {
	return num(fmt.Sprintf("%.2f", v))
}
