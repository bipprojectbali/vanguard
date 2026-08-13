package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// contacts_ui_test.go — invarian yang lahir dari penyelarasan UI ke wireframe M3:
//
//   - DAFTAR GLOBAL: nomor WhatsApp ikut di baris tapi TETAP tunduk F4 (Sales utuh,
//     non-Sales tersamar) — masking berlaku di daftar, bukan hanya detail.
//   - Kolom "Desa" hanya di daftar global (village_name dari JOIN accounts).
//   - Tab Semua/Kontak Saya HANYA muncul untuk peran ber-cakupan penuh (ScopeAll);
//     Sales/CSM (ScopeOwn) tak melihatnya (Semua≡Saya). ?view=my mempersempit ke
//     desa milik aktor walau perannya ScopeAll.
//   - DETAIL: Owner/Atasan/Dibuat-Diubah tampil sebagai NAMA (bukan id); tanggal
//     audit terformat; kolom aktivitas yang ditunda modul Activities dirender "—".

// seedContactReports = seedContact yang juga menyetel reports_to_id (atasan) —
// untuk membuktikan detail meresolusi nama atasan, bukan menampilkan id.
func (e *testEnv) seedContactReports(t *testing.T, accountID int64, firstName string, owner, reportsTo *int64) db.Contact {
	t.Helper()
	c, err := e.q.CreateContact(t.Context(), db.CreateContactParams{
		TenantID:     e.tenantID,
		AccountID:    accountID,
		ContactOwner: owner,
		ReportsToID:  reportsTo,
		FirstName:    firstName,
		CreatedBy:    owner,
	})
	if err != nil {
		t.Fatalf("seed contact %s: %v", firstName, err)
	}
	return c
}

// --- Daftar global: masking WhatsApp + kolom Desa -------------------------

// TestContacts_ListWhatsappMasking: di DAFTAR global, nomor WhatsApp utuh HANYA
// bagi Sales; Admin (non-Sales) menerima penanda tersamar — nomor asli tak pernah
// sampai ke browser. Membuktikan F4 menjaga daftar, bukan cuma detail.
func TestContacts_ListWhatsappMasking(t *testing.T) {
	env, uid := setupAccounts(t)
	whatsapp := "0813-2222-3333"
	a := env.seedAccount(t, "Desa WA", &uid, nil, nil)
	env.seedContactPhone(t, a.ID, "Sari", "0812-0000-0000", whatsapp, "021-555-0000")

	req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
	salesBody := env.runAccount(uid, "member", "sales", req, env.h.ContactsAll).Body.String()
	if !strings.Contains(salesBody, whatsapp) {
		t.Error("Sales harus melihat nomor WhatsApp utuh di daftar")
	}

	req2 := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
	adminBody := env.runAccount(uid, "owner", "admin", req2, env.h.ContactsAll).Body.String()
	if strings.Contains(adminBody, whatsapp) {
		t.Error("Admin non-Sales BOCOR — nomor WhatsApp asli sampai ke daftar")
	}
	if !strings.Contains(adminBody, flsHidden) {
		t.Error("Admin harus menerima penanda tersamar (flsHidden) di kolom WhatsApp")
	}
}

// TestContacts_GlobalListShowsVillage: daftar global menampilkan kolom Desa —
// nama desa induk (village_name) tampil di baris kontak lintas-desa.
func TestContacts_GlobalListShowsVillage(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	env.seedContact(t, a.ID, "Andi", &uid, false)

	req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
	body := env.runAccount(uid, "owner", "admin", req, env.h.ContactsAll).Body.String()
	if !strings.Contains(body, "Desa Sukamaju") {
		t.Error("daftar global harus menampilkan nama desa induk (kolom Desa)")
	}
}

// --- Tab cakupan (Semua / Kontak Saya) ------------------------------------

// TestContacts_TabsVisibilityByScope: bilah tab Semua/Kontak Saya hanya untuk
// peran ScopeAll (admin/manager). Sales (ScopeOwn) tak melihatnya — baginya
// Semua≡Saya, tab redundan.
func TestContacts_TabsVisibilityByScope(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Tab", &uid, nil, nil)
	env.seedContact(t, a.ID, "Kontak Tab", &uid, false)

	cases := []struct {
		role     string
		showTabs bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", false},
		{"csm", false},
	}
	for _, tc := range cases {
		t.Run("role="+tc.role, func(t *testing.T) {
			req := contactsReq(http.MethodGet, "/w/test/contacts", nil, "", "")
			body := env.runAccount(uid, "owner", tc.role, req, env.h.ContactsAll).Body.String()
			has := strings.Contains(body, "Semua Kontak") && strings.Contains(body, "Kontak Saya")
			if has != tc.showTabs {
				t.Errorf("role %q showTabs=%v, tapi tab %v di body", tc.role, tc.showTabs, has)
			}
		})
	}
}

// TestContacts_TabMyFiltersToOwned: admin (ScopeAll) dengan ?view=my dipersempit
// ke desa MILIKNYA — kontak desa milik anggota lain tak tampil, walau tanpa filter
// admin melihat semuanya.
func TestContacts_TabMyFiltersToOwned(t *testing.T) {
	env, admin := setupAccounts(t)
	other := env.seedMember(t, "lain@local", "member", 0).ID
	mine := env.seedAccount(t, "Desa Admin", &admin, nil, nil)
	theirs := env.seedAccount(t, "Desa Lain", &other, nil, nil)
	env.seedContact(t, mine.ID, "KontakMilikku", &admin, false)
	env.seedContact(t, theirs.ID, "KontakOrangLain", &other, false)

	// Tanpa tab (Semua): admin melihat keduanya.
	all := env.runAccount(admin, "owner", "admin",
		contactsReq(http.MethodGet, "/w/test/contacts", nil, "", ""), env.h.ContactsAll).Body.String()
	if !strings.Contains(all, "KontakMilikku") || !strings.Contains(all, "KontakOrangLain") {
		t.Fatal("tanpa filter, admin harus melihat semua kontak")
	}

	// ?view=my: hanya desa milik admin.
	my := env.runAccount(admin, "owner", "admin",
		contactsReq(http.MethodGet, "/w/test/contacts?view=my", nil, "", ""), env.h.ContactsAll).Body.String()
	if !strings.Contains(my, "KontakMilikku") {
		t.Error("view=my harus tetap menampilkan kontak desa milik admin")
	}
	if strings.Contains(my, "KontakOrangLain") {
		t.Error("view=my harus menyembunyikan kontak desa milik anggota lain")
	}
}

// --- Detail: nama Owner/Atasan/Audit + kolom ditunda ----------------------

// TestContacts_DetailNamesAndAudit: detail meresolvasi id → NAMA untuk Pemilik,
// Atasan, dan Dibuat/Diubah Oleh; tanggal audit terformat; kolom aktivitas yang
// ditunda modul Activities dirender "—".
func TestContacts_DetailNamesAndAudit(t *testing.T) {
	env, admin := setupAccounts(t)
	owner := env.seedMember(t, "pemilik@local", "member", 0).ID
	a := env.seedAccount(t, "Desa Detail", &owner, nil, nil)
	sup := env.seedContact(t, a.ID, "Pak Atasan", &owner, false)
	c := env.seedContactReports(t, a.ID, "Bawahan", &owner, &sup.ID)

	req := contactsReq(http.MethodGet,
		"/w/test/accounts/"+itoa(a.ID)+"/contacts/"+itoa(c.ID), nil, itoa(a.ID), itoa(c.ID))
	body := env.runAccount(admin, "owner", "admin", req, env.h.ContactDetail).Body.String()

	// Pemilik & Dibuat-Oleh (keduanya = owner) → email pemilik (nama jatuh ke email).
	if !strings.Contains(body, "pemilik@local") {
		t.Error("detail harus menampilkan NAMA pemilik/pembuat, bukan id")
	}
	// Atasan → nama kontak atasan (reports_to_id resolusi).
	if !strings.Contains(body, "Pak Atasan") {
		t.Error("detail harus meresolusi nama atasan (reports_to)")
	}
	// Tanggal audit terformat — bandingkan dengan format handler atas nilai tersimpan.
	got, err := env.q.GetContact(t.Context(), c.ID)
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if want := fmtDateTime(got.CreatedAt); !strings.Contains(body, want) {
		t.Errorf("detail harus menampilkan tanggal dibuat terformat %q", want)
	}
	// Kolom yang ditunda modul Activities + label audit hadir (label tanpa "&"
	// agar tak tergantung escaping HTML "&amp;" pada judul kartu).
	for _, label := range []string{"Ringkasan Keterlibatan", "Terakhir Dihubungi", "Aktivitas Terakhir", "Dibuat Oleh", "Terakhir Diubah"} {
		if !strings.Contains(body, label) {
			t.Errorf("detail harus memuat kartu/kolom %q", label)
		}
	}
}
