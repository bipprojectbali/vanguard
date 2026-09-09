package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/ui/pages/panel"
)

// contacts_fls_test.go — dua sumbu keamanan kontak yang bekerja tegak lurus F2:
//
//   - F3 (ownership) DIWARISI DESA INDUK: kontak yang tampil = desa yang tampil.
//     Sales lihat kontak desanya; Support nol baris; Admin semua; luar cakupan &
//     account_id mismatch → 404 (menyangkal keberadaan, bukan 403).
//   - F4 (field-level): HP & WhatsApp utuh untuk Sales & Admin; role lain menerima mask &
//     masknya tak boleh menimpa nomor asli. Telepon kantor tak pernah disamarkan.

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

// --- F4: masking nomor -----------------------------------------------------

// TestContacts_F4_PhoneMasking: HP & WhatsApp utuh untuk Sales & Admin; role lain
// (Manager, CSM) menerima mask. Telepon KANTOR (kelembagaan) tak pernah disamarkan. Nilai asli tak boleh
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
		{"admin", true}, // Admin CRM = pengelola workspace → akses penuh HP/WA
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
				t.Errorf("role %q harus melihat HP & WhatsApp utuh", tc.role)
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

// TestContactDetailView_PhoneMasked_Support: HP & WhatsApp tersamar utk
// Support — melengkapi matriks F4_PhoneMasking di atas (yang tak bisa
// mencakup Support: F3 ScopeNone, TestContacts_F3_SupportNolBaris, warisan
// desa induk membuat Support tak pernah lolos ke detail kontak lewat HTTP
// nyata). Diuji LANGSUNG atas contactDetailView, pola sama
// TestAccountDetailView_VillageBudgetMasked. Gap M9-2.
func TestContactDetailView_PhoneMasked_Support(t *testing.T) {
	env, uid := setupAccounts(t)
	mobile, whatsapp, office := "0812-3456-7890", "0813-0000-1111", "021-555-0000"
	a := env.seedAccount(t, "Desa Kontak Support", &uid, nil, nil)
	c := env.seedContactPhone(t, a.ID, "Budi", mobile, whatsapp, office)

	var got panel.ContactDetailView
	req := contactsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID),
		nil, itoa(a.ID), itoa(c.ID))
	env.runAccount(uid, "owner", "support", req, func(w http.ResponseWriter, r *http.Request) {
		got = env.h.contactDetailView(r.Context(), "", "", a.VillageName, c, nil, "")
	})
	if got.MobilePhone != flsHidden || got.WhatsappNumber != flsHidden {
		t.Errorf("support: HP/WhatsApp harus tersamar (%s), got %q/%q", flsHidden, got.MobilePhone, got.WhatsappNumber)
	}
	if got.MobilePhone == mobile || got.WhatsappNumber == whatsapp {
		t.Errorf("support: nomor pribadi mentah BOCOR")
	}
	if got.OfficePhone != office {
		t.Errorf("support: telepon kantor tak boleh disamarkan, got %q", got.OfficePhone)
	}
}

// TestContacts_F4_OverrideLewatSettings: BL-107 end-to-end. Manager default TAK
// melihat nomor; setelah admin memberi Manager "lihat" lewat halaman Field Security,
// permintaan detail Manager BERIKUTNYA menampilkan nomor utuh — bukti config
// per-tenant mengalir dari POST Settings → cache → masking handler, tanpa restart.
// Pakai setupRoles: WorkspaceFieldSecurityUpdate menulis baris ber-FK ke
// business_roles, jadi peran wajib tertanam di DB.
func TestContacts_F4_OverrideLewatSettings(t *testing.T) {
	env, uid := setupRoles(t)
	mobile, whatsapp, office := "0812-3456-7890", "0813-0000-1111", "021-555-0000"
	a := env.seedAccount(t, "Desa Override", &uid, nil, nil)
	c := env.seedContactPhone(t, a.ID, "Budi", mobile, whatsapp, office)

	detailReq := func() *http.Request {
		return contactsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID),
			nil, itoa(a.ID), itoa(c.ID))
	}

	// Default: Manager (DataScopeAll → bisa membuka) menerima mask.
	body := env.runAccount(uid, "owner", "manager", detailReq(), env.h.ContactDetail).Body.String()
	if strings.Contains(body, mobile) || strings.Contains(body, whatsapp) {
		t.Fatal("prakondisi: Manager default harus menerima mask")
	}

	// Admin memberi Manager "lihat" lewat halaman Settings.
	form := url.Values{"view.manager": {"1"}}
	post := rolesReq(http.MethodPost, "/w/test/field-security", form, "")
	if rec := env.runAccount(uid, "owner", "admin", post, env.h.WorkspaceFieldSecurityUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("simpan kebijakan gagal: %d", rec.Code)
	}

	// Permintaan berikutnya: Manager kini melihat nomor utuh.
	body = env.runAccount(uid, "owner", "manager", detailReq(), env.h.ContactDetail).Body.String()
	if !strings.Contains(body, mobile) || !strings.Contains(body, whatsapp) {
		t.Error("setelah override: Manager harus melihat HP & WhatsApp utuh")
	}
	// Telepon kantor tetap tampil (bukan sasaran F4).
	if !strings.Contains(body, office) {
		t.Error("telepon kantor tak boleh disamarkan")
	}
	// Sales TAK dicentang → tenant kini terkonfigurasi → Sales fail-closed di detail.
	sReq := contactsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID),
		nil, itoa(a.ID), itoa(c.ID))
	sBody := env.runAccount(uid, "owner", "sales", sReq, env.h.ContactDetail).Body.String()
	if strings.Contains(sBody, mobile) || strings.Contains(sBody, whatsapp) {
		t.Error("tenant terkonfigurasi: Sales tak dicentang harus fail-closed (mask), bukan default lama")
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
