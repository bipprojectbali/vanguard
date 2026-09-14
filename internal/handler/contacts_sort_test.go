package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// contacts_sort_test.go — BL-157d: daftar Kontak GLOBAL (/w/{workspace}/contacts)
// terurut ke-4 kolom sortable-nya via ?sort=col&dir=asc|desc, klon arsitektur
// BL-157a/b/c (subscriptions_sort_test.go / sales_leads_sort_test.go /
// accounts_sort_test.go). Tak menguji ulang SEMUA kolom simetris — representatif
// per pola cursor (nullable direct-column: code, role; NOT NULL computed
// expression: name; NOT NULL via JOIN: village) + fallback + F3 + pagination,
// karena mekanisme cursor generik (sortcursor.go) sudah teruji tuntas di
// 157a/b/c.
//
// (Kontrak URL header/pager sort di sisi VIEW diuji di
// panel/contacts_sort_test.go.)

// seedContactFull = varian seed kontak dengan kontrol penuh atas entity_code &
// contact_role (untuk uji sort Kode/Peran) — seedContact (contacts_test.go)
// tak cukup fleksibel karena keduanya selalu nil. "" = nil (kelompok NULL).
func (e *testEnv) seedContactFull(t *testing.T, accountID int64, firstName, entityCode, contactRole string) db.Contact {
	t.Helper()
	var codePtr, rolePtr *string
	if entityCode != "" {
		codePtr = &entityCode
	}
	if contactRole != "" {
		rolePtr = &contactRole
	}
	c, err := e.q.CreateContact(t.Context(), db.CreateContactParams{
		TenantID:    e.tenantID,
		AccountID:   accountID,
		FirstName:   firstName,
		EntityCode:  codePtr,
		ContactRole: rolePtr,
	})
	if err != nil {
		t.Fatalf("seed contact full %s: %v", firstName, err)
	}
	return c
}

// generatedContactCode = kode kontak REAL via GenerateEntityCode (KON-xxx),
// dipakai saat urutan kodenya (bukan sekadar keberadaannya) yang diuji.
func (e *testEnv) generatedContactCode(t *testing.T) string {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityContact)
	if err != nil {
		t.Fatalf("generate contact code: %v", err)
	}
	return code
}

func TestContactsAll_SortCodeAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Kode", &uid, nil, nil)
	codeA := "KON-001"
	codeZ := "KON-999"
	env.seedContactFull(t, a.ID, "KontakZ", codeZ, "")
	env.seedContactFull(t, a.ID, "KontakA", codeA, "")
	env.seedContactFull(t, a.ID, "KontakNull", "", "")

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=code&dir=asc", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, codeA)
	iZ := strings.Index(body, codeZ)
	iN := strings.Index(body, "KontakNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut %s < %s < NULL(akhir), dapat posisi %d/%d/%d", codeA, codeZ, iA, iZ, iN)
	}
}

func TestContactsAll_SortCodeDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Kode", &uid, nil, nil)
	codeA := "KON-001"
	codeZ := "KON-999"
	env.seedContactFull(t, a.ID, "KontakZ", codeZ, "")
	env.seedContactFull(t, a.ID, "KontakA", codeA, "")
	env.seedContactFull(t, a.ID, "KontakNull", "", "")

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=code&dir=desc", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, codeA)
	iZ := strings.Index(body, codeZ)
	iN := strings.Index(body, "KontakNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iZ && iZ < iA) {
		t.Errorf("dir=desc harus urut NULL(awal) < %s < %s, dapat posisi %d/%d/%d", codeZ, codeA, iN, iZ, iA)
	}
}

// Nama diurut ekspresi PERSIS yang tampil (first_name; last_name nil di sini →
// tak ganggu urutan lintas first_name berbeda) — SELALU NOT NULL.
func TestContactsAll_SortNameAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Nama", &uid, nil, nil)
	env.seedContact(t, a.ID, "Zebra", &uid, false)
	env.seedContact(t, a.ID, "Awal", &uid, false)
	env.seedContact(t, a.ID, "Mekar", &uid, false)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=name&dir=asc", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Awal")
	iMekar := strings.Index(body, "Mekar")
	iZebra := strings.Index(body, "Zebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga kontak harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestContactsAll_SortNameDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Nama", &uid, nil, nil)
	env.seedContact(t, a.ID, "Zebra", &uid, false)
	env.seedContact(t, a.ID, "Awal", &uid, false)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=name&dir=desc", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Awal")
	iZebra := strings.Index(body, "Zebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua kontak harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// Peran (contact_role nullable, RAW alfabetis — sama keputusan "Status"
// Leads/"Tipe" Accounts: tak menduplikasi urutan tampil ke SQL).
func TestContactsAll_SortRoleAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Peran", &uid, nil, nil)
	env.seedContactFull(t, a.ID, "KontakDecisionMaker", "", "Decision Maker")
	env.seedContactFull(t, a.ID, "KontakInfluencer", "", "Influencer")
	env.seedContactFull(t, a.ID, "KontakNull", "", "")

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=role&dir=asc", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iDM := strings.Index(body, "KontakDecisionMaker")
	iInf := strings.Index(body, "KontakInfluencer")
	iN := strings.Index(body, "KontakNull")
	if iDM < 0 || iInf < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iDM < iInf && iInf < iN) {
		t.Errorf("dir=asc harus urut Decision Maker < Influencer < NULL(akhir), dapat posisi %d/%d/%d", iDM, iInf, iN)
	}
}

// Desa (village_name via JOIN accounts, TIDAK NULLABLE) — representatif
// membuktikan JOIN lintas tabel bekerja; pola NULL sudah tuntas diuji di Kode/Peran.
func TestContactsAll_SortVillageAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	aZ := env.seedAccount(t, "Desa Zebra", &uid, nil, nil)
	aA := env.seedAccount(t, "Desa Awal", &uid, nil, nil)
	env.seedContact(t, aZ.ID, "KontakDiZebra", &uid, false)
	env.seedContact(t, aA.ID, "KontakDiAwal", &uid, false)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=village&dir=asc", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "KontakDiAwal")
	iZebra := strings.Index(body, "KontakDiZebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua kontak harus tampil:\n%s", body)
	}
	if !(iAwal < iZebra) {
		t.Errorf("dir=asc harus urut desa-Awal < desa-Zebra, dapat posisi %d/%d", iAwal, iZebra)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestContactsAll_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Fallback", &uid, nil, nil)
	env.seedContact(t, a.ID, "KontakFallback", &uid, false)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=bogus", nil)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "KontakFallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// F3 ownership (diwarisi desa induk) tetap dihormati di jalur sort
// (representatif, predikat ownership TAK berubah antar kolom).
func TestContactsAll_SortNameRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-kontak-sort@x", "member", env.tenantID)

	aMine := env.seedAccount(t, "Desa Saya", &uid, nil, nil)
	aTheirs := env.seedAccount(t, "Desa Orang", &lain.ID, nil, nil)
	env.seedContact(t, aMine.ID, "KontakMilikSaya", &uid, false)
	env.seedContact(t, aTheirs.ID, "KontakMilikOrang", &lain.ID, false)

	req := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=name&dir=asc", nil)
	rec := env.runAccount(uid, "member", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MilikSaya") {
		t.Errorf("sales harus melihat kontak desa MILIKNYA walau sort aktif")
	}
	if strings.Contains(body, "MilikOrang") {
		t.Errorf("sort tak boleh menembus F3: sales melihat kontak desa milik orang lain")
	}
}

func TestContactsAll_SortNamePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Banyak Kontak", &uid, nil, nil)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Kontak" + padded(i)
		env.seedContact(t, a.ID, name, &uid, false)
		lastName = name
	}

	first := contactsGlobalReq(http.MethodGet, "/w/test/contacts?sort=name&dir=asc", nil)
	rec1 := env.runAccount(uid, "owner", "sales", first, env.h.ContactsAll)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("kontak urutan akhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/contacts")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := contactsGlobalReq(http.MethodGet,
		"/w/test/contacts?sort=name&dir=asc&after="+after, nil)
	rec2 := env.runAccount(uid, "owner", "sales", second, env.h.ContactsAll)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("kontak urutan akhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}
