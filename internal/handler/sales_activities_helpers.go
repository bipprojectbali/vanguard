package handler

import (
	"context"
	"errors"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// sales_activities_helpers.go — loader F3 (loadOwnedActivity) & pemeriksa target
// polimorfik (targetInScope) dipakai lintas aksi Activity. Dipecah dari
// sales_activities.go untuk file health.

// loadOwnedActivity memuat satu aktivitas & menegakkan F3 (owner_id): di luar
// cakupan aktor → 404 (kembaran ActivityDetail). (activity, true) bila boleh.
func (h *Handler) loadOwnedActivity(w http.ResponseWriter, r *http.Request, id int64) (db.Activity, bool) {
	ctx := r.Context()
	a, err := h.q(ctx).GetActivity(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Activity{}, false
		}
		h.Log.Error("activities: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Activity{}, false
	}
	filter := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), a.OwnerID) {
		http.NotFound(w, r)
		return db.Activity{}, false
	}
	return a, true
}

// targetInScope memverifikasi target polimorfik (deal/account/contact) ADA &
// dalam cakupan F3 aktor — target_id bukan FK, jadi integritas ditegakkan di
// sini sebelum insert. (false, nil) = tak ada / di luar cakupan (tolak form);
// (false, err) = galat query (500-worthy). Contact memakai cakupan AKUN induk
// (tak ada ContactsListFilter tersendiri — visibilitas kontak ikut akunnya).
func (h *Handler) targetInScope(ctx context.Context, targetType string, id int64) (bool, error) {
	uid := session.UserID(ctx)
	scope := session.BusinessDataScope(ctx)
	switch targetType {
	case "deal":
		d, err := h.q(ctx).GetDeal(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return db.DealsListFilterFor(scope).Allows(uid, d.DealOwner), nil
	case "account":
		a, err := h.q(ctx).GetAccount(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return db.AccountsListFilterFor(scope).Allows(uid, a.AccountOwner, a.AssignedCsm, a.BackupCsm), nil
	case "contact":
		c, err := h.q(ctx).GetContact(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		a, err := h.q(ctx).GetAccount(ctx, c.AccountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		return db.AccountsListFilterFor(scope).Allows(uid, a.AccountOwner, a.AssignedCsm, a.BackupCsm), nil
	default:
		return false, nil
	}
}
