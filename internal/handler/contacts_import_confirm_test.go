package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// contacts_import_confirm_test.go — BL-134: bukti all-or-nothing di DB nyata
// (butuh TEST_DATABASE_URL, auto-skip via setupAccounts→setupTest bila kosong).
// Gerbang F2 & helper seed/CSV ada di contacts_import_test.go. Skenario:
//  1. Valid → tersimpan di desa yang benar, entity_code teralokasi, audit dua kali.
//  2. kode_desa tak dikenal → baris error → seluruh file ditolak.
//  3. kode_desa di luar cakupan F3 aktor → baris error → seluruh file ditolak.
//  4. Dua baris klaim primary utk account_id sama → ditolak seluruh file.
//  5. Klaim primary utk akun yang sudah punya primary hidup → ditolak seluruh file.
//  6. FLS: importer tanpa canEditPhone tetap berhasil menulis HP/WA dari CSV
//     (keputusan 14 Sep — canEditPhone SENGAJA tak dipanggil di jalur impor).
//
// Isolasi TENANT sesungguhnya TIDAK diuji ulang di sini secara khusus — koneksi
// test superuser bypass RLS (dikonfirmasi contacts_test.go), jadi tak bisa
// membuktikan RLS beneran; itu sudah dibuktikan generik sekali di rls_test.go
// utk SEMUA fitur (bukan per-fitur), pola yang sama diikuti BL-63
// (accounts_import_confirm_test.go tak punya test isolasi tenant sendiri).

func TestContactImportConfirm_AllValid(t *testing.T) {
	env, uid := setupAccounts(t)
	vs := villages(t, env, 2)
	a1 := env.seedAccountForImport(t, vs[0], &uid)
	a2 := env.seedAccountForImport(t, vs[1], &uid)

	rawCSV := contactImportHeaderLine + "\n" +
		contactImportRowCSV(vs[0].Code, "Budi", map[string]string{"is_primary_contact": "true"}) + "\n" +
		contactImportRowCSV(vs[1].Code, "Siti", nil) + "\n"

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", url.Values{"raw_csv": {rawCSV}}, "", "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "ok=imported") {
		t.Fatalf("harus redirect ok=imported (303), got %d %q\n%s", rec.Code, loc, rec.Body.String())
	}

	rows1 := env.liveContacts(t, a1.ID)
	rows2 := env.liveContacts(t, a2.ID)
	if len(rows1) != 1 || len(rows2) != 1 {
		t.Fatalf("harus 1 kontak per desa, ada %d & %d", len(rows1), len(rows2))
	}
	if rows1[0].EntityCode == nil || *rows1[0].EntityCode == "" {
		t.Error("entity_code harus teralokasi")
	}
	if !rows1[0].IsPrimaryContact {
		t.Error("baris pertama menandai primary — harus tersimpan sbg primary")
	}
	env.assertAudited(t, "contact.create")
	env.assertAudited(t, "contact.import")
}

func TestContactImportConfirm_UnknownVillage_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	a := env.seedAccountForImport(t, v, &uid)

	rawCSV := contactImportHeaderLine + "\n" +
		contactImportRowCSV(v.Code, "Budi", nil) + "\n" +
		contactImportRowCSV("99.99.99.9999", "TakDikenal", nil) + "\n" // kode desa fiktif

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", url.Values{"raw_csv": {rawCSV}}, "", "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "err=import_row_invalid") {
		t.Fatalf("harus redirect err=import_row_invalid, got %d %q", rec.Code, loc)
	}
	if len(env.liveContacts(t, a.ID)) != 0 {
		t.Error("satu baris tak valid harus menolak SELURUH file — nol baris tersimpan")
	}
}

func TestContactImportConfirm_ForeignVillageF3_NoRowsWritten(t *testing.T) {
	env, salesA := setupAccounts(t)
	salesB := env.seedMember(t, "salesb-import@local", "member", 0).ID
	vs := villages(t, env, 2)
	mine := env.seedAccountForImport(t, vs[0], &salesA)
	theirs := env.seedAccountForImport(t, vs[1], &salesB)

	rawCSV := contactImportHeaderLine + "\n" +
		contactImportRowCSV(vs[0].Code, "Budi", nil) + "\n" +
		contactImportRowCSV(vs[1].Code, "Diluar", nil) + "\n" // desa milik salesB, di luar cakupan salesA

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", url.Values{"raw_csv": {rawCSV}}, "", "")
	rec := env.runAccount(salesA, "owner", "sales", req, env.h.ContactImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "err=import_row_invalid") {
		t.Fatalf("harus redirect err=import_row_invalid, got %d %q", rec.Code, loc)
	}
	if len(env.liveContacts(t, mine.ID)) != 0 {
		t.Error("baris di luar cakupan F3 harus menolak SELURUH file — desa sendiri pun tak boleh tersimpan")
	}
	if len(env.liveContacts(t, theirs.ID)) != 0 {
		t.Error("desa di luar cakupan tak boleh kebagian kontak")
	}
}

func TestContactImportConfirm_PrimaryDupInFile_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	a := env.seedAccountForImport(t, v, &uid)

	rawCSV := contactImportHeaderLine + "\n" +
		contactImportRowCSV(v.Code, "Budi", map[string]string{"is_primary_contact": "true"}) + "\n" +
		contactImportRowCSV(v.Code, "Siti", map[string]string{"is_primary_contact": "true"}) + "\n"

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", url.Values{"raw_csv": {rawCSV}}, "", "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "err=import_row_invalid") {
		t.Fatalf("harus redirect err=import_row_invalid, got %d %q", rec.Code, loc)
	}
	if len(env.liveContacts(t, a.ID)) != 0 {
		t.Error("dua klaim primary utk desa sama harus menolak SELURUH file")
	}
}

func TestContactImportConfirm_PrimaryExists_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	vs := villages(t, env, 2)
	a1 := env.seedAccountForImport(t, vs[0], &uid)
	a2 := env.seedAccountForImport(t, vs[1], &uid)
	env.seedContact(t, a1.ID, "PrimaryLama", &uid, true) // sudah punya primary hidup

	rawCSV := contactImportHeaderLine + "\n" +
		contactImportRowCSV(vs[0].Code, "Baru", map[string]string{"is_primary_contact": "true"}) + "\n" +
		contactImportRowCSV(vs[1].Code, "Lain", nil) + "\n" // baris lain valid, tapi tetap ikut ditolak

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", url.Values{"raw_csv": {rawCSV}}, "", "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "err=import_row_invalid") {
		t.Fatalf("harus redirect err=import_row_invalid, got %d %q", rec.Code, loc)
	}
	if len(env.liveContacts(t, a1.ID)) != 1 {
		t.Error("desa dgn primary sudah ada tak boleh kebagian kontak baru dari impor ditolak")
	}
	if len(env.liveContacts(t, a2.ID)) != 0 {
		t.Error("baris lain yang valid pun harus ikut ditolak (all-or-nothing)")
	}
}

func TestContactImportConfirm_PrimaryInvalidValue_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	vs := villages(t, env, 2)
	a1 := env.seedAccountForImport(t, vs[0], &uid)
	a2 := env.seedAccountForImport(t, vs[1], &uid)

	// "0" BUKAN "true"/"false"/kosong — di bawah optBool form manual nilai
	// apa pun yang tak kosong dibaca true, tapi jalur impor (15 Sep) sengaja
	// lebih ketat: nilai selain 3 bentuk itu = baris invalid, bukan diam-diam
	// jadi true.
	rawCSV := contactImportHeaderLine + "\n" +
		contactImportRowCSV(vs[0].Code, "Budi", map[string]string{"is_primary_contact": "0"}) + "\n" +
		contactImportRowCSV(vs[1].Code, "Siti", nil) + "\n" // baris lain valid, tapi tetap ikut ditolak

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", url.Values{"raw_csv": {rawCSV}}, "", "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "err=import_row_invalid") {
		t.Fatalf("harus redirect err=import_row_invalid, got %d %q", rec.Code, loc)
	}
	if len(env.liveContacts(t, a1.ID)) != 0 {
		t.Error("nilai is_primary_contact tak dikenal harus menolak SELURUH file, bukan ditafsir true")
	}
	if len(env.liveContacts(t, a2.ID)) != 0 {
		t.Error("baris lain yang valid pun harus ikut ditolak (all-or-nothing)")
	}
}

func TestContactImportConfirm_FLSSkipped_PhoneSaved(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	a := env.seedAccountForImport(t, v, &uid)

	// admin TIDAK punya canEditPhone default (defaultEdit hanya true utk Sales,
	// internal/fls/fls.go) — tapi jalur impor SENGAJA tak pernah memanggil
	// canEditPhone (insertContact tak memanggilnya), jadi HP/WA CSV harus tetap
	// tersimpan apa adanya.
	rawCSV := contactImportHeaderLine + "\n" +
		contactImportRowCSV(v.Code, "Budi", map[string]string{
			"mobile_phone":    "081234567890",
			"whatsapp_number": "081234567890",
		}) + "\n"

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", url.Values{"raw_csv": {rawCSV}}, "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "ok=imported") {
		t.Fatalf("harus redirect ok=imported (303), got %d %q\n%s", rec.Code, loc, rec.Body.String())
	}

	rows := env.liveContacts(t, a.ID)
	if len(rows) != 1 {
		t.Fatalf("harus 1 kontak tersimpan, ada %d", len(rows))
	}
	if rows[0].MobilePhone == nil || *rows[0].MobilePhone != "081234567890" {
		t.Errorf("mobile_phone harus tersimpan apa adanya dari CSV (FLS dilewati di jalur impor), got %v", rows[0].MobilePhone)
	}
	if rows[0].WhatsappNumber == nil || *rows[0].WhatsappNumber != "081234567890" {
		t.Errorf("whatsapp_number harus tersimpan apa adanya dari CSV, got %v", rows[0].WhatsappNumber)
	}
}
