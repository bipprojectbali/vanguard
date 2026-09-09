package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5"
)

// cs_list_account_filter.go — BL-102: menyaring daftar Implementation Tracker /
// Training Schedule ke SATU desa lewat ?account={id} (entry point dari halaman
// Customer Success per-desa). Dipakai bersama CSImplTasksList & CSTrainingsList.

// csAccountFilter menerjemahkan ?account= opsional menjadi konteks filter:
//   - accountID: 0 = tak ada filter (tampilkan semua sesuai scope); >0 = saring.
//   - accountName: HANYA diisi bila F3 aktor mengizinkan desa ini (allows).
//     Nama desa di luar scope TAK boleh bocor lewat chip; kuerinya sendiri sudah
//     fail-closed (AND account_id = X di bawah gerbang ownership → nol baris).
//
// Param liar (bukan angka / ≤0) → abaikan filter, jangan 404: degradasi mulus ke
// daftar penuh (senada "cursor rusak → halaman pertama"). Desa tak ada
// (pgx.ErrNoRows) → filter tetap aktif (kueri nol baris), chip tanpa nama.
// ok=false → galat internal sudah ditulis ke w, handler harus berhenti.
func (h *Handler) csAccountFilter(w http.ResponseWriter, r *http.Request, allows func(db.Account) bool) (accountID int64, accountName string, ok bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("account"))
	if raw == "" {
		return 0, "", true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, "", true
	}
	ctx := r.Context()
	a, err := h.q(ctx).GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return id, "", true
		}
		h.Log.Error("cs account filter: get account", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return 0, "", false
	}
	if allows(a) {
		accountName = a.VillageName
	}
	return id, accountName, true
}
