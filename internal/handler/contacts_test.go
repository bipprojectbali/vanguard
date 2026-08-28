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

// contacts_test.go — hub Kontak (orang di dalam sebuah desa) di sisi handler:
// setup/helper bersama + gerbang F2 (read/write) + keyset. CRUD & keunikan primary
// di contacts_crud_test.go; F3 (warisan desa) & F4 (masking) di contacts_fls_test.go.
// Yang dijaga sama seperti accounts_test.go, PLUS invarian khas M3:
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
//   - F4 (field-level): HP & WhatsApp utuh untuk Sales & Admin; role lain tak
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
