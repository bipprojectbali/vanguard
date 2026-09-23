package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_options.go — pembangun opsi picker (target polimorfik &
// kontak) untuk form Sales Activity. Dipecah dari sales_activities_new.go untuk
// file health. activityTargetOptions dipindah ke
// sales_activities_target_options.go — dipisah krn ambang yang sama.

// activityContactsForTarget memuat kontak MILIK DESA target yang dipilih (BL-164
// — sumber: user 15 Sep, Kontak sebelumnya selalu memuat seluruh kontak
// lintas-desa tak peduli Target). Menggantikan activityContactOptions lama.
//
// Gate F3 LEBIH DULU via targetInScope: ListContactsByAccount sendiri TAK
// menegakkan ownership (komentar query, gerbangnya cuma desa induk) — tanpa
// pengecekan ini, resolusi account_id dari Target sembarang user bisa
// membocorkan kontak desa di luar cakupan aktor. Target di luar cakupan / tak
// ada → kosong (nil, "", nil, nil), BUKAN error (form tetap tampil, field
// Kontak cuma kosong — pola sama targetInScope sendiri).
//
// Lead: TIDAK punya relasi ke tabel contacts (data kontak tersimpan mentah di
// kolom lead) → leadInfo diisi dari field lead itu sendiri, tak ada query
// kontak (return lebih awal).
func (h *Handler) activityContactsForTarget(ctx context.Context, targetType string, id int64) (
	contacts []panel.AccountMemberOption, preselect string, leadInfo *panel.LeadContactInfoView, err error,
) {
	ok, err := h.targetInScope(ctx, targetType, id)
	if err != nil {
		return nil, "", nil, err
	}
	if !ok {
		return nil, "", nil, nil
	}

	var accountID int64
	switch targetType {
	case "account":
		accountID = id
	case "deal":
		d, err := h.q(ctx).GetDeal(ctx, id)
		if err != nil {
			return nil, "", nil, err
		}
		accountID = d.AccountID
		if d.PrimaryContactID != nil {
			preselect = strconv.FormatInt(*d.PrimaryContactID, 10)
		}
	case "contact":
		c, err := h.q(ctx).GetContact(ctx, id)
		if err != nil {
			return nil, "", nil, err
		}
		accountID = c.AccountID
		preselect = strconv.FormatInt(id, 10)
	case "lead":
		l, err := h.q(ctx).GetLead(ctx, id)
		if err != nil {
			return nil, "", nil, err
		}
		return nil, "", &panel.LeadContactInfoView{
			Name:     deref(l.ContactPerson),
			Phone:    deref(l.MobilePhone),
			WhatsApp: deref(l.Whatsapp),
			Email:    deref(l.Email),
		}, nil
	default:
		return nil, "", nil, nil
	}

	at, cid := firstPageCursor()
	rows, err := h.q(ctx).ListContactsByAccount(ctx, db.ListContactsByAccountParams{
		AccountID: accountID, CursorCreatedAt: at, CursorID: cid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, "", nil, err
	}
	contacts = make([]panel.AccountMemberOption, 0, len(rows))
	for _, c := range rows {
		contacts = append(contacts, panel.AccountMemberOption{ID: c.ID, Label: fullName(c.FirstName, c.LastName)})
	}
	return contacts, preselect, nil, nil
}

// int32ContactStr memformat *int64 contact_id untuk prefill dropdown: nil → "".
func int32ContactStr(n *int64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatInt(*n, 10)
}
