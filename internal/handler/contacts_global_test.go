package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// contacts_global_test.go — jalur BUAT kontak dari daftar kontak GLOBAL
// (/w/{slug}/contacts), tanpa desa induk di URL. Yang dijaga khusus di sini —
// selain gerbang F2 yang sama dengan jalur nested (contacts_test.go):
//
//   - Desa induk datang dari FORM (account_id), lalu divalidasi ULANG lewat
//     loadOwnedAccount: id di luar cakupan aktor (F3) → 404, jadi memalsu id di
//     form tak menembus kepemilikan. Sukses → 303 + ok=created + audit.
//   - account_id kosong/rusak → redirect err=account_id, tak menyentuh DB.
//   - GET form: dropdown hanya berisi desa yang boleh DITULIS aktor; aktor tanpa
//     desa terpilih (mis. Sales belum punya desa) → 404 (tak ada induk).
//   - Tombol "Tambah Kontak" di daftar hanya muncul saat aktor boleh menulis DAN
//     punya ≥1 desa induk (canWrite); role read-only → tanpa tombol.

// contactsGlobalReq membangun request untuk jalur global: slug param, TANPA
// {id}/{contactID} desa induk (itu justru dipilih lewat form account_id).
func contactsGlobalReq(method, target string, form url.Values) *http.Request {
	return contactsReq(method, target, form, "", "")
}

// TestContactsGlobal_CreateSuccess: Sales membuat kontak dari daftar global
// dengan memilih desa miliknya → 303, ok=created, 1 baris tersimpan, audit.
func TestContactsGlobal_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)

	form := contactFormValues("Budi")
	form.Set("account_id", itoa(a.ID))
	form.Set("last_name", "Santoso")
	req := contactsGlobalReq(http.MethodPost, "/w/test/contacts", form)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactCreateGlobal)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	// Baris tersimpan DI BAWAH desa induk terpilih membuktikan account_id form
	// benar-benar dipakai sebagai induk (liveContacts disaring per desa).
	rows := env.liveContacts(t, a.ID)
	if len(rows) != 1 {
		t.Fatalf("harus 1 kontak di desa induk, ada %d", len(rows))
	}
	if rows[0].FirstName != "Budi" {
		t.Errorf("nama tak tersimpan: %q", rows[0].FirstName)
	}
	env.assertAudited(t, "contact.create")
}

// TestContactsGlobal_AccountOutOfScope404: Sales mem-POST account_id desa milik
// anggota LAIN → loadOwnedAccount 404 (F3), tak ada baris tersimpan. Memalsu id
// induk di form tak boleh menembus kepemilikan.
func TestContactsGlobal_AccountOutOfScope404(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "lain@local", "member", 0).ID
	theirs := env.seedAccount(t, "Desa Orang Lain", &other, nil, nil)

	form := contactFormValues("Penyusup")
	form.Set("account_id", itoa(theirs.ID))
	req := contactsGlobalReq(http.MethodPost, "/w/test/contacts", form)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactCreateGlobal)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("desa di luar cakupan harus 404, got %d", rec.Code)
	}
	if rows := env.liveContacts(t, theirs.ID); len(rows) != 0 {
		t.Errorf("tak boleh menyimpan kontak di desa luar cakupan, ada %d", len(rows))
	}
}

// TestContactsGlobal_MissingAccountID: account_id kosong/rusak → redirect
// err=account_id, tak menyentuh DB. Dua kasus (absen & non-angka) sama jalurnya.
func TestContactsGlobal_MissingAccountID(t *testing.T) {
	cases := map[string]string{
		"absen":     "",
		"non-angka": "abc",
		"nol":       "0",
	}
	for name, val := range cases {
		t.Run(name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			a := env.seedAccount(t, "Desa Uji", &uid, nil, nil)

			form := contactFormValues("Budi")
			if val != "" {
				form.Set("account_id", val)
			}
			req := contactsGlobalReq(http.MethodPost, "/w/test/contacts", form)
			rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactCreateGlobal)

			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=account_id") {
				t.Errorf("harus err=account_id, got %q (status %d)", loc, rec.Code)
			}
			if rows := env.liveContacts(t, a.ID); len(rows) != 0 {
				t.Errorf("account_id invalid tak boleh menyimpan, ada %d", len(rows))
			}
		})
	}
}

// TestContactsGlobal_GateWrite: POST global butuh act write (sama seperti jalur
// nested). support (read saja) & role "" ditolak 403 tanpa menyentuh DB;
// admin/sales lolos.
func TestContactsGlobal_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"sales", true},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			a := env.seedAccount(t, "Desa Uji", &uid, nil, nil)

			form := contactFormValues("Budi")
			form.Set("account_id", itoa(a.ID))
			req := contactsGlobalReq(http.MethodPost, "/w/test/contacts", form)
			rec := env.runAccount(uid, "owner", c.role, req, env.h.ContactCreateGlobal)

			rows := env.liveContacts(t, a.ID)
			if c.allow {
				if rec.Code != http.StatusSeeOther {
					t.Errorf("role %q harus create (303), got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				if len(rows) != 1 {
					t.Errorf("role %q harus menyimpan 1 kontak, ada %d", c.role, len(rows))
				}
			} else {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if len(rows) != 0 {
					t.Errorf("role %q ditolak tak boleh menyimpan, ada %d", c.role, len(rows))
				}
			}
		})
	}
}

// TestContactsGlobal_NewShowsDropdown: GET /contacts/new sebagai Sales dengan ≥1
// desa milik sendiri → 200 + <select name="account_id"> berisi nama desa itu.
func TestContactsGlobal_NewShowsDropdown(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Punyaku", &uid, nil, nil)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts/new", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactNewGlobal)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body:\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="account_id"`) {
		t.Error("form global harus punya dropdown name=\"account_id\"")
	}
	if !strings.Contains(body, "Desa Punyaku") {
		t.Error("dropdown harus memuat desa yang boleh ditulis aktor")
	}
}

// TestContactsGlobal_NewNoWritableAccount404: aktor boleh menulis tapi TAK punya
// desa (Sales tanpa desa) → 404 (tak ada induk untuk dilekati). Menjaga URL yang
// diketik langsung; tombolnya pun sudah disembunyikan.
func TestContactsGlobal_NewNoWritableAccount404(t *testing.T) {
	env, uid := setupAccounts(t)
	// Desa ini milik orang lain → di luar cakupan Sales → nol desa yang bisa ditulis.
	other := env.seedMember(t, "lain@local", "member", 0).ID
	env.seedAccount(t, "Desa Orang Lain", &other, nil, nil)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts/new", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactNewGlobal)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Sales tanpa desa harus 404, got %d", rec.Code)
	}
}

// TestContactsGlobal_ListButtonVisibility: tombol "Tambah Kontak" di daftar
// global muncul HANYA bila aktor boleh menulis DAN punya ≥1 desa induk.
//   - admin + ada desa → tombol tautan /contacts/new
//   - support (read-only) → tanpa tombol (canWrite false)
func TestContactsGlobal_ListButtonVisibility(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Uji", &uid, nil, nil)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts", nil)
	adminBody := env.runAccount(uid, "owner", "admin", req, env.h.ContactsAll).Body.String()
	if !strings.Contains(adminBody, "/contacts/new") || !strings.Contains(adminBody, "Tambah Kontak") {
		t.Error("admin dengan desa harus melihat tombol Tambah Kontak → /contacts/new")
	}

	req2 := contactsGlobalReq(http.MethodGet, "/w/test/contacts", nil)
	supportBody := env.runAccount(uid, "owner", "support", req2, env.h.ContactsAll).Body.String()
	if strings.Contains(supportBody, "/contacts/new") {
		t.Error("support (read-only) tak boleh melihat tombol Tambah Kontak")
	}
}
