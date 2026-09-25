package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// jena_ai_tools_contacts_mine.go — tool list_my_contacts (BL-162 fase 5),
// dipisah dari jena_ai_tools_contacts.go (file health, CLAUDE.md §8; file itu
// sudah di ambang 150 baris dgn search_contacts+get_contact_summary). Sama
// aturan F2/F3/F4 jena_ai_tools.go:16-31 & header jena_ai_tools_contacts.go:
//
//	F2 — canViewContacts(ctx), identik search_contacts/get_contact_summary.
//	F3 — SENGAJA memaksa IsSales=true & IsCsm=true (literal "milik SAYA"),
//	  TERLEPAS dari data_scope peran penanya — pola sama list_my_deals/
//	  list_my_leads (jena_ai_tools_sales.go). Manager/Admin (ScopeAll) TETAP
//	  hanya melihat kontak di desa yang account_owner/assigned_csm/backup_csm-
//	  nya uid sendiri lewat tool ini; untuk lintas-pemilik pakai search_contacts.
//	F4 — tak berlaku: bentuk baris (id+nama+desa) sama search_contacts, tanpa
//	  field sensitif (nomor HP/WA), jadi tak ada yang perlu disamarkan di sini.
func (h *Handler) jenaListMyContacts(ctx context.Context) (string, error) {
	if !canViewContacts(ctx) {
		return `{"error":"tidak berhak melihat data kontak"}`, nil
	}

	uid := session.UserID(ctx)
	cursorAt, cursorID := firstPageCursor()
	rows, err := h.q(ctx).ListContacts(ctx, db.ListContactsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        false,
		IsSales:         true,
		Uid:             &uid,
		IsCsm:           true,
		Search:          "",
		PageSize:        jenaMyContactsLimit,
	})
	if err != nil {
		return "", fmt.Errorf("daftar kontak saya: %w", err)
	}

	type row struct {
		ContactID   int64  `json:"contact_id"`
		Name        string `json:"name"`
		VillageName string `json:"village_name"`
	}
	results := make([]row, 0, len(rows))
	for _, c := range rows {
		results = append(results, row{
			ContactID:   c.ID,
			Name:        fullName(c.FirstName, c.LastName),
			VillageName: c.VillageName,
		})
	}
	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(out), nil
}
