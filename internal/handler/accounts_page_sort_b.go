package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_page_sort_b.go — fungsi sort kolom province/owner/csm + jalur
// default (created_at DESC) untuk AccountsList (accounts_page.go). Dipisah
// krn ambang File Health (Route/Handler 150 baris); badan tiap fungsi = isi
// case switch semula APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) accountsSortByProvince(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListAccountsParams, query, dir string, regions map[int64]regionNode,
) ([]db.Account, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListAccountsSortByProvince(ctx, db.ListAccountsSortByProvinceParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     params.ScopeAll,
		IsSales:      params.IsSales,
		IsCsm:        params.IsCsm,
		Uid:          params.Uid,
		Unowned:      params.Unowned,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("accounts: list sort province", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
		_, _, province := regionNames(regions, a.DistrictID)
		if province == "" {
			return "", a.ID, true
		}
		return province, a.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) accountsSortByOwner(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListAccountsParams, query, dir string, names map[int64]string,
) ([]db.Account, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListAccountsSortByOwner(ctx, db.ListAccountsSortByOwnerParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     params.ScopeAll,
		IsSales:      params.IsSales,
		IsCsm:        params.IsCsm,
		Uid:          params.Uid,
		Unowned:      params.Unowned,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("accounts: list sort owner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	// Kunci cursor Owner = nama/email resolusi peta anggota (memberName),
	// PERSIS yang ditampilkan — bukan account_owner mentah. NULL-nya
	// mengikuti account_owner asli (bukan string kosong hasil memberName),
	// sama dgn kondisi NULL di kunci sort SQL (LEFT JOIN users).
	shown, nextCursor := splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
		if a.AccountOwner == nil {
			return "", a.ID, true
		}
		return memberName(names, a.AccountOwner), a.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) accountsSortByCsm(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListAccountsParams, query, dir string, names map[int64]string,
) ([]db.Account, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListAccountsSortByCsm(ctx, db.ListAccountsSortByCsmParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     params.ScopeAll,
		IsSales:      params.IsSales,
		IsCsm:        params.IsCsm,
		Uid:          params.Uid,
		Unowned:      params.Unowned,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("accounts: list sort csm", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
		if a.AssignedCsm == nil {
			return "", a.ID, true
		}
		return memberName(names, a.AssignedCsm), a.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) accountsSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListAccountsParams,
) ([]db.Account, string, bool) {
	params.CursorCreatedAt, params.CursorID = pageCursor(r)
	// Ambil SATU lebih (pageSize+1): kelebihan itulah penanda "masih ada"
	// untuk splitPage — tanpanya tombol "Berikutnya" muncul di halaman
	// terakhir lalu berujung kosong.
	params.PageSize = pageSize + 1
	rows, err := h.q(ctx).ListAccounts(ctx, params)
	if err != nil {
		h.Log.Error("accounts: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPage(rows, func(a db.Account) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})
	return shown, nextCursor, true
}

// renderAccountsForbidden — 403 + penjelasan bagi anggota yang membuka hub Desa
// tanpa peran CRM. Status ditulis SEBELUM body (WriteHeader setelah body tak
// berpengaruh) agar penolakan tak terkirim sebagai 200 yang tampak sukses.
func (h *Handler) renderAccountsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Desa", "/accounts", panel.AccountsForbidden())
}
