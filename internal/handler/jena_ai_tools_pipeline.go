package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// jena_ai_tools_pipeline.go — implementasi tool get_subscription_status.
// Dipisah dari jena_ai_tools.go krn ambang File Health (Route/Handler 150
// baris); murni pindah, tak ada perubahan logika. list_my_deals/list_my_leads
// pindah ke jena_ai_tools_sales.go (BL-162 fase 3, bersama search_deals/
// search_leads yang baru).

func (h *Handler) jenaGetSubscriptionStatus(ctx context.Context, input json.RawMessage) (string, error) {
	var in jenaAccountIDInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("input tidak valid: %w", err)
	}
	a, ok, err := h.jenaLoadAccount(ctx, in.AccountID)
	if err != nil {
		return "", fmt.Errorf("ambil desa: %w", err)
	}
	if !ok {
		return `{"error":"desa tidak ditemukan"}`, nil
	}

	sub, err := h.q(ctx).GetLatestSubscriptionForAccount(ctx, a.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return `{"error":"desa ini belum punya langganan"}`, nil
		}
		return "", fmt.Errorf("ambil langganan: %w", err)
	}

	canARR := canSeeSubscriptionARR(ctx)
	out := map[string]any{
		"village_name": a.VillageName,
		"status":       sub.Status,
		"start_date":   dateStr(sub.StartDate),
		"end_date":     dateStr(sub.EndDate),
		"mrr":          maskARR(formatRupiah(sub.Mrr), canARR),
		"arr":          maskSubscriptionARR(formatRupiah(sub.Arr), canARR),
	}
	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(result), nil
}
