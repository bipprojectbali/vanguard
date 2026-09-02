package handler

import (
	"errors"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// accounts_detail_page.go — HALAMAN detail satu Desa (AccountDetail), dipisah
// dari accounts_page.go (daftar + renderAccountsForbidden) demi ambang tipe
// Route/Handler (150). Package sama; F2 read + F3 ownership identik.

// AccountDetail — GET /w/{workspace}/accounts/{id}. Satu desa. GetAccount tak
// menerapkan ownership (detail bisa dibuka lewat tautan langsung), jadi
// keputusan "boleh lihat baris ini?" diambil DI SINI lewat filter.Allows —
// kembaran per-baris dari list-query, sumber yang sama, jadi "tampil di daftar"
// & "boleh buka detail" tak pernah beda jawabannya.
//
// Di luar cakupan → 404 (menyangkal keberadaan), BUKAN 403 (yang mengakui desa
// itu ada di workspace lalu menolak — bocor halus).
func (h *Handler) AccountDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewAccounts(ctx) {
		h.renderAccountsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	a, err := h.q(ctx).GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("accounts: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), a.AccountOwner, a.AssignedCsm, a.BackupCsm) {
		http.NotFound(w, r)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, a.VillageName, "/accounts",
		panel.AccountDetail(h.accountDetailView(ctx, base, a)))
}
