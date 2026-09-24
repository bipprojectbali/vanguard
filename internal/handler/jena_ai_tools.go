package handler

import (
	"context"
	"encoding/json"

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
// (README paket itu).
//
// Skema tool (nama+deskripsi+input schema) di jena_ai_tools_registry.go.
// Implementasi tool Account/Subscription (jenaAccountIDInput, jenaSearchAccounts,
// jenaLoadAccount, jenaGetAccountSummary) di jena_ai_tools_accounts.go;
// get_subscription_status di jena_ai_tools_pipeline.go; tool Deal/Lead
// (list_my_deals/list_my_leads/search_deals/search_leads, BL-162 fase 3) di
// jena_ai_tools_sales.go; tool Kontak (search_contacts/get_contact_summary,
// BL-162 fase 4) di jena_ai_tools_contacts.go — dipisah krn ambang File
// Health (Route/Handler 150 baris) yang sama.

const jenaSearchAccountsLimit = 5
const jenaMyDealsLimit = 10
const jenaMyLeadsLimit = 10
const jenaSearchDealsLimit = 10
const jenaSearchLeadsLimit = 10
const jenaSearchContactsLimit = 5

// jenaDispatch adalah claudeai.ToolDispatcher — switch ke satu fungsi per tool.
// Audit RINGAN per panggilan (nama tool saja, bukan input/hasil — gotcha
// #12/#15), dicatat terlepas dari tool itu akhirnya berhasil/ditolak F2/F3.
// Tool tak dikenal balas JSON error defensif — harusnya tak pernah terjadi
// karena `tools` yang dikirim ke model selalu dari jenaTools() (registry.go).
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
	case "search_contacts":
		return h.jenaSearchContacts(ctx, input)
	case "get_contact_summary":
		return h.jenaGetContactSummary(ctx, input)
	default:
		return `{"error":"tool tidak dikenal"}`, nil
	}
}
