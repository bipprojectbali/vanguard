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
// contact. Nilai opsi = "type:id" (dikonsumsi parseActivityTarget); label diberi
// awalan tipe agar tercampur jelas. Tiap tipe satu query berbatas (bukan N+1,
// bukan seluruh tabel).
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
			Label: "Desa · " + a.VillageName,
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
	return opts, nil
}

// activityContactOptions memuat kontak dalam cakupan aktor (F3 desa induk) untuk
// dropdown contact_id pada Call & Chat. Kind lain → nil (field tak dirender).
func (h *Handler) activityContactOptions(ctx context.Context, kind string) ([]panel.AccountMemberOption, error) {
	if kind != "call" && kind != "chat" {
		return nil, nil
	}
	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	at, cid := firstPageCursor()
	rows, err := h.q(ctx).ListContacts(ctx, db.ListContactsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: filter.ScopeAll, IsSales: filter.IsOwn, IsCsm: filter.IsOwn, Uid: &uid,
		PageSize: activityTargetPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	opts := make([]panel.AccountMemberOption, 0, len(rows))
	for _, c := range rows {
		opts = append(opts, panel.AccountMemberOption{ID: c.ID, Label: fullName(c.FirstName, c.LastName)})
	}
	return opts, nil
}

// int32ContactStr memformat *int64 contact_id untuk prefill dropdown: nil → "".
func int32ContactStr(n *int64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatInt(*n, 10)
}
