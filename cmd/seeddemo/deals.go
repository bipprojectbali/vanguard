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

// deals.go — 48 deals: taper 5 stage terbuka (Prospecting 15/Qualification
// 10/Demo 6/Proposal 4/Negotiation 2) + Closed Won 8 + Closed Lost 3, agar
// win-rate & pipeline-by-stage (Sales Report) proporsional bukan rata.

var dealStageSpecs = []struct {
	stage       string
	count       int
	probability int16
	pastClose   bool // true = expected_close_date di MASA LALU (deal closed)
}{
	{"Prospecting", 15, 10, false},
	{"Qualification", 10, 30, false},
	{"Demo", 6, 50, false},
	{"Proposal", 4, 70, false},
	{"Negotiation", 2, 85, false},
	{"Closed Won", 8, 100, true},
	{"Closed Lost", 3, 0, true},
}

func seedDeals(ctx context.Context, q *db.Queries, tenantID int64, tag string, rng *rand.Rand, owner *int64, accounts []accountInfo, plans []int64) ([]int64, error) {
	var ids []int64
	seq := 0
	for _, spec := range dealStageSpecs {
		for i := 0; i < spec.count; i++ {
			seq++
			code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityDeal)
			if err != nil {
				return nil, fmt.Errorf("kode deal: %w", err)
			}
			acc := accounts[seq%len(accounts)]
			planID := plans[seq%len(plans)]
			amount, err := num(fmt.Sprintf("%d.00", 5_000_000+rng.Intn(40_000_000)))
			if err != nil {
				return nil, err
			}

			var closeDate pgtype.Date
			if spec.pastClose {
				closeDate = pgDate(time.Now().AddDate(0, 0, -(7 + rng.Intn(60))))
			} else {
				closeDate = pgDate(time.Now().AddDate(0, 0, 7+rng.Intn(90)))
			}

			var dealOwner *int64
			if rng.Intn(100) < 70 {
				dealOwner = owner
			}

			forecast := forecastFor(spec.stage)
			dealType := pick(rng, []string{"New Business", "Renewal", "Upsell", "Cross-sell"})
			term := pick(rng, []string{"Monthly", "Annual", "Multi-year"})
			prob := spec.probability

			deal, err := q.CreateDeal(ctx, db.CreateDealParams{
				TenantID:          tenantID,
				EntityCode:        &code,
				DealName:          fmt.Sprintf("Langganan %s #%s", acc.Type, tag),
				AccountID:         acc.ID,
				DealOwner:         dealOwner,
				PlanRequestedID:   &planID,
				DealType:          &dealType,
				Stage:             spec.stage,
				Amount:            amount,
				Probability:       &prob,
				ExpectedCloseDate: closeDate,
				ForecastCategory:  &forecast,
				SubscriptionTerm:  &term,
				CreatedBy:         owner,
			})
			if err != nil {
				return nil, fmt.Errorf("deal #%d: %w", seq, err)
			}
			ids = append(ids, deal.ID)
		}
	}
	return ids, nil
}

// forecastFor mengembalikan kategori forecast yg wajar sesuai stage (kolom
// tanpa CHECK constraint di DB, jadi nilai bebas asal konsisten).
func forecastFor(stage string) string {
	switch stage {
	case "Closed Won":
		return "Closed"
	case "Closed Lost":
		return "Omitted"
	case "Negotiation", "Proposal":
		return "Best Case"
	case "Demo":
		return "Pipeline"
	default:
		return "Pipeline"
	}
}

// pgDate mengubah time.Time → pgtype.Date (tanggal murni, tanpa jam).
func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}
