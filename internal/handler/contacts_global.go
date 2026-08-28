package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// contacts_global.go — jalur BUAT kontak dari daftar kontak GLOBAL (/w/{slug}/
// contacts), tanpa desa induk di URL. Beda dari jalur nested (contacts.go):
// desa induk BELUM diketahui, jadi form menampilkan dropdown pemilih desa —
// hanya desa yang boleh DITULIS aktor (F3), diturunkan dari sumber yang SAMA
// dengan daftar (AccountsListFilter). account_id datang dari FORM, lalu
// divalidasi ulang lewat loadOwnedAccount (di luar cakupan → 404), jadi memalsu
// id di form tak bisa menembus F3. Insert & audit dibagi dengan jalur nested
// (insertContact).

// writableAccountSelectParams merakit flag ownership untuk Count/ListAccounts-
// ForSelect dari cakupan CRM aktor. Sumber SAMA dengan daftar (AccountsListFilter):
// IsOwn → KEDUA flag SQL (is_sales/is_csm) true (union kepemilikan). ScopeNone
// (Support/role kosong) → semua flag false → NOL desa (fail-closed).
func writableAccountSelectParams(ctx context.Context) db.ListAccountsForSelectParams {
	uid := session.UserID(ctx)
	f := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	return db.ListAccountsForSelectParams{
		ScopeAll: f.ScopeAll,
		IsSales:  f.IsOwn,
		IsCsm:    f.IsOwn,
		Uid:      &uid,
	}
}

// hasWritableAccount = aktor punya ≥1 desa yang boleh ditulis (gerbang tombol
// "Tambah Kontak" di daftar global). Murah (count + RLS satu workspace); tak
// memuat baris ke handler daftar.
func (h *Handler) hasWritableAccount(ctx context.Context) (bool, error) {
	p := writableAccountSelectParams(ctx)
	n, err := h.q(ctx).CountAccountsForSelect(ctx, db.CountAccountsForSelectParams{
		ScopeAll: p.ScopeAll, IsSales: p.IsSales, IsCsm: p.IsCsm, Uid: p.Uid,
	})
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// writableAccountOptions = daftar desa (id+nama) yang boleh ditulis aktor, untuk
// <option> dropdown. Urut nama (query) agar dropdown terbaca.
func (h *Handler) writableAccountOptions(ctx context.Context) ([]panel.AccountOption, error) {
	rows, err := h.q(ctx).ListAccountsForSelect(ctx, writableAccountSelectParams(ctx))
	if err != nil {
		return nil, err
	}
	opts := make([]panel.AccountOption, 0, len(rows))
	for _, r := range rows {
		opts = append(opts, panel.AccountOption{ID: r.ID, Name: r.VillageName})
	}
	return opts, nil
}

// ContactNewGlobal — GET /w/{workspace}/contacts/new. Form BUAT kontak dengan
// pemilih desa induk. Tanpa desa yang bisa ditulis (cakupan kosong) → 404: tak
// ada induk untuk dilekati, jadi halaman form tak punya arti (tombol pun sudah
// disembunyikan; ini menjaga URL yang diketik langsung).
func (h *Handler) ContactNewGlobal(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()
	accounts, err := h.writableAccountOptions(ctx)
	if err != nil {
		h.Log.Error("contacts: list accounts for select", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(accounts) == 0 {
		http.NotFound(w, r)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Tambah Kontak", "/contacts", panel.ContactForm(panel.ContactFormView{
		Action:   base + "/contacts",
		ListHref: base + "/contacts",
		IsEdit:   false,
		Err:      wsErrMsg(r.URL.Query().Get("err")),
		// F4: hanya Sales (canEditPhone) boleh mengisi HP/WhatsApp — sejajar jalur
		// nested & form EDIT.
		PhoneEditable: canEditPhone(ctx),
		Accounts:      accounts,
		Positions:     contactPositionOptions,
		Roles:         contactRoleOptions,
		Channels:      contactChannelOptions,
	}))
}

// ContactCreateGlobal — POST /w/{workspace}/contacts. Membuat kontak; desa induk
// dari FORM (account_id). F3 ditegakkan loadOwnedAccount: id di luar cakupan
// aktor → 404 (tak bisa memalsu induk). Gagal validasi form → kembali ke form
// global dengan ?err=. Sukses → detail kontak.
func (h *Handler) ContactCreateGlobal(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()

	accountID, err := strconv.ParseInt(r.FormValue("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		wsRedirect(w, r, "/contacts/new", "account_id")
		return
	}
	// F3: desa harus ada & dalam cakupan aktor — di luar cakupan → 404 (loadOwnedAccount).
	if _, ok := h.loadOwnedAccount(w, r, accountID); !ok {
		return
	}

	form, errCode := parseContactForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/contacts/new", errCode)
		return
	}

	c, err := h.insertContact(ctx, accountID, form)
	if err != nil {
		h.Log.Error("contacts: create global", "err", err)
		wsRedirect(w, r, "/contacts/new", "failed")
		return
	}
	wsRedirectOK(w, r, contactPath(accountID, c.ID), "created")
}
