package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// jena_ai_tools_sales.go — tool Jena AI modul Sales (Deal/Lead), dipisah dari
// jena_ai_tools.go (file health, lihat CLAUDE.md §8). Sama aturan F2/F3/F4
// dari jena_ai_tools.go:16-31 — tiap fungsi menegakkan MANUAL:
//
//	F2 — canViewDeals/canViewLeads ("boleh buka modul apa")
//	F3 — DealsListFilterFor/LeadsListFilterFor dari data_scope PENANYA (BUKAN
//	  dipaksa ScopeOwn) untuk search_*; dipaksa IsOwn=true untuk list_my_*
//	F4 — maskARR (Deal Amount / Lead Estimated Value) via canSeeARR(ctx)
//
// list_my_deals/list_my_leads SENGAJA memaksa IsOwn=true terlepas dari
// data_scope role penanya — maksudnya literal "milik SAYA". search_deals/
// search_leads (BL-162 fase 3, dipisah ke jena_ai_tools_sales_search.go agar
// file ini di bawah ambang tipe Route/Handler 150) sebaliknya MENGIKUTI
// data_scope apa adanya — menjawab pertanyaan lintas-pemilik ("lead dibuat
// oleh user jun") tapi tetap tunduk cakupan role: scope 'own' tetap nol baris
// utk data milik orang lain, owner_search tak bisa melebarkan cakupan itu
// (lihat ListLeadsForJena/ListDealsForJena, queries/leads.sql & deals.sql).

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
