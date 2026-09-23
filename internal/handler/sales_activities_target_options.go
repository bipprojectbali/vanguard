package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_target_options.go — activityTargetOptions (picker target
// polimorfik). Dipisah dari sales_activities_options.go krn ambang File
// Health yang sama.

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
