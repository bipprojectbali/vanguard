package handler

import (
	"context"
	"errors"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// success_plans_data.go — pemuatan data Success Plan: loadSuccessPlan (baca +
// guard 404) & opsi anggota dropdown. Dipisah dari success_plans.go agar file
// di bawah ambang tipe Route/Handler (150). Satu paket handler.
//
// Pemetaan baris/KPI/badge (view murni-data) di success_plans_rows.go.

// loadSuccessPlan memuat satu success plan + akun terkait dan menegakkan F3.
// Mengembalikan (plan, akun, true) jika berhasil; menulis respons dan
// mengembalikan false jika gagal.
func (h *Handler) loadSuccessPlan(w http.ResponseWriter, r *http.Request, id int64) (db.GetSuccessPlanRow, db.Account, bool) {
	ctx := r.Context()

	sp, err := h.q(ctx).GetSuccessPlan(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
		h.Log.Error("success_plans: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.GetSuccessPlanRow{}, db.Account{}, false
	}

	dataScope := session.BusinessDataScope(ctx)
	filter := db.SuccessPlansListFilterFor(dataScope)
	uid := session.UserID(ctx)

	// Muat akun: untuk VillageName (dipakai form edit) dan F3 ownership check.
	acct, err := h.q(ctx).GetAccount(ctx, sp.AccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
		h.Log.Error("success_plans: get account", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.GetSuccessPlanRow{}, db.Account{}, false
	}

	if !filter.ScopeAll {
		if !filter.IsOwn {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
		if !filter.Allows(uid, acct.AccountOwner, acct.AssignedCsm, acct.BackupCsm, sp.OwnerCsm) {
			http.NotFound(w, r)
			return db.GetSuccessPlanRow{}, db.Account{}, false
		}
	}
	return sp, acct, true
}

// successPlanMemberOptions merakit slice option anggota untuk dropdown owner_csm.
// Pola sama dengan csRenewalMemberOptions.
func (h *Handler) successPlanMemberOptions(ctx context.Context) ([]panel.SuccessPlanMemberOption, error) {
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	opts := make([]panel.SuccessPlanMemberOption, 0, len(memberRows))
	for _, m := range memberRows {
		name := m.Email
		if m.Name != nil && *m.Name != "" {
			name = *m.Name
		}
		opts = append(opts, panel.SuccessPlanMemberOption{ID: m.UserID, Name: name})
	}
	return opts, nil
}
