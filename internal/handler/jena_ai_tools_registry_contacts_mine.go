package handler

import (
	"encoding/json"

	"go_starter/internal/claudeai"
)

// jena_ai_tools_registry_contacts_mine.go — skema tool list_my_contacts
// (BL-162 fase 5), dipisah dari jena_ai_tools_registry.go (file health,
// CLAUDE.md §8; file itu sudah di ambang 150 baris). Murni DATA, sama seperti
// file induknya — implementasi F2/F3 tool ini ada di
// jena_ai_tools_contacts_mine.go.

// jenaContactsMineTools dipanggil dari jenaTools() (jena_ai_tools_registry.go)
// untuk menambah entri list_my_contacts ke allowlist tanpa menumbuhkan file
// registry utama.
func (h *Handler) jenaContactsMineTools() []claudeai.Tool {
	return []claudeai.Tool{
		{
			Name: "list_my_contacts",
			Description: "Daftar Kontak MILIK penanya sendiri — kontak di desa " +
				"(Account) yang Account Owner/CSM/backup CSM-nya penanya, apa pun " +
				"cakupan data perannya. Tanpa parameter. Untuk kontak MILIK ORANG " +
				"LAIN atau lintas-pemilik, pakai search_contacts.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}
