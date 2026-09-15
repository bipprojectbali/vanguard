package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_options.go — pembangun opsi picker (target polimorfik &
// kontak) untuk form Sales Activity. Dipecah dari sales_activities_new.go untuk
// file health.

// activityTargetPickerLimit membatasi opsi per-tipe di dropdown target (guardrail:
// picker tak boleh memuat seluruh tabel). Named-const, bukan angka telanjang.
const activityTargetPickerLimit = 200

// activityTargetOptions memuat target dalam cakupan aktor (F3): deal + account +
// contact + lead (BL-160). Nilai opsi = "type:id" (dikonsumsi parseActivityTarget);
// label diberi awalan tipe agar tercampur jelas. Tiap tipe satu query berbatas
// (bukan N+1, bukan seluruh tabel). Label Lead diawali entity_code
// (leadPickerLabel) agar bisa dicari via kode di picker cari-ketik — bukan cuma
// nama (permintaan user 14 Sep, kembaran alasan accountPickerLabel utk Desa).
func (h *Handler) activityTargetOptions(ctx context.Context) ([]panel.ActivityTargetOption, error) {
	uid := session.UserID(ctx)
	scope := session.BusinessDataScope(ctx)
	at, cid := firstPageCursor()
	opts := make([]panel.ActivityTargetOption, 0, activityTargetPickerLimit)

	df := db.DealsListFilterFor(scope)
	deals, err := h.q(ctx).ListDeals(ctx, db.ListDealsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: df.ScopeAll, IsOwn: df.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	for _, d := range deals {
		opts = append(opts, panel.ActivityTargetOption{
			Value: "deal:" + strconv.FormatInt(d.ID, 10),
			Label: "Deal · " + d.DealName,
		})
	}

	af := db.AccountsListFilterFor(scope)
	accounts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: af.ScopeAll, IsSales: af.IsOwn, IsCsm: af.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		opts = append(opts, panel.ActivityTargetOption{
			Value: "account:" + strconv.FormatInt(a.ID, 10),
			Label: "Desa · " + accountPickerLabel(a.VillageCode, a.VillageName),
		})
	}

	contacts, err := h.q(ctx).ListContacts(ctx, db.ListContactsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: af.ScopeAll, IsSales: af.IsOwn, IsCsm: af.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	for _, c := range contacts {
		opts = append(opts, panel.ActivityTargetOption{
			Value: "contact:" + strconv.FormatInt(c.ID, 10),
			Label: "Kontak · " + fullName(c.FirstName, c.LastName),
		})
	}

	lf := db.LeadsListFilterFor(scope)
	leads, err := h.q(ctx).ListLeads(ctx, db.ListLeadsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: lf.ScopeAll, IsOwn: lf.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	for _, l := range leads {
		opts = append(opts, panel.ActivityTargetOption{
			Value: "lead:" + strconv.FormatInt(l.ID, 10),
			Label: "Lead · " + leadPickerLabel(l.EntityCode, l.LeadName),
		})
	}
	return opts, nil
}

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
