package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// contacts_crud_test.go — jalur happy-path CRUD kontak + keunikan PRIMARY. Gerbang
// F2, warisan F3, dan masking F4 diuji di file lain (contacts_test.go hub &
// contacts_fls_test.go); di sini fokusnya penyimpanan benar + invarian
// idx_contacts_primary (maks 1 utama hidup per desa).

// TestContacts_CreateSuccess: sales membuat kontak di desanya → 303 ok=created,
// tersimpan dengan pembuat sebagai contact_owner & created_by, audit tercatat.
func TestContacts_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)

	form := contactFormValues("Budi")
	form.Set("last_name", "Santoso")
	form.Set("position_category", "Kepala Desa")
	form.Set("mobile_phone", "0812-1111-2222")
	req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts", form, itoa(a.ID), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	rows := env.liveContacts(t, a.ID)
	if len(rows) != 1 {
		t.Fatalf("harus 1 kontak, ada %d", len(rows))
	}
	c := rows[0]
	if c.FirstName != "Budi" || c.LastName == nil || *c.LastName != "Santoso" {
		t.Errorf("nama tak tersimpan: %q / %v", c.FirstName, c.LastName)
	}
	if c.ContactOwner == nil || *c.ContactOwner != uid {
		t.Errorf("pembuat harus jadi contact_owner awal, got %v", c.ContactOwner)
	}
	if c.CreatedBy == nil || *c.CreatedBy != uid {
		t.Errorf("created_by harus pembuat, got %v", c.CreatedBy)
	}
	env.assertAudited(t, "contact.create")
}

// TestContacts_CreateRejectsInvalid: nama depan kosong ditolak backend →
// redirect err=first_name + tak menyentuh DB.
func TestContacts_CreateRejectsInvalid(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa X", &uid, nil, nil)

	form := contactFormValues("") // nama depan wajib
	req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts", form, itoa(a.ID), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=first_name") {
		t.Errorf("harus err=first_name, got %q", loc)
	}
	if rows := env.liveContacts(t, a.ID); len(rows) != 0 {
		t.Errorf("input invalid tak boleh menyimpan, ada %d", len(rows))
	}
}

// TestContacts_UpdateSuccess: update sebagai admin → tersimpan, ok=saved, audit.
func TestContacts_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Lama", &uid, nil, nil)
	c := env.seedContact(t, a.ID, "Budi", &uid, false)

	form := contactFormValues("Budiman")
	form.Set("job_title", "Sekretaris")
	req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID),
		form, itoa(a.ID), itoa(c.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetContact(t.Context(), c.ID)
	if got.FirstName != "Budiman" || got.JobTitle == nil || *got.JobTitle != "Sekretaris" {
		t.Errorf("update tak tersimpan: %q / %v", got.FirstName, got.JobTitle)
	}
	env.assertAudited(t, "contact.update")
}

// TestContacts_SoftDelete: delete → hilang dari GetContact, ok=deleted, audit.
func TestContacts_SoftDelete(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Hapus", &uid, nil, nil)
	c := env.seedContact(t, a.ID, "Budi", &uid, false)

	req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID)+"/delete",
		url.Values{}, itoa(a.ID), itoa(c.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=deleted") {
		t.Errorf("harus ok=deleted, got %q (status %d)", loc, rec.Code)
	}
	if _, err := env.q.GetContact(t.Context(), c.ID); err == nil {
		t.Error("kontak ter-soft-delete tak boleh lagi terbaca GetContact")
	}
	env.assertAudited(t, "contact.delete")
}

// TestContacts_OptOutWritable: penanda kepatuhan (opt-out email / jangan hubungi)
// bisa di-set lewat form & tersimpan — kriteria M3 "opt-out tampil jelas" bermula
// dari bisa MENYIMPANNYA.
func TestContacts_OptOutWritable(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Patuh", &uid, nil, nil)

	form := contactFormValues("Budi")
	form.Set("email_opt_out", "1")
	form.Set("do_not_contact", "1")
	req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts", form, itoa(a.ID), "")
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	rows := env.liveContacts(t, a.ID)
	if len(rows) != 1 {
		t.Fatalf("harus 1 kontak, ada %d", len(rows))
	}
	if !rows[0].EmailOptOut || !rows[0].DoNotContact {
		t.Errorf("penanda opt-out harus tersimpan true, got opt=%v dnc=%v",
			rows[0].EmailOptOut, rows[0].DoNotContact)
	}
}

// --- Primary uniqueness ----------------------------------------------------

// TestContacts_PrimaryMenggantikanLama: membuat kontak baru sebagai utama
// mengosongkan utama sebelumnya — tepat 1 primary per desa (idx_contacts_primary).
func TestContacts_PrimaryMenggantikanLama(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Utama", &uid, nil, nil)
	first := env.seedContact(t, a.ID, "Pertama", &uid, true) // utama awal

	form := contactFormValues("Kedua")
	form.Set("is_primary_contact", "1")
	req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts", form, itoa(a.ID), "")
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create utama kedua gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	rows := env.liveContacts(t, a.ID)
	if n := countPrimary(rows); n != 1 {
		t.Fatalf("harus tepat 1 kontak utama, ada %d", n)
	}
	// Yang utama sekarang "Kedua"; "Pertama" sudah dikosongkan.
	got, _ := env.q.GetContact(t.Context(), first.ID)
	if got.IsPrimaryContact {
		t.Error("kontak utama lama harus dikosongkan saat utama baru dibuat")
	}
}

// TestContacts_SetPrimaryMemindah: ContactSetPrimary memindah penanda utama ke
// kontak lain — yang lama dikosongkan, tepat 1 primary tersisa.
func TestContacts_SetPrimaryMemindah(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Utama", &uid, nil, nil)
	old := env.seedContact(t, a.ID, "Lama", &uid, true)
	fresh := env.seedContact(t, a.ID, "Baru", &uid, false)

	req := contactsReq(http.MethodPost,
		"/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(fresh.ID)+"/primary",
		url.Values{}, itoa(a.ID), itoa(fresh.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactSetPrimary)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=primary") {
		t.Errorf("harus ok=primary, got %q (status %d)", loc, rec.Code)
	}

	rows := env.liveContacts(t, a.ID)
	if n := countPrimary(rows); n != 1 {
		t.Fatalf("harus tepat 1 kontak utama, ada %d", n)
	}
	gotOld, _ := env.q.GetContact(t.Context(), old.ID)
	gotNew, _ := env.q.GetContact(t.Context(), fresh.ID)
	if gotOld.IsPrimaryContact {
		t.Error("kontak utama lama harus dikosongkan")
	}
	if !gotNew.IsPrimaryContact {
		t.Error("kontak sasaran harus jadi utama")
	}
	env.assertAudited(t, "contact.primary")
}

// TestContacts_DeletePrimaryMembebaskanSlot: menghapus kontak utama membebaskan
// slot (index partial WHERE deleted_at IS NULL) — kontak utama baru bisa dibuat
// tanpa melanggar keunikan.
func TestContacts_DeletePrimaryMembebaskanSlot(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Utama", &uid, nil, nil)
	old := env.seedContact(t, a.ID, "Lama", &uid, true)

	// Hapus kontak utama.
	delReq := contactsReq(http.MethodPost,
		"/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(old.ID)+"/delete",
		url.Values{}, itoa(a.ID), itoa(old.ID))
	if rec := env.runAccount(uid, "owner", "admin", delReq, env.h.ContactDelete); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete gagal: %d", rec.Code)
	}

	// Kontak utama baru → tak boleh bentrok index (slot bebas).
	form := contactFormValues("Baru")
	form.Set("is_primary_contact", "1")
	newReq := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts", form, itoa(a.ID), "")
	if rec := env.runAccount(uid, "owner", "admin", newReq, env.h.ContactCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create utama baru setelah hapus harus sukses, got %d\n%s", rec.Code, rec.Body.String())
	}

	rows := env.liveContacts(t, a.ID)
	if n := countPrimary(rows); n != 1 {
		t.Errorf("setelah hapus+buat, harus tepat 1 kontak utama hidup, ada %d", n)
	}
}
