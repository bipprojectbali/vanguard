package handler

import (
	"net/http"

	"go_starter/internal/ui/pages/panel"
)

// accounts.go — AKSI TULIS atas desa (Account): gerbang tulis, form buat, create.
// Sunting/update di accounts_update.go, penugasan CSM & soft-delete di
// accounts_assign.go, helper muat+prefill di accounts_helpers.go. Dipisah dari
// accounts_page.go (baca): aksi tumbuh dengan aturan TULIS (F2 write), halaman
// dengan aturan LIHAT (F2 read + F3).
//
// Gerbang SEMUA aksi = CanBusiness("crm:accounts","write") — support (read saja)
// DITOLAK menulis. Ownership F3 TIDAK menjaga tulis di M2-3: siapa pun pemegang
// peran tulis boleh membuat/menyunting desa mana pun di workspace-nya (batas
// per-baris untuk edit menyusul bila dibutuhkan). RLS tetap mengurung workspace.

// sqlStateUniqueViolation/sqlStateForeignKeyViolation = kode Postgres yang
// dikenali khusus. UNIQUE → tabrakan village_code (idx_accounts_code) atau
// entity_code override manual (idx_accounts_entity_code); FK → district_id yang
// dikirim klien sudah tak ada di master regions (mis. race dgn migrasi data, atau
// payload dipalsukan) — keduanya pesan spesifik, bukan "internal error" yang
// menyembunyikan sebab yang bisa diperbaiki user.
const (
	sqlStateUniqueViolation     = "23505"
	sqlStateForeignKeyViolation = "23503"
)

// requireAccountWrite = gerbang tulis bersama. Mengembalikan false & menulis
// respons penolakan bila aktor tak berhak. Read-only workspace (arsip) juga
// ditolak di sini: gerbang lifecycle memblokir POST, tapi menolak lebih awal
// dengan pesan jelas lebih baik daripada 403 telanjang dari middleware.
func (h *Handler) requireAccountWrite(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	if !canWriteAccountsPerm(ctx) {
		h.renderAccountsForbidden(w, r)
		return false
	}
	return true
}

// AccountNew — GET /w/{workspace}/accounts/new. Form kosong untuk membuat desa.
func (h *Handler) AccountNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	base := wsPath(slugFromRequest(r), "")
	v := panel.AccountFormView{
		Base:            base,
		Action:          base + "/accounts",
		IsEdit:          false,
		Err:             wsErrMsg(r.URL.Query().Get("err")),
		RegionsJSON:     h.regionsJSON(ctx),
		VillagesURL:     base + "/accounts/villages",
		Types:           accountTypeOptions,
		Statuses:        villageStatusOptions,
		Classifications: classificationOptions,
	}
	h.renderWorkspaceShell(w, r, "Tambah Desa", "/accounts", panel.AccountForm(v))
}
