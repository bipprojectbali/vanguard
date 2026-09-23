package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// jena_ai_tools_pipeline.go — implementasi tool get_subscription_status,
// list_my_deals & list_my_leads. Dipisah dari jena_ai_tools.go krn ambang
// File Health (Route/Handler 150 baris); murni pindah, tak ada perubahan
// logika.

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

func (h *Handler) jenaListMyDeals(ctx context.Context) (string, error) {
	if !canViewDeals(ctx) {
		return `{"error":"tidak berhak melihat data deal"}`, nil
	}

	uid := session.UserID(ctx)
	canARR := canSeeARR(ctx)
	rows, err := h.q(ctx).ListDealsForPipeline(ctx, db.ListDealsForPipelineParams{
		ScopeAll: false,
		IsOwn:    true,
		Uid:      &uid,
		PageSize: jenaMyDealsLimit,
	})
	if err != nil {
		return "", fmt.Errorf("daftar deal: %w", err)
	}

	type row struct {
		DealName string `json:"deal_name"`
		Stage    string `json:"stage"`
		Amount   string `json:"amount"`
	}
	results := make([]row, 0, len(rows))
	for _, d := range rows {
		results = append(results, row{
			DealName: d.DealName,
			Stage:    d.Stage,
			Amount:   maskARR(formatRupiah(d.Amount), canARR),
		})
	}
	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(out), nil
}

func (h *Handler) jenaListMyLeads(ctx context.Context) (string, error) {
	if !canViewLeads(ctx) {
		return `{"error":"tidak berhak melihat data lead"}`, nil
	}

	uid := session.UserID(ctx)
	canARR := canSeeARR(ctx)
	cursorAt, cursorID := firstPageCursor()
	rows, err := h.q(ctx).ListLeads(ctx, db.ListLeadsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        false,
		IsOwn:           true,
		Uid:             &uid,
		MineOnly:        true,
		StatusFilter:    "",
		Search:          "",
		PageSize:        jenaMyLeadsLimit,
	})
	if err != nil {
		return "", fmt.Errorf("daftar lead: %w", err)
	}

	type row struct {
		LeadName string `json:"lead_name"`
		Status   string `json:"status"`
		EstValue string `json:"est_value"`
	}
	results := make([]row, 0, len(rows))
	for _, l := range rows {
		results = append(results, row{
			LeadName: l.LeadName,
			Status:   l.LeadStatus,
			EstValue: maskARR(formatRupiah(l.EstimatedValue), canARR),
		})
	}
	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(out), nil
}
