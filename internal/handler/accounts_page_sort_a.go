package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
)

// accounts_page_sort_a.go — fungsi sort kolom village/type/regency untuk
// AccountsList (accounts_page.go). Dipisah krn ambang File Health
// (Route/Handler 150 baris); badan tiap fungsi = isi case switch semula
// APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) accountsSortByVillage(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListAccountsParams, query, dir string,
) ([]db.Account, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListAccountsSortByVillage(ctx, db.ListAccountsSortByVillageParams{
		HasCursor: hasCursor,
		Dir:       dir,
		CursorVal: cursorVal,
		CursorID:  cursorSortID,
		ScopeAll:  params.ScopeAll,
		IsSales:   params.IsSales,
		IsCsm:     params.IsCsm,
		Uid:       params.Uid,
		Unowned:   params.Unowned,
		Search:    query,
		PageSize:  pageSize + 1,
	})
	if err != nil {
		h.Log.Error("accounts: list sort village", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageText(rows, func(a db.Account) (string, int64) {
		return a.VillageName, a.ID
	})
	return shown, nextCursor, true
}

func (h *Handler) accountsSortByType(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListAccountsParams, query, dir string,
) ([]db.Account, string, bool) {
	// Tipe diurut RAW nilai kolom (customer/former_customer/prospect,
	// alfabetis) — sama keputusan "Status" Leads BL-157b, walau tampilan
	// pakai label Indonesia (accountTypeLabel).
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListAccountsSortByType(ctx, db.ListAccountsSortByTypeParams{
		HasCursor: hasCursor,
		Dir:       dir,
		CursorVal: cursorVal,
		CursorID:  cursorSortID,
		ScopeAll:  params.ScopeAll,
		IsSales:   params.IsSales,
		IsCsm:     params.IsCsm,
		Uid:       params.Uid,
		Unowned:   params.Unowned,
		Search:    query,
		PageSize:  pageSize + 1,
	})
	if err != nil {
		h.Log.Error("accounts: list sort type", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageText(rows, func(a db.Account) (string, int64) {
		return a.AccountType, a.ID
	})
	return shown, nextCursor, true
}

func (h *Handler) accountsSortByRegency(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListAccountsParams, query, dir string, regions map[int64]regionNode,
) ([]db.Account, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListAccountsSortByRegency(ctx, db.ListAccountsSortByRegencyParams{
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
		h.Log.Error("accounts: list sort regency", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	// Kunci cursor = regionNames(regions, district_id) PERSIS yang tampil
	// (accountRowView) — bukan district_id mentah. "" (district_id kosong
	// ATAU tak ketemu di peta) dianggap kelompok NULL, sama dgn kondisi
	// rgc.name IS NULL di SQL (LEFT JOIN regions).
	shown, nextCursor := splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
		_, regency, _ := regionNames(regions, a.DistrictID)
		if regency == "" {
			return "", a.ID, true
		}
		return regency, a.ID, false
	})
	return shown, nextCursor, true
}
