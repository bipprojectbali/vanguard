package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_page.go — HALAMAN baca hub Desa (Account): daftar & detail. Aksi
// (buat/sunting/tugaskan/hapus) ada di accounts.go — dipisah karena keduanya
// tumbuh dengan aturan berbeda: halaman ini soal APA yang boleh DILIHAT & oleh
// SIAPA (F2 read + F3 ownership), aksi soal apa yang boleh DIUBAH (F2 write).
//
// TIGA sumbu izin bekerja bersama, tegak lurus (sistem-dan-role.md §3):
//   - F2 (Casbin bisnis): CanBusiness("crm:accounts","read") — GERBANG modul.
//     write mencakup read, jadi admin/manager/sales/csm lolos; support (read
//     saja) juga lolos; business_role "" → DITOLAK (deny-default, tak ada root
//     — super_admin/owner yang bukan pemegang peran CRM TIDAK otomatis masuk).
//   - F3 (ownership, layer app): AccountsListFilterFor(data_scope) → baris mana
//     yang tampil. data_scope 'none' (mis. Support) lolos gerbang read TAPI
//     ScopeNone → NOL baris; ia menyentuh desa hanya lewat konteks tiket, bukan
//     daftar desa umum. Cakupan dibaca dari kolom role, bukan ditebak dari nama.
//   - F4 (field-level, fls.go): dilakukan saat merender detail.
//
// RLS tetap mengisolasi WORKSPACE di bawah semua ini (h.q ber-tenant).

// AccountsList — GET /w/{workspace}/accounts. Daftar desa, keyset + filter
// kepemilikan F3. Ditolak (bukan pemegang peran CRM) → 403 + penjelasan, BUKAN
// 404: penerimanya sudah terbukti anggota workspace (Scope memvalidasinya), jadi
// menyangkal keberadaan halaman hanya membuat ia mengira ada yang rusak.
func (h *Handler) AccountsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewAccounts(ctx) {
		h.renderAccountsForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	// Tab (All/My/Belum-ada-Owner) HANYA untuk peran ScopeAll — Sales/CSM (ScopeOwn)
	// cuma lihat desanya (All≡My), jadi tab redundan & disembunyikan. Nilai view
	// dinormalkan: showTabs=false → "" (filter cakupan asli); liar → AccViewAll.
	showTabs := db.AccountsScopeFor(dataScope) == db.ScopeAll
	view := normalizeAccountsView(r.URL.Query().Get("view"), showTabs)
	// q = pencarian teks bebas (BL-6). Kosong → tak menyaring. TrimSpace agar
	// spasi murni tak jadi "%  %". MENYEMPITKAN di atas F3, tak pernah memperluas.
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// sort/dir (BL-157c): whitelist 6 kolom sortable (accountSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !accountSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	params := accountsListParams(dataScope, view, uid)
	params.Search = query

	names, err := h.accountMemberNames(ctx)
	if err != nil {
		h.Log.Error("accounts: member names", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Muat SEKALI (bukan per-baris — rule #13, ADR 0009): peta ancestry seluruh
	// master wilayah, dipakai resolve Regency/Province tiap baris di bawah —
	// TERMASUK kunci cursor sort Regency/Province (harus persis nilai tampil).
	regions, err := h.regionAncestryMap(ctx)
	if err != nil {
		h.Log.Error("accounts: region ancestry", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var shown []db.Account
	var nextCursor string
	switch sortCol {
	case "village":
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
			return
		}
		shown, nextCursor = splitPageText(rows, func(a db.Account) (string, int64) {
			return a.VillageName, a.ID
		})
	case "type":
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
			return
		}
		shown, nextCursor = splitPageText(rows, func(a db.Account) (string, int64) {
			return a.AccountType, a.ID
		})
	case "regency":
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
			return
		}
		// Kunci cursor = regionNames(regions, district_id) PERSIS yang tampil
		// (accountRowView) — bukan district_id mentah. "" (district_id kosong
		// ATAU tak ketemu di peta) dianggap kelompok NULL, sama dgn kondisi
		// rgc.name IS NULL di SQL (LEFT JOIN regions).
		shown, nextCursor = splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
			_, regency, _ := regionNames(regions, a.DistrictID)
			if regency == "" {
				return "", a.ID, true
			}
			return regency, a.ID, false
		})
	case "province":
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
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
			_, _, province := regionNames(regions, a.DistrictID)
			if province == "" {
				return "", a.ID, true
			}
			return province, a.ID, false
		})
	case "owner":
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
			return
		}
		// Kunci cursor Owner = nama/email resolusi peta anggota (memberName),
		// PERSIS yang ditampilkan — bukan account_owner mentah. NULL-nya
		// mengikuti account_owner asli (bukan string kosong hasil memberName),
		// sama dgn kondisi NULL di kunci sort SQL (LEFT JOIN users).
		shown, nextCursor = splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
			if a.AccountOwner == nil {
				return "", a.ID, true
			}
			return memberName(names, a.AccountOwner), a.ID, false
		})
	case "csm":
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
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(a db.Account) (string, int64, bool) {
			if a.AssignedCsm == nil {
				return "", a.ID, true
			}
			return memberName(names, a.AssignedCsm), a.ID, false
		})
	default:
		params.CursorCreatedAt, params.CursorID = pageCursor(r)
		// Ambil SATU lebih (pageSize+1): kelebihan itulah penanda "masih ada"
		// untuk splitPage — tanpanya tombol "Berikutnya" muncul di halaman
		// terakhir lalu berujung kosong.
		params.PageSize = pageSize + 1
		rows, err := h.q(ctx).ListAccounts(ctx, params)
		if err != nil {
			h.Log.Error("accounts: list", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPage(rows, func(a db.Account) (pgtype.Timestamptz, int64) {
			return a.CreatedAt, a.ID
		})
	}

	items := make([]panel.AccountRow, 0, len(shown))
	for _, a := range shown {
		items = append(items, accountRowView(a, names, regions))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Desa", "/accounts", panel.AccountsList(panel.AccountsListView{
		Base:       base,
		Items:      items,
		CanWrite:   canWriteAccounts(ctx),
		ShowTabs:   showTabs,
		ActiveView: view,
		Query:      query,
		NextCursor: nextCursor,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Msg:        accountsMsg(r.URL.Query().Get("ok")),
		Sort:       sortCol,
		Dir:        dir,
	}))
}

// accountSortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157c: 6 kolom tabel Accounts). ?sort= di luar daftar ini diperlakukan
// seolah absen (jatuh ke default created_at DESC), TAK error.
var accountSortableColumns = map[string]bool{
	"village":  true,
	"type":     true,
	"regency":  true,
	"province": true,
	"owner":    true,
	"csm":      true,
}

// renderAccountsForbidden — 403 + penjelasan bagi anggota yang membuka hub Desa
// tanpa peran CRM. Status ditulis SEBELUM body (WriteHeader setelah body tak
// berpengaruh) agar penolakan tak terkirim sebagai 200 yang tampak sukses.
func (h *Handler) renderAccountsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Desa", "/accounts", panel.AccountsForbidden())
}
