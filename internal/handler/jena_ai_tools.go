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

// jena_ai_tools.go — allowlist tool BL-162 fase 2: Jena AI dapat akses BACA ke
// database lewat tool-calling Anthropic, dieksekusi via h.q(ctx) tenant-scoped
// (BUKAN db.WithSuper — itu untuk internal/mcpserver yang tanpa Scope). Tiap
// tool menegakkan MANUAL tiga sumbu keamanan CRM (lihat fls.go:11-28) — sqlc
// query di baliknya TIDAK menerapkan F2/F3/F4 sendiri:
//
//	F2 Casbin    — canViewAccounts/canViewDeals ("boleh buka modul apa")
//	F3 ownership — XxxListFilter.Allows ("atas data siapa"); gagal → jawab
//	  "tidak ditemukan", MENIRU pola 404-gate handler lain (accounts_detail_page.go)
//	  — jangan bocorkan ke model/user bahwa baris itu ADA di workspace lain.
//	F4 FLS       — maskARR/maskSubscriptionARR ("field mana boleh tampil")
//
// Pola thin-adapter + allowlist eksplisit sama seperti internal/mcpserver
// (README paket itu).
//
// Tool Deal/Lead (implementasi di jena_ai_tools_sales.go) datang DUA rasa,
// SENGAJA dipisah bukan satu tool ber-parameter, agar Claude tak perlu
// menebak default parameter mana yang berarti "punya saya":
//   - list_my_deals/list_my_leads — SELALU memaksa IsOwn=true (mengabaikan
//     data_scope role penanya) — literal "milik SAYA", tanpa parameter.
//   - search_deals/search_leads — F3 mengikuti data_scope APA ADANYA (via
//     DealsListFilterFor/LeadsListFilterFor, TIDAK dipaksa own) + filter
//     opsional nama/kode & nama/email pemilik — utk pertanyaan lintas-pemilik
//     ("lead dibuat oleh user jun"), tetap tunduk cakupan role penanya: scope
//     'own' tetap nol baris utk data milik orang lain (BL-162 fase 3).

const jenaSearchAccountsLimit = 5
const jenaMyDealsLimit = 10
const jenaMyLeadsLimit = 10
const jenaSearchDealsLimit = 10
const jenaSearchLeadsLimit = 10

// jenaDispatch adalah claudeai.ToolDispatcher — switch ke satu fungsi per tool.
// Audit RINGAN per panggilan (nama tool saja, bukan input/hasil — gotcha
// #12/#15), dicatat terlepas dari tool itu akhirnya berhasil/ditolak F2/F3.
// Tool tak dikenal balas JSON error defensif — harusnya tak pernah terjadi
// karena `tools` yang dikirim ke model selalu dari jenaTools() di atas.
func (h *Handler) jenaDispatch(ctx context.Context, name string, input json.RawMessage) (string, error) {
	h.auditWorkspace(ctx, session.UserID(ctx), "jena_ai.tool_call", session.TenantID(ctx),
		map[string]string{"tool": name})

	switch name {
	case "search_accounts":
		return h.jenaSearchAccounts(ctx, input)
	case "get_account_summary":
		return h.jenaGetAccountSummary(ctx, input)
	case "get_subscription_status":
		return h.jenaGetSubscriptionStatus(ctx, input)
	case "list_my_deals":
		return h.jenaListMyDeals(ctx)
	case "list_my_leads":
		return h.jenaListMyLeads(ctx)
	case "search_deals":
		return h.jenaSearchDeals(ctx, input)
	case "search_leads":
		return h.jenaSearchLeads(ctx, input)
	default:
		return `{"error":"tool tidak dikenal"}`, nil
	}
}

type jenaAccountIDInput struct {
	AccountID int64 `json:"account_id"`
}

func (h *Handler) jenaSearchAccounts(ctx context.Context, input json.RawMessage) (string, error) {
	if !canViewAccounts(ctx) {
		return `{"error":"tidak berhak melihat data desa"}`, nil
	}
	var in struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("input tidak valid: %w", err)
	}

	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	cursorAt, cursorID := firstPageCursor()
	rows, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsSales:         filter.IsOwn,
		IsCsm:           filter.IsOwn,
		Uid:             &uid,
		Search:          in.Query,
		PageSize:        jenaSearchAccountsLimit,
	})
	if err != nil {
		return "", fmt.Errorf("cari desa: %w", err)
	}

	type row struct {
		AccountID   int64  `json:"account_id"`
		VillageName string `json:"village_name"`
	}
	results := make([]row, 0, len(rows))
	for _, a := range rows {
		results = append(results, row{AccountID: a.ID, VillageName: a.VillageName})
	}
	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(out), nil
}

// jenaLoadAccount memuat satu Account + menegakkan F2+F3 — dipakai bersama
// get_account_summary & get_subscription_status supaya keduanya tak
// menduplikasi pola gate (accounts_detail_page.go:26-58).
func (h *Handler) jenaLoadAccount(ctx context.Context, accountID int64) (db.Account, bool, error) {
	if !canViewAccounts(ctx) {
		return db.Account{}, false, nil
	}
	a, err := h.q(ctx).GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Account{}, false, nil
		}
		return db.Account{}, false, err
	}
	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), a.AccountOwner, a.AssignedCsm, a.BackupCsm) {
		return db.Account{}, false, nil
	}
	return a, true, nil
}

func (h *Handler) jenaGetAccountSummary(ctx context.Context, input json.RawMessage) (string, error) {
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

	out := map[string]any{
		"village_name":   a.VillageName,
		"account_type":   a.AccountType,
		"village_status": deref(a.VillageStatus),
		"territory":      deref(a.Territory),
		"village_budget": maskARR(formatRupiah(a.VillageBudget), canSeeARR(ctx)),
	}
	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(result), nil
}

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

// jenaListMyDeals, jenaListMyLeads, jenaSearchDeals, jenaSearchLeads — lihat
// jena_ai_tools_sales.go.
