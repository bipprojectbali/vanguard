package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
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
//
// Fungsi sort per-kolom (village/type/regency di sort_a.go, province/owner/csm/
// default di sort_b.go) dipisah krn ambang File Health yang sama.

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
	var ok bool
	switch sortCol {
	case "village":
		shown, nextCursor, ok = h.accountsSortByVillage(w, r, ctx, params, query, dir)
	case "type":
		shown, nextCursor, ok = h.accountsSortByType(w, r, ctx, params, query, dir)
	case "regency":
		shown, nextCursor, ok = h.accountsSortByRegency(w, r, ctx, params, query, dir, regions)
	case "province":
		shown, nextCursor, ok = h.accountsSortByProvince(w, r, ctx, params, query, dir, regions)
	case "owner":
		shown, nextCursor, ok = h.accountsSortByOwner(w, r, ctx, params, query, dir, names)
	case "csm":
		shown, nextCursor, ok = h.accountsSortByCsm(w, r, ctx, params, query, dir, names)
	default:
		shown, nextCursor, ok = h.accountsSortDefault(w, r, ctx, params)
	}
	if !ok {
		return
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
