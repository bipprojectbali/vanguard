package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// health_score_sort_more_test.go — lanjutan health_score_sort_test.go
// (dipecah krn ambang File Health; lihat komentar file itu utk konteks BL-157i).

// ── Adopsi/Engagement/Support (smallint nullable) — representatif 1 arah,
// pola query identik dengan "score" (klon manual, risiko sama). ──

func TestHealthScore_SortAdoptionAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accLow := env.seedAccount(t, "Desa HARendah", &uid, nil, nil)
	accHigh := env.seedAccount(t, "Desa HATinggi", &uid, nil, nil)
	env.seedHealthFull(t, healthFullParams{AccountID: accLow.ID, Adoption: i16p(10)})
	env.makeCustomer(t, accLow.ID, "Active")
	env.seedHealthFull(t, healthFullParams{AccountID: accHigh.ID, Adoption: i16p(80)})
	env.makeCustomer(t, accHigh.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("adoption", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Desa HARendah")
	iH := strings.Index(body, "Desa HATinggi")
	if iL < 0 || iH < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iL < iH) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi, dapat posisi %d/%d", iL, iH)
	}
}

func TestHealthScore_SortEngagementAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accLow := env.seedAccount(t, "Desa HERendah", &uid, nil, nil)
	accHigh := env.seedAccount(t, "Desa HETinggi", &uid, nil, nil)
	env.seedHealthFull(t, healthFullParams{AccountID: accLow.ID, Engagement: i16p(15)})
	env.makeCustomer(t, accLow.ID, "Active")
	env.seedHealthFull(t, healthFullParams{AccountID: accHigh.ID, Engagement: i16p(75)})
	env.makeCustomer(t, accHigh.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("engagement", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Desa HERendah")
	iH := strings.Index(body, "Desa HETinggi")
	if iL < 0 || iH < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iL < iH) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi, dapat posisi %d/%d", iL, iH)
	}
}

func TestHealthScore_SortSupportDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	accLow := env.seedAccount(t, "Desa HSuRendah", &uid, nil, nil)
	accHigh := env.seedAccount(t, "Desa HSuTinggi", &uid, nil, nil)
	accNull := env.seedAccount(t, "Desa HSuNull", &uid, nil, nil)
	env.seedHealthFull(t, healthFullParams{AccountID: accLow.ID, Support: i16p(30)})
	env.makeCustomer(t, accLow.ID, "Active")
	env.seedHealthFull(t, healthFullParams{AccountID: accHigh.ID, Support: i16p(95)})
	env.makeCustomer(t, accHigh.ID, "Active")
	env.makeCustomer(t, accNull.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("support", "desc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Desa HSuRendah")
	iH := strings.Index(body, "Desa HSuTinggi")
	iN := strings.Index(body, "Desa HSuNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iH && iH < iL) {
		t.Errorf("dir=desc harus urut NULL(awal) < Tinggi < Rendah, dapat posisi %d/%d/%d", iN, iH, iL)
	}
}

// ── Tren (score_trend, text nullable) ──

func TestHealthScore_SortTrendAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	accD := env.seedAccount(t, "Desa HTDeclining", &uid, nil, nil)
	accI := env.seedAccount(t, "Desa HTImproving", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa HTNull", &uid, nil, nil)
	env.seedHealthFull(t, healthFullParams{AccountID: accD.ID, Trend: strp2("Declining")})
	env.makeCustomer(t, accD.ID, "Active")
	env.seedHealthFull(t, healthFullParams{AccountID: accI.ID, Trend: strp2("Improving")})
	env.makeCustomer(t, accI.ID, "Active")
	env.makeCustomer(t, accN.ID, "Active") // tanpa customer_success → trend NULL

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("trend", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iD := strings.Index(body, "Desa HTDeclining")
	iI := strings.Index(body, "Desa HTImproving")
	iN := strings.Index(body, "Desa HTNull")
	if iD < 0 || iI < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iD < iI && iI < iN) {
		t.Errorf("dir=asc harus urut Declining < Improving < NULL(akhir), dapat posisi %d/%d/%d", iD, iI, iN)
	}
}

func TestHealthScore_SortTrendDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	accD := env.seedAccount(t, "Desa HTDDeclining", &uid, nil, nil)
	accI := env.seedAccount(t, "Desa HTDImproving", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa HTDNull", &uid, nil, nil)
	env.seedHealthFull(t, healthFullParams{AccountID: accD.ID, Trend: strp2("Declining")})
	env.makeCustomer(t, accD.ID, "Active")
	env.seedHealthFull(t, healthFullParams{AccountID: accI.ID, Trend: strp2("Improving")})
	env.makeCustomer(t, accI.ID, "Active")
	env.makeCustomer(t, accN.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("trend", "desc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iD := strings.Index(body, "Desa HTDDeclining")
	iI := strings.Index(body, "Desa HTDImproving")
	iN := strings.Index(body, "Desa HTDNull")
	if iD < 0 || iI < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iI && iI < iD) {
		t.Errorf("dir=desc harus urut NULL(awal) < Improving < Declining, dapat posisi %d/%d/%d", iN, iI, iD)
	}
}

// ── Jatuh Tempo (sub.end_date via LATERAL, date nullable) ──

func TestHealthScore_SortRenewalAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	accE := env.seedAccount(t, "Desa HRAwal", &uid, nil, nil)
	accL := env.seedAccount(t, "Desa HRAkhir", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa HRNull", &uid, nil, nil)
	env.seedHealthSub(t, accE.ID, base.AddDate(0, 0, 10))
	env.seedHealthSub(t, accL.ID, base.AddDate(0, 6, 0))
	env.seedHealthSub(t, accN.ID, time.Time{}) // Active TANPA end_date → renewal NULL

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("renewal", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iE := strings.Index(body, "Desa HRAwal")
	iL := strings.Index(body, "Desa HRAkhir")
	iN := strings.Index(body, "Desa HRNull")
	if iE < 0 || iL < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iE < iL && iL < iN) {
		t.Errorf("dir=asc harus urut Awal < Akhir < NULL(akhir), dapat posisi %d/%d/%d", iE, iL, iN)
	}
}

func TestHealthScore_SortRenewalDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	accE := env.seedAccount(t, "Desa HRDAwal", &uid, nil, nil)
	accL := env.seedAccount(t, "Desa HRDAkhir", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa HRDNull", &uid, nil, nil)
	env.seedHealthSub(t, accE.ID, base.AddDate(0, 0, 10))
	env.seedHealthSub(t, accL.ID, base.AddDate(0, 6, 0))
	env.seedHealthSub(t, accN.ID, time.Time{})

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("renewal", "desc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iE := strings.Index(body, "Desa HRDAwal")
	iL := strings.Index(body, "Desa HRDAkhir")
	iN := strings.Index(body, "Desa HRDNull")
	if iE < 0 || iL < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iL && iL < iE) {
		t.Errorf("dir=desc harus urut NULL(awal) < Akhir < Awal, dapat posisi %d/%d/%d", iN, iL, iE)
	}
}

// Pagination representatif utk cursor bertipe DATE NULLABLE.
func TestHealthScore_SortRenewalPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa HRPag" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		env.seedHealthSub(t, acc.ID, base.AddDate(0, 0, i))
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", healthSortReq("renewal", "asc"), env.h.HealthScoreList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("tanggal terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/health-scores")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/health-scores?sort=renewal&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.HealthScoreList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("tanggal terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// ── ?sort= tak dikenal / tak sortable (Status) → fallback default ──

func TestHealthScore_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa HFallback", &uid, nil, nil)
	env.makeCustomer(t, acc.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("status", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Desa HFallback") {
		t.Errorf("?sort= tak sortable (mis. status) harus tetap render daftar (jatuh ke default)")
	}
}
