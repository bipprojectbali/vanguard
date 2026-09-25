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

// jena_ai_tools_contacts.go — tool Jena AI modul Kontak (BL-162 fase 4),
// dipisah dari jena_ai_tools.go (file health, CLAUDE.md §8). Sama aturan
// F2/F3/F4 dari jena_ai_tools.go:16-31, tapi F3 di sini BUKAN filter kontak
// sendiri — MEWARISI kepemilikan desa (Account) INDUK, identik pola
// contacts_list.go/loadOwnedContactAccount:
//
//	F2 — canViewContacts(ctx) ("boleh buka modul Kontak"). SENGAJA terpisah
//	  arsitektural dari canViewAccounts — search_contacts/get_contact_summary
//	  TAK memeriksa hak atas modul Accounts, hanya modul Kontak (sama seperti
//	  loadOwnedAccount dipanggil canViewContacts-gated handler tanpa canViewAccounts).
//	F3 — db.AccountsListFilterFor(session.BusinessDataScope(ctx)).Allows(...)
//	  atas DESA INDUK kontak (bukan contact_owner) — "kontak siapa yang
//	  tampil" diturunkan dari "desa siapa yang tampil", satu kebenaran dengan
//	  ListContacts (queries/contacts.sql). Gagal → "tidak ditemukan", MENIRU
//	  pola 404-gate handler lain — jangan bocorkan ke model bahwa kontak itu
//	  ADA di desa/workspace lain.
//	F4 — maskPhone (nomor HP/WhatsApp) via canSeeFullPhone(ctx), reuse fls.go.
//	  Email & telepon kantor TAK disamarkan di app ini (contacts_view.go).

func (h *Handler) jenaSearchContacts(ctx context.Context, input json.RawMessage) (string, error) {
	if !canViewContacts(ctx) {
		return `{"error":"tidak berhak melihat data kontak"}`, nil
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
	rows, err := h.q(ctx).ListContacts(ctx, db.ListContactsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsSales:         filter.IsOwn,
		Uid:             &uid,
		IsCsm:           filter.IsOwn,
		Search:          in.Query,
		PageSize:        jenaSearchContactsLimit,
	})
	if err != nil {
		return "", fmt.Errorf("cari kontak: %w", err)
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

// jenaLoadContact memuat satu Contact + desa induknya sambil menegakkan F2
// (canViewContacts) dan F3 (kepemilikan desa induk) — dipakai oleh
// get_contact_summary. Cermin jenaLoadAccount (jena_ai_tools_accounts.go),
// tapi gate F2-nya modul Kontak, BUKAN modul Accounts (lihat header file ini).
func (h *Handler) jenaLoadContact(ctx context.Context, contactID int64) (db.Contact, db.Account, bool, error) {
	if !canViewContacts(ctx) {
		return db.Contact{}, db.Account{}, false, nil
	}
	c, err := h.q(ctx).GetContact(ctx, contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Contact{}, db.Account{}, false, nil
		}
		return db.Contact{}, db.Account{}, false, err
	}
	a, err := h.q(ctx).GetAccount(ctx, c.AccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Contact{}, db.Account{}, false, nil
		}
		return db.Contact{}, db.Account{}, false, err
	}
	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), a.AccountOwner, a.AssignedCsm, a.BackupCsm) {
		return db.Contact{}, db.Account{}, false, nil
	}
	return c, a, true, nil
}

func (h *Handler) jenaGetContactSummary(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		ContactID int64 `json:"contact_id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("input tidak valid: %w", err)
	}
	c, a, ok, err := h.jenaLoadContact(ctx, in.ContactID)
	if err != nil {
		return "", fmt.Errorf("ambil kontak: %w", err)
	}
	if !ok {
		return `{"error":"kontak tidak ditemukan"}`, nil
	}

	out := map[string]any{
		"name":            contactFullName(c),
		"village_name":    a.VillageName,
		"job_title":       deref(c.JobTitle),
		"is_primary":      c.IsPrimaryContact,
		"mobile_phone":    maskPhone(ctx, deref(c.MobilePhone)),
		"whatsapp_number": maskPhone(ctx, deref(c.WhatsappNumber)),
		"office_phone":    deref(c.OfficePhone),
		"email":           deref(c.Email),
	}
	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("format hasil: %w", err)
	}
	return string(result), nil
}
