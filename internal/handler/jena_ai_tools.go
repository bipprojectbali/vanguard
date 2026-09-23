package handler

import (
	"context"
	"encoding/json"

	"go_starter/internal/claudeai"
	"go_starter/internal/session"
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
//
// Implementasi tool account/subscription (jenaAccountIDInput, jenaSearchAccounts,
// jenaLoadAccount, jenaGetAccountSummary) di jena_ai_tools_accounts.go;
// get_subscription_status/list_my_deals/list_my_leads di jena_ai_tools_pipeline.go
// — dipisah krn ambang File Health (Route/Handler 150 baris).

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
