package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_leads_sort_test.go — BL-157b: daftar Leads terurut ke-7 kolomnya via
// ?sort=col&dir=asc|desc, klon arsitektur BL-157a (subscriptions_sort_test.go).
// Tak menguji ulang SEMUA kolom simetris — cukup representatif per pola cursor
// (NOT NULL teks: name/status; nullable teks: code; nullable numeric: value;
// nullable via JOIN: owner) + fallback + F3 + pagination, karena mekanisme
// cursor generik (sortcursor.go) sudah teruji tuntas di 157a.
//
// (Kontrak URL header/tab/pager sort di sisi VIEW diuji di
// panel/sales_leads_sort_test.go.)

// seedLeadFull = varian seed lead dgn kontrol penuh atas kolom sortable
// nullable (source/rating/estimatedValue/owner) — seedLeadStatus
// (reports_sales_panels_test.go) tak cukup fleksibel utk uji posisi NULL.
// estValue "" ≡ estimated_value NULL (pgtype.Numeric{} nol-nilai).
func (e *testEnv) seedLeadFull(t *testing.T, name string, owner *int64, source, status, rating, estValue string) db.Lead {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityLead)
	if err != nil {
		t.Fatalf("generate lead code: %v", err)
	}
	var estNum pgtype.Numeric
	if estValue != "" {
		estNum = numFrom(t, estValue)
	}
	var srcPtr, ratingPtr *string
	if source != "" {
		srcPtr = &source
	}
	if rating != "" {
		ratingPtr = &rating
	}
	l, err := e.q.CreateLead(t.Context(), db.CreateLeadParams{
		TenantID:       e.tenantID,
		EntityCode:     &code,
		LeadName:       name,
		LeadOwner:      owner,
		LeadSource:     srcPtr,
		LeadStatus:     status,
		Rating:         ratingPtr,
		EstimatedValue: estNum,
		CreatedBy:      owner,
	})
	if err != nil {
		t.Fatalf("seed lead %s: %v", name, err)
	}
	return l
}

// seedLeadNoCode = lead dgn entity_code NULL (GenerateEntityCode dilewati) —
// utk uji posisi NULL kolom "Kode".
func (e *testEnv) seedLeadNoCode(t *testing.T, name string, owner *int64) db.Lead {
	t.Helper()
	l, err := e.q.CreateLead(t.Context(), db.CreateLeadParams{
		TenantID:   e.tenantID,
		LeadName:   name,
		LeadOwner:  owner,
		LeadStatus: "New",
		CreatedBy:  owner,
	})
	if err != nil {
		t.Fatalf("seed lead no-code %s: %v", name, err)
	}
	return l
}

func TestLeadsList_SortNameAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadFull(t, "Lead Zebra", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead Awal", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead Mekar", &uid, "", "New", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=name&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Lead Awal")
	iMekar := strings.Index(body, "Lead Mekar")
	iZebra := strings.Index(body, "Lead Zebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga lead harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestLeadsList_SortNameDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadFull(t, "Lead Zebra", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead Awal", &uid, "", "New", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=name&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Lead Awal")
	iZebra := strings.Index(body, "Lead Zebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua lead harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestLeadsList_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadFull(t, "Lead Fallback", &uid, "", "New", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=bogus", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Lead Fallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// Status (NOT NULL, raw enum alfabetis — bukan urutan tingkat funnel).
func TestLeadsList_SortStatusAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadFull(t, "Lead StatusNew", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead StatusQualified", &uid, "", "Qualified", "", "")
	env.seedLeadFull(t, "Lead StatusUnqualified", &uid, "", "Unqualified", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=status&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iN := strings.Index(body, "Lead StatusNew")
	iQ := strings.Index(body, "Lead StatusQualified")
	iU := strings.Index(body, "Lead StatusUnqualified")
	if iN < 0 || iQ < 0 || iU < 0 {
		t.Fatalf("ketiga baris status harus tampil:\n%s", body)
	}
	if !(iN < iQ && iQ < iU) {
		t.Errorf("dir=asc harus urut New < Qualified < Unqualified, dapat posisi %d/%d/%d", iN, iQ, iU)
	}
}

// Kode (entity_code nullable). asc→NULL akhir, desc→NULL awal.
func TestLeadsList_SortCodeAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadFull(t, "Lead CodeAwal", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead CodeZebra", &uid, "", "New", "", "")
	env.seedLeadNoCode(t, "Lead CodeNull", &uid)

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=code&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Lead CodeAwal")
	iZ := strings.Index(body, "Lead CodeZebra")
	iN := strings.Index(body, "Lead CodeNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	// entity_code dialokasikan berurutan (LEAD-001, LEAD-002, ...) → Awal < Zebra.
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut kode-Awal < kode-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

func TestLeadsList_SortCodeDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadFull(t, "Lead CodeAwal", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead CodeZebra", &uid, "", "New", "", "")
	env.seedLeadNoCode(t, "Lead CodeNull", &uid)

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=code&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Lead CodeAwal")
	iZ := strings.Index(body, "Lead CodeZebra")
	iN := strings.Index(body, "Lead CodeNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iZ && iZ < iA) {
		t.Errorf("dir=desc harus urut NULL(awal) < kode-Zebra < kode-Awal, dapat posisi %d/%d/%d", iN, iZ, iA)
	}
}

// Estimasi (estimated_value nullable numeric) + pagination representatif utk
// cursor bertipe numeric (mirror MRR Subscriptions).
func TestLeadsList_SortValueAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadFull(t, "Lead ValueRendah", &uid, "", "New", "", "1000000")
	env.seedLeadFull(t, "Lead ValueTinggi", &uid, "", "New", "", "9000000")
	env.seedLeadFull(t, "Lead ValueNull", &uid, "", "New", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=value&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Lead ValueRendah")
	iH := strings.Index(body, "Lead ValueTinggi")
	iN := strings.Index(body, "Lead ValueNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iL < iH && iH < iN) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi < NULL(akhir), dapat posisi %d/%d/%d", iL, iH, iN)
	}
}

func TestLeadsList_SortValuePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Lead ValueV" + padded(i)
		val := itoa(int64(100000 + i*1000))
		env.seedLeadFull(t, name, &uid, "", "New", "", val)
		lastName = name
	}

	first := accountsReq(http.MethodGet, "/w/test/leads?sort=value&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "sales", first, env.h.LeadsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("Estimasi terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/leads")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/leads?sort=value&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "sales", second, env.h.LeadsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("Estimasi terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// Pemilik (lead_owner nullable, kunci sort = ownerName/COALESCE(name,email),
// mirror CSM Subscriptions).
func TestLeadsList_SortOwnerAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	memA := env.seedMember(t, "lead-owner-awal@x", "member", env.tenantID)
	memZ := env.seedMember(t, "lead-owner-zebra@x", "member", env.tenantID)

	env.seedLeadFull(t, "Lead OwnerAwal", &memA.ID, "", "New", "", "")
	env.seedLeadFull(t, "Lead OwnerZebra", &memZ.ID, "", "New", "", "")
	env.seedLeadFull(t, "Lead OwnerNull", nil, "", "New", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=owner&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Lead OwnerAwal")
	iZ := strings.Index(body, "Lead OwnerZebra")
	iN := strings.Index(body, "Lead OwnerNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut email-Awal < email-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// F3 ownership tetap dihormati di jalur sort (representatif, predikat ownership
// TAK berubah antar kolom — cukup 1 test lintas kolom yang sudah ada).
func TestLeadsList_SortNameRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-lead-sort@x", "member", env.tenantID)

	env.seedLeadFull(t, "Lead MilikSaya", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead MilikOrang", &lain.ID, "", "New", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=name&dir=asc", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MilikSaya") {
		t.Errorf("sales harus melihat lead MILIKNYA walau sort aktif")
	}
	if strings.Contains(body, "MilikOrang") {
		t.Errorf("sort tak boleh menembus F3: sales melihat lead milik orang lain")
	}
}

// Kombinasi ?sort=name&tab=my tetap menghormati mine_only (tab), sama seperti
// F3 di atas tapi utk sumbu filter ORTOGONAL (leadTab), bukan ownership scope.
func TestLeadsList_SortNameWithMineOnlyTab(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-lead-tab@x", "member", env.tenantID)

	env.seedLeadFull(t, "Lead TabMilikSaya", &uid, "", "New", "", "")
	env.seedLeadFull(t, "Lead TabMilikOrang", &lain.ID, "", "New", "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?sort=name&dir=asc&tab=my", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "TabMilikSaya") {
		t.Errorf("tab=my harus tetap menampilkan lead milik aktor walau sort aktif")
	}
	if strings.Contains(body, "TabMilikOrang") {
		t.Errorf("tab=my tak boleh menampilkan lead milik orang lain walau owner (scope_all)")
	}
}
