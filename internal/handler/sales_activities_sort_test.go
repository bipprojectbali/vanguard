package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sales_activities_sort_test.go — BL-157j: daftar Sales Activities terurut ke-4
// kolomnya via ?sort=col&dir=asc|desc, klon arsitektur BL-157a..i (deals/leads/
// contacts/tickets dst _sort_test.go). Tak menguji ulang SEMUA kolom simetris —
// cukup representatif per pola cursor (NOT NULL teks: kind/subject; nullable
// via JOIN: owner; nullable raw enum: status) + fallback + F3 + pagination,
// karena mekanisme cursor generik (sortcursor.go) sudah teruji tuntas di 157a.

// seedSalesActivityFull = varian seedSalesActivity (sales_activities_test.go)
// dgn kontrol penuh atas status (nullable) — seedSalesActivity tak cukup
// fleksibel utk uji posisi NULL kolom Status. status "" ≡ NULL.
func (e *testEnv) seedSalesActivityFull(t *testing.T, kind, targetType string, targetID int64, subject string, owner *int64, status string) db.Activity {
	t.Helper()
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	a, err := e.q.CreateActivity(t.Context(), db.CreateActivityParams{
		TenantID:        e.tenantID,
		Kind:            kind,
		Subject:         subject,
		TargetType:      targetType,
		TargetID:        targetID,
		OwnerID:         owner,
		ActivityContext: ptr(activityContextSales),
		Status:          statusPtr,
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed activity %s: %v", subject, err)
	}
	return a
}

// Jenis (kind, NOT NULL, RAW enum alfabetis — bukan urutan tampil badge).
func TestActivitiesList_SortKindAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedSalesActivity(t, "task", "account", acc.ID, "Aktivitas-Task", &uid)
	env.seedSalesActivity(t, "call", "account", acc.ID, "Aktivitas-Call", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-Note", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=kind&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iCall := strings.Index(body, "Aktivitas-Call")
	iNote := strings.Index(body, "Aktivitas-Note")
	iTask := strings.Index(body, "Aktivitas-Task")
	if iCall < 0 || iNote < 0 || iTask < 0 {
		t.Fatalf("ketiga aktivitas harus tampil:\n%s", body)
	}
	if !(iCall < iNote && iNote < iTask) {
		t.Errorf("dir=asc harus urut call < note < task, dapat posisi %d/%d/%d", iCall, iNote, iTask)
	}
}

// Subjek (NOT NULL) asc & desc.
func TestActivitiesList_SortSubjectAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Subjek Zebra", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Subjek Awal", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Subjek Mekar", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=subject&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Subjek Awal")
	iMekar := strings.Index(body, "Subjek Mekar")
	iZebra := strings.Index(body, "Subjek Zebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestActivitiesList_SortSubjectDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Subjek Zebra", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Subjek Awal", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=subject&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Subjek Awal")
	iZebra := strings.Index(body, "Subjek Zebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestActivitiesList_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-Fallback", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=bogus", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Aktivitas-Fallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// Pemilik (owner_id nullable, kunci sort = ownerName/COALESCE(name,email),
// mirror Pemilik Deals).
func TestActivitiesList_SortOwnerAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	memA := env.seedMember(t, "act-owner-awal@x", "member", env.tenantID)
	memZ := env.seedMember(t, "act-owner-zebra@x", "member", env.tenantID)

	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-OwnerAwal", &memA.ID)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-OwnerZebra", &memZ.ID)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-OwnerNull", nil)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=owner&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Aktivitas-OwnerAwal")
	iZ := strings.Index(body, "Aktivitas-OwnerZebra")
	iN := strings.Index(body, "Aktivitas-OwnerNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut email-Awal < email-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// Status (nullable — call/note tanpa status, RAW enum alfabetis, bukan
// derivasi seperti Renewals; mirror keputusan Tickets).
func TestActivitiesList_SortStatusAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedSalesActivityFull(t, "task", "account", acc.ID, "Aktivitas-StatusCompleted", &uid, "Completed")
	env.seedSalesActivityFull(t, "task", "account", acc.ID, "Aktivitas-StatusInProgress", &uid, "In Progress")
	env.seedSalesActivityFull(t, "call", "account", acc.ID, "Aktivitas-StatusNull", &uid, "")

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=status&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iC := strings.Index(body, "Aktivitas-StatusCompleted")
	iP := strings.Index(body, "Aktivitas-StatusInProgress")
	iN := strings.Index(body, "Aktivitas-StatusNull")
	if iC < 0 || iP < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iC < iP && iP < iN) {
		t.Errorf("dir=asc harus urut Completed < In Progress < NULL(akhir), dapat posisi %d/%d/%d", iC, iP, iN)
	}
}

// F3 ownership tetap dihormati di jalur sort (representatif, predikat
// ownership TAK berubah antar kolom — cukup 1 test lintas kolom yang sudah ada).
func TestActivitiesList_SortSubjectRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-act-sort@x", "member", env.tenantID)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)

	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-MilikSaya", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-MilikOrang", &lain.ID)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=subject&dir=asc", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MilikSaya") {
		t.Errorf("sales harus melihat aktivitas MILIKNYA walau sort aktif")
	}
	if strings.Contains(body, "MilikOrang") {
		t.Errorf("sort tak boleh menembus F3: sales melihat aktivitas milik orang lain")
	}
}

// Pagination representatif utk cursor bertipe teks (subject) — membuktikan
// splitPageText + ?after= bekerja pada jalur sort, bukan cuma default.
func TestActivitiesList_SortSubjectPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	var lastSubject string
	for i := 0; i <= pageSize; i++ {
		subject := "Aktivitas-PageV" + padded(i)
		env.seedSalesActivity(t, "note", "account", acc.ID, subject, &uid)
		lastSubject = subject
	}

	first := accountsReq(http.MethodGet, "/w/test/activities?sort=subject&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "sales", first, env.h.ActivitiesList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastSubject) {
		t.Fatalf("subjek terakhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/activities")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/activities?sort=subject&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "sales", second, env.h.ActivitiesList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastSubject) {
		t.Errorf("subjek terakhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// Tanggal (created_at, NOT NULL) asc & desc — BEDA dari kind/subject/owner/
// status: kolom ini SUDAH sumbu default (DESC via jalur ListActivities biasa,
// tanpa ?sort=), jadi test ini secara khusus menempuh jalur
// ListActivitiesSortByDate (arah dinamis) via ?sort=date eksplisit, bukan
// default. cursor pageCursorTimestamp/splitPageTimestamp baru ditulis khusus
// utk kolom ini (bukan reuse pageCursor/splitPage yg sentinelnya arah tetap)
// — layak diuji tersendiri.
func TestActivitiesList_SortDateAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-Pertama", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-Kedua", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-Ketiga", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=date&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	i1 := strings.Index(body, "Aktivitas-Pertama")
	i2 := strings.Index(body, "Aktivitas-Kedua")
	i3 := strings.Index(body, "Aktivitas-Ketiga")
	if i1 < 0 || i2 < 0 || i3 < 0 {
		t.Fatalf("ketiga aktivitas harus tampil:\n%s", body)
	}
	if !(i1 < i2 && i2 < i3) {
		t.Errorf("dir=asc harus urut dibuat-lebih-dulu ke lebih-baru, dapat posisi %d/%d/%d", i1, i2, i3)
	}
}

func TestActivitiesList_SortDateDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-Pertama", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Aktivitas-Kedua", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activities?sort=date&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	i1 := strings.Index(body, "Aktivitas-Pertama")
	i2 := strings.Index(body, "Aktivitas-Kedua")
	if i1 < 0 || i2 < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(i2 < i1) {
		t.Errorf("dir=desc harus urut lebih-baru dulu: Kedua < Pertama, dapat posisi %d/%d", i2, i1)
	}
}

// Pagination representatif utk cursor timestamptz arah-dinamis (pageCursorTimestamp/
// splitPageTimestamp, ditulis khusus BL-157j lanjutan) — membuktikan ?after=
// bekerja pada jalur sort=date, bukan cuma jalur default created_at DESC tetap.
func TestActivitiesList_SortDatePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	var lastSubject string
	for i := 0; i <= pageSize; i++ {
		subject := "Aktivitas-TglV" + padded(i)
		env.seedSalesActivity(t, "note", "account", acc.ID, subject, &uid)
		lastSubject = subject
	}

	first := accountsReq(http.MethodGet, "/w/test/activities?sort=date&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "sales", first, env.h.ActivitiesList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastSubject) {
		t.Fatalf("aktivitas terakhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/activities")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/activities?sort=date&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "sales", second, env.h.ActivitiesList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastSubject) {
		t.Errorf("aktivitas terakhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}
