package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"go_starter/internal/db"

	"github.com/go-chi/chi/v5"
)

// contacts_test.go — modul Kontak (orang di dalam sebuah desa) di sisi handler.
// Yang dijaga sama seperti accounts_test.go, PLUS dua invarian khas M3:
//
//   - F2 (Casbin bisnis "crm:contacts"): GET butuh "read", POST butuh "write".
//     Support lolos read tapi bukan write; business_role "" ditolak keduanya;
//     tenant role setinggi apa pun (owner/super_admin) tanpa peran CRM tetap 403.
//   - F3 DIWARISI DESA INDUK (bukan filter kontak sendiri): "kontak siapa yang
//     tampil" = "desa siapa yang tampil". Sales melihat kontak desa miliknya;
//     Support nol baris; Admin semua; kontak di luar cakupan → 404.
//   - PRIMARY: maks 1 kontak utama per desa (idx_contacts_primary). Set-primary
//     mengosongkan yang lama; menghapus kontak utama membebaskan slotnya.
//   - Opt-out (email_opt_out / do_not_contact) writable & tersimpan.
//   - F4 (field-level): HP & WhatsApp utuh HANYA Sales; editor non-Sales tak
//     pernah menerima nomor asli & masknya tak boleh menimpa nilai tersimpan.
//   - account_id URL WAJIB cocok c.account_id → mismatch 404 (URL nested jujur).
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// sesungguhnya diuji terpisah di rls_test.go.

// --- setup & helper --------------------------------------------------------

// contactsReq membangun request dengan chi param slug (+ {id} desa induk &
// {contactID} bila diberi) dan body form terkode. id/contactID "" = tanpa param.
func contactsReq(method, target string, form url.Values, id, contactID string) *http.Request {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "test")
	if id != "" {
		rctx.URLParams.Add("id", id)
	}
	if contactID != "" {
		rctx.URLParams.Add("contactID", contactID)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// seedContact menaruh satu kontak langsung lewat pool (bypass handler). owner =
// contact_owner (biasanya tak relevan untuk F3 kontak — kepemilikan diwarisi desa).
func (e *testEnv) seedContact(t *testing.T, accountID int64, firstName string, owner *int64, primary bool) db.Contact {
	t.Helper()
	c, err := e.q.CreateContact(t.Context(), db.CreateContactParams{
		TenantID:         e.tenantID,
		AccountID:        accountID,
		ContactOwner:     owner,
		FirstName:        firstName,
		IsPrimaryContact: primary,
		CreatedBy:        owner,
	})
	if err != nil {
		t.Fatalf("seed contact %s: %v", firstName, err)
	}
	return c
}

// seedContactPhone = seedContact + nomor HP/WhatsApp/kantor (untuk F4).
func (e *testEnv) seedContactPhone(t *testing.T, accountID int64, firstName, mobile, whatsapp, office string) db.Contact {
	t.Helper()
	c, err := e.q.CreateContact(t.Context(), db.CreateContactParams{
		TenantID:       e.tenantID,
		AccountID:      accountID,
		FirstName:      firstName,
		MobilePhone:    ptr(mobile),
		WhatsappNumber: ptr(whatsapp),
		OfficePhone:    ptr(office),
	})
	if err != nil {
		t.Fatalf("seed contact %s: %v", firstName, err)
	}
	return c
}

// liveContacts mendaftar SEMUA kontak hidup satu desa langsung dari pool — untuk
// membuktikan sebuah aksi menyimpan / tak menyimpan baris & menghitung primary.
func (e *testEnv) liveContacts(t *testing.T, accountID int64) []db.Contact {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListContactsByAccount(t.Context(), db.ListContactsByAccountParams{
		AccountID: accountID, CursorCreatedAt: at, CursorID: id, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	return rows
}

// countPrimary menghitung kontak utama hidup satu desa (invarian: ≤ 1).
func countPrimary(rows []db.Contact) int {
	n := 0
	for _, c := range rows {
		if c.IsPrimaryContact {
			n++
		}
	}
	return n
}

// contactFormValues merakit form minimal valid untuk create/update kontak.
func contactFormValues(firstName string) url.Values {
	return url.Values{"first_name": {firstName}}
}

var contactsAfterRe = regexp.MustCompile(`contacts\?after=([0-9]+_[0-9]+)`)

// extractContactsAfter menarik cursor ?after= dari href "Berikutnya" di body.
func extractContactsAfter(t *testing.T, body string) string {
	t.Helper()
	m := contactsAfterRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("tautan contacts ?after= tak ditemukan di body")
	}
	return m[1]
}

// --- F2: gerbang read/write ------------------------------------------------

// TestContacts_GateRead: siapa boleh MEMBUKA daftar kontak (act read). write
// mencakup read → admin/manager/sales/csm/support lolos; "" ditolak 403.
func TestContacts_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", true},
		{"csm", true},
		{"support", true}, // lolos gate read; F3 ScopeNone → nol baris (diuji terpisah)
		{"", false},       // tanpa peran CRM → 403 (deny-default, tak ada root)
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.ContactsAll)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menjelaskan butuh peran CRM")
				}
			}
		})
	}
}

// TestContacts_GateReadTegakLurusPlatform: owner/super_admin TANPA business_role
// tetap DITOLAK — sumbu bisnis tak mewarisi otoritas platform (§3).
func TestContacts_GateReadTegakLurusPlatform(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, tenantRole := range []string{"owner", "admin", "super_admin"} {
		req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
		rec := env.runAccount(uid, tenantRole, "", req, env.h.ContactsAll)
		if rec.Code != http.StatusForbidden {
			t.Errorf("tenant role %q tanpa peran CRM harus 403, got %d", tenantRole, rec.Code)
		}
	}
}

// TestContacts_GateWrite: POST create butuh act write. support (read saja) & ""
// ditolak; admin/sales lolos. Ditolak → 403, tak pernah menyentuh DB.
func TestContacts_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"sales", true},
		{"support", false}, // read-only di sumbu bisnis
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			a := env.seedAccount(t, "Desa Uji", &uid, nil, nil)
			form := contactFormValues("Budi")
			req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts", form, itoa(a.ID), "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.ContactCreate)

			rows := env.liveContacts(t, a.ID)
			if c.allow {
				if rec.Code != http.StatusSeeOther {
					t.Errorf("role %q harus create (303), got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				if len(rows) != 1 {
					t.Errorf("role %q create harus menyimpan 1 kontak, ada %d", c.role, len(rows))
				}
			} else {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if len(rows) != 0 {
					t.Errorf("role %q ditolak tak boleh menyimpan apa pun, ada %d", c.role, len(rows))
				}
			}
		})
	}
}

// --- CRUD happy path -------------------------------------------------------

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

// --- F3: kepemilikan DIWARISI desa induk -----------------------------------

// TestContacts_F3_SalesLihatKontakDesanya: di daftar global, Sales melihat HANYA
// kontak desa miliknya — kontak desa sales lain tak tampil (F3 diwarisi desa).
func TestContacts_F3_SalesLihatKontakDesanya(t *testing.T) {
	env, salesA := setupAccounts(t)
	salesB := env.seedMember(t, "salesb@local", "member", 0).ID

	mine := env.seedAccount(t, "Desa Milik A", &salesA, nil, nil)
	theirs := env.seedAccount(t, "Desa Milik B", &salesB, nil, nil)
	env.seedContact(t, mine.ID, "KontakA", &salesA, false)
	env.seedContact(t, theirs.ID, "KontakB", &salesB, false)

	req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
	body := env.runAccount(salesA, "member", "sales", req, env.h.ContactsAll).Body.String()
	if !strings.Contains(body, "KontakA") {
		t.Error("sales A harus melihat kontak desanya sendiri")
	}
	if strings.Contains(body, "KontakB") {
		t.Error("sales A TAK boleh melihat kontak desa milik sales B (F3 bocor)")
	}
}

// TestContacts_F3_SupportNolBaris: Support lolos gate read tapi ScopeNone →
// daftar global KOSONG walau ada kontak.
func TestContacts_F3_SupportNolBaris(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Ada", &uid, nil, nil)
	env.seedContact(t, a.ID, "KontakAda", &uid, false)

	req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("support lolos gate read, harus 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "KontakAda") {
		t.Error("support ScopeNone harus nol baris — kontak tak boleh tampil")
	}
}

// TestContacts_F3_AdminLihatSemua: Admin bisnis melihat SEMUA kontak lintas-desa.
func TestContacts_F3_AdminLihatSemua(t *testing.T) {
	env, sales := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	p := env.seedAccount(t, "Desa P", &sales, nil, nil)
	q := env.seedAccount(t, "Desa Q", &other, nil, nil)
	env.seedContact(t, p.ID, "KontakP", &sales, false)
	env.seedContact(t, q.ID, "KontakQ", &other, false)

	req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
	body := env.runAccount(sales, "owner", "admin", req, env.h.ContactsAll).Body.String()
	if !strings.Contains(body, "KontakP") || !strings.Contains(body, "KontakQ") {
		t.Error("admin bisnis harus melihat semua kontak di workspace")
	}
}

// TestContacts_F3_DetailLuarCakupan404: detail kontak yang desanya di luar cakupan
// Sales → 404 (menyangkal keberadaan, bukan 403). Warisan F3 lewat desa induk.
func TestContacts_F3_DetailLuarCakupan404(t *testing.T) {
	env, salesA := setupAccounts(t)
	salesB := env.seedMember(t, "salesb@local", "member", 0).ID
	theirs := env.seedAccount(t, "Desa Milik B", &salesB, nil, nil)
	c := env.seedContact(t, theirs.ID, "KontakB", &salesB, false)

	req := contactsReq(http.MethodGet, "/w/test/accounts/"+itoa(theirs.ID)+"/contacts/"+itoa(c.ID),
		nil, itoa(theirs.ID), itoa(c.ID))
	if rec := env.runAccount(salesA, "member", "sales", req, env.h.ContactDetail); rec.Code != http.StatusNotFound {
		t.Errorf("detail kontak di luar cakupan harus 404, got %d", rec.Code)
	}
}

// TestContacts_AccountMismatch404: URL nested menjanjikan kontak milik desa {id};
// kontak dari desa LAIN → 404 walau aktor (admin) boleh melihat kedua desa. Guard
// ini terjadi SEBELUM ownership — alamat yang berbohong ditolak apa adanya.
func TestContacts_AccountMismatch404(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa A", &uid, nil, nil)
	b := env.seedAccount(t, "Desa B", &uid, nil, nil)
	c := env.seedContact(t, a.ID, "KontakA", &uid, false) // milik desa A

	// URL menyebut desa B tapi contactID milik desa A → 404.
	req := contactsReq(http.MethodGet, "/w/test/accounts/"+itoa(b.ID)+"/contacts/"+itoa(c.ID),
		nil, itoa(b.ID), itoa(c.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactDetail); rec.Code != http.StatusNotFound {
		t.Errorf("account_id mismatch harus 404, got %d", rec.Code)
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

// --- F4: masking nomor -----------------------------------------------------

// TestContacts_F4_PhoneMasking: HP & WhatsApp utuh HANYA Sales; role lain menerima
// mask. Telepon KANTOR (kelembagaan) tak pernah disamarkan. Nilai asli tak boleh
// SAMPAI ke browser non-Sales (view-source).
func TestContacts_F4_PhoneMasking(t *testing.T) {
	env, uid := setupAccounts(t)
	mobile, whatsapp, office := "0812-3456-7890", "0813-0000-1111", "021-555-0000"
	// Owner DAN assigned-CSM = aktor, supaya keempat role (sales/admin/manager/csm)
	// sama-sama bisa MEMBUKA detailnya — di sini yang diuji masking F4, bukan F3.
	a := env.seedAccount(t, "Desa Kontak", &uid, &uid, nil)
	c := env.seedContactPhone(t, a.ID, "Budi", mobile, whatsapp, office)

	cases := []struct {
		role string
		full bool
	}{
		{"sales", true},
		{"admin", false},
		{"manager", false},
		{"csm", false},
	}
	for _, tc := range cases {
		t.Run("role="+tc.role, func(t *testing.T) {
			req := contactsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID),
				nil, itoa(a.ID), itoa(c.ID))
			body := env.runAccount(uid, "owner", tc.role, req, env.h.ContactDetail).Body.String()

			hasMobile := strings.Contains(body, mobile)
			hasWhatsapp := strings.Contains(body, whatsapp)
			if tc.full && (!hasMobile || !hasWhatsapp) {
				t.Errorf("sales harus melihat HP & WhatsApp utuh")
			}
			if !tc.full && (hasMobile || hasWhatsapp) {
				t.Errorf("role %q BOCOR — nomor pribadi sampai ke browser non-Sales", tc.role)
			}
			// Telepon kantor = kelembagaan → selalu tampil, semua role.
			if !strings.Contains(body, office) {
				t.Errorf("role %q: telepon kantor tak boleh disamarkan", tc.role)
			}
		})
	}
}

// TestContacts_F4_NonSalesTakBisaTimpaPhone: editor non-Sales mengirim mask (field
// terkunci), tapi handler mempertahankan nomor ASLI — mask tak menimpa tersimpan.
func TestContacts_F4_NonSalesTakBisaTimpaPhone(t *testing.T) {
	env, uid := setupAccounts(t)
	mobile, whatsapp := "0812-3456-7890", "0813-0000-1111"
	a := env.seedAccount(t, "Desa Kontak", &uid, nil, nil)
	c := env.seedContactPhone(t, a.ID, "Budi", mobile, whatsapp, "021-555-0000")

	// Admin menyunting: form mengirim mask (flsHidden) untuk kedua nomor pribadi.
	form := contactFormValues("Budi")
	form.Set("mobile_phone", flsHidden)
	form.Set("whatsapp_number", flsHidden)
	req := contactsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID),
		form, itoa(a.ID), itoa(c.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got, _ := env.q.GetContact(t.Context(), c.ID)
	if got.MobilePhone == nil || *got.MobilePhone != mobile {
		t.Errorf("HP asli harus dipertahankan, got %v (mask menimpa = F4 bocor)", got.MobilePhone)
	}
	if got.WhatsappNumber == nil || *got.WhatsappNumber != whatsapp {
		t.Errorf("WhatsApp asli harus dipertahankan, got %v", got.WhatsappNumber)
	}
}

// --- Keyset pagination -----------------------------------------------------

// TestContacts_KeysetPagination: dengan pageSize+1 kontak, daftar global halaman
// pertama menawarkan "Berikutnya"; mengikuti cursor menampilkan sisanya + "Ujung
// daftar." — baris ke-(pageSize+1) tetap terjangkau, tak hilang senyap.
func TestContacts_KeysetPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Banyak", &uid, nil, nil)
	total := pageSize + 1
	for i := 0; i < total; i++ {
		env.seedContact(t, a.ID, "Kontak"+itoa(int64(i)), &uid, false)
	}

	req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
	body := env.runAccount(uid, "owner", "admin", req, env.h.ContactsAll).Body.String()
	if !strings.Contains(body, "Berikutnya") {
		t.Fatal("halaman pertama dengan >pageSize kontak harus punya tautan Berikutnya")
	}
	if strings.Contains(body, "Ujung daftar.") {
		t.Error("halaman pertama (masih ada lanjutan) tak boleh menyatakan ujung daftar")
	}

	after := extractContactsAfter(t, body)
	req2 := contactsReq(http.MethodGet, "/w/test/contacts?after="+after, nil, "", "")
	body2 := env.runAccount(uid, "owner", "admin", req2, env.h.ContactsAll).Body.String()
	if !strings.Contains(body2, "Ujung daftar.") {
		t.Error("halaman terakhir harus menyatakan ujung daftar")
	}
	if strings.Contains(body2, "Berikutnya") {
		t.Error("halaman terakhir tak boleh menawarkan Berikutnya")
	}
}
