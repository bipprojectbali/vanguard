package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_deals_helpers.go — loader F3 & pembantu bersama (opsi dropdown, prefill
// form) dipakai lintas aksi Deal. Dipecah dari sales_deals.go untuk file health.

// loadOwnedDeal memuat satu deal & menegakkan F3: di luar cakupan aktor → 404
// (kembaran DealDetail). Mengembalikan (deal, true) bila boleh, atau menulis
// 404/500 & (zero, false).
func (h *Handler) loadOwnedDeal(w http.ResponseWriter, r *http.Request, id int64) (db.Deal, bool) {
	ctx := r.Context()
	d, err := h.q(ctx).GetDeal(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Deal{}, false
		}
		h.Log.Error("deals: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Deal{}, false
	}
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), d.DealOwner) {
		http.NotFound(w, r)
		return db.Deal{}, false
	}
	return d, true
}

// dealAccountOptions memuat desa dalam cakupan aktor (F3) sebagai pilihan dropdown
// form deal. Label = kode + nama (kode dulu agar terurut & jelas). Satu query
// berbatas (bukan N+1, bukan seluruh tabel).
func (h *Handler) dealAccountOptions(ctx context.Context) ([]panel.AccountMemberOption, error) {
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
		PageSize:        dealAccountPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	opts := make([]panel.AccountMemberOption, 0, len(rows))
	for _, a := range rows {
		label := a.VillageName
		if a.EntityCode != nil && *a.EntityCode != "" {
			label = *a.EntityCode + " — " + a.VillageName
		}
		opts = append(opts, panel.AccountMemberOption{ID: a.ID, Label: label})
	}
	return opts, nil
}

// dealFormFields memetakan Deal → nilai prefill form (semua string; nil → "").
func dealFormFields(d db.Deal) panel.DealFormFields {
	return panel.DealFormFields{
		DealName:          d.DealName,
		AccountID:         strconv.FormatInt(d.AccountID, 10),
		DealType:          deref(d.DealType),
		Amount:            moneyRupiahStr(d.Amount),
		Probability:       probabilityStr(d.Probability),
		ExpectedCloseDate: dateStr(d.ExpectedCloseDate),
		ForecastCategory:  deref(d.ForecastCategory),
		NextStep:          deref(d.NextStep),
		SubscriptionTerm:  deref(d.SubscriptionTerm),
		Competitor:        deref(d.Competitor),
	}
}
