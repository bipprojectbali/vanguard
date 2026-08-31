package handler

import (
	"errors"
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
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

	params := accountsListParams(dataScope, view, uid)
	params.Search = query
	params.CursorCreatedAt, params.CursorID = pageCursor(r)
	// Ambil SATU lebih (pageSize+1): kelebihan itulah penanda "masih ada" untuk
	// splitPage — tanpanya tombol "Berikutnya" muncul di halaman terakhir lalu
	// berujung kosong.
	params.PageSize = pageSize + 1
	rows, err := h.q(ctx).ListAccounts(ctx, params)
	if err != nil {
		h.Log.Error("accounts: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	names, err := h.accountMemberNames(ctx)
	if err != nil {
		h.Log.Error("accounts: member names", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Muat SEKALI (bukan per-baris — rule #13, ADR 0009): peta ancestry seluruh
	// master wilayah, dipakai resolve Regency/Province tiap baris di bawah.
	regions, err := h.regionAncestryMap(ctx)
	if err != nil {
		h.Log.Error("accounts: region ancestry", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(a db.Account) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})

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
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Msg:        accountsMsg(r.URL.Query().Get("ok")),
	}))
}

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

// renderAccountsForbidden — 403 + penjelasan bagi anggota yang membuka hub Desa
// tanpa peran CRM. Status ditulis SEBELUM body (WriteHeader setelah body tak
// berpengaruh) agar penolakan tak terkirim sebagai 200 yang tampak sukses.
func (h *Handler) renderAccountsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Desa", "/accounts", panel.AccountsForbidden())
}
