package handler

import (
	"net/http"
	"strings"
	"testing"
)

// contacts_import_flash_test.go — BL-134 follow-up (16 Sep): regresi toast
// impor CSV Kontak. Perilaku (contactsMsg("imported") sudah punya kasus,
// ContactsAll merender ui.Toast dari v.Msg/v.Err) SUDAH berfungsi sejak
// awal — beda dari bug leadsMsg (BL-133, lihat
// sales_leads_import_confirm_flash_test.go) — tapi belum ada test yang
// menguncinya. Mirror gaya file itu: GET halaman TUJUAN redirect dgn
// ?ok=/?err= terpasang manual (bukan mengikuti Location dari confirm —
// sudah dibuktikan contacts_import_confirm_test.go), assert body memuat
// pesan yang dipetakan.

// TestContactsAll_RendersImportOKFlash: GET /contacts?ok=imported (tujuan
// PRG ContactImportConfirm, contacts_import_confirm.go) memuat toast sukses
// — bukan menelan kode ok= senyap.
func TestContactsAll_RendersImportOKFlash(t *testing.T) {
	env, uid := setupAccounts(t)

	req := contactsReq(http.MethodGet, "/w/test/contacts?ok=imported", nil, "", "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if want := contactsMsg("imported"); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("daftar kontak harus memuat toast sukses %q, body tak mengandungnya", want)
	}
}

// TestContactImportForm_RendersImportErrFlash: GET /contacts/import?err=<code>
// (tujuan PRG saat baris CSV ditolak, contacts_import_confirm.go &
// contacts_import_preview.go) memuat toast galat yg dipetakan wsErrMsgImport
// — bukan halaman form polos yang menelan kode err= senyap.
func TestContactImportForm_RendersImportErrFlash(t *testing.T) {
	env, uid := setupAccounts(t)

	req := contactsReq(http.MethodGet, "/w/test/contacts/import?err=import_row_invalid", nil, "", "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactImportForm)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if want := wsErrMsg("import_row_invalid"); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("form impor kontak harus memuat toast galat %q, body tak mengandungnya", want)
	}
}
