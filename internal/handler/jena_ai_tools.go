package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go_starter/internal/claudeai"
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
// (README paket itu). list_my_deals SENGAJA memaksa IsOwn=true terlepas dari
// data_scope role penanya — maksudnya literal "deal milik SAYA", bukan "semua
// deal yang boleh saya lihat" (itu tetap lewat menu Sales biasa).

const jenaSearchAccountsLimit = 5
const jenaMyDealsLimit = 10
const jenaMyLeadsLimit = 10

// jenaTools mendaftarkan allowlist eksplisit tool yang boleh dipanggil Jena AI
// — dikirim ke claudeai.AskWithTools, dibaca ulang oleh jenaDispatch di bawah.
func (h *Handler) jenaTools() []claudeai.Tool {
	return []claudeai.Tool{
		{
			Name: "search_accounts",
			Description: "Cari desa (Account) berdasarkan nama atau kode desa. " +
				"Kembalikan daftar id+nama saja (tanpa data finansial/kepemilikan) — " +
				"dipakai untuk menemukan account_id sebelum memanggil " +
				"get_account_summary atau get_subscription_status.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "nama atau kode desa yang dicari"}
				},
				"required": ["query"]
			}`),
		},
		{
			Name: "get_account_summary",
			Description: "Ambil ringkasan satu desa (Account) berdasarkan account_id " +
				"— profil pemerintahan, kepemilikan (Account Owner/CSM), dan anggaran " +
				"desa (bila penanya berhak melihat nilai komersial). Hanya berhasil " +
				"untuk desa yang boleh dilihat penanya.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"account_id": {"type": "integer", "description": "id account, hasil search_accounts"}
				},
				"required": ["account_id"]
			}`),
		},
		{
			Name: "get_subscription_status",
			Description: "Ambil status langganan (Subscription) TERBARU satu desa " +
				"berdasarkan account_id — paket, status, tanggal mulai/berakhir, " +
				"MRR/ARR (bila penanya berhak melihat nilai komersial).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"account_id": {"type": "integer", "description": "id account, hasil search_accounts"}
				},
				"required": ["account_id"]
			}`),
		},
		{
			Name: "list_my_deals",
			Description: "Daftar Deal (proses penjualan) MILIK penanya sendiri — " +
				"selalu hanya deal dengan Deal Owner = penanya, apa pun cakupan data " +
				"perannya. Tanpa parameter.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name: "list_my_leads",
			Description: "Daftar Lead (prospek belum jadi Deal) MILIK penanya sendiri — " +
				"selalu hanya lead dengan Lead Owner = penanya, apa pun cakupan data " +
				"perannya. Tanpa parameter.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

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
