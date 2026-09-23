package handler

import (
	"net/http"
	"strings"
	"testing"
)

// sales_deals_sort_more_test.go — lanjutan sales_deals_sort_test.go
// (dipecah krn ambang File Health; lihat komentar file itu utk konteks BL-157e).

// Nilai (amount nullable numeric, disamarkan F4 hanya di TAMPILAN — cursor
// pakai nilai mentah) + pagination representatif utk cursor bertipe numeric.
func TestDealsList_SortAmountAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal AmountRendah", &uid, dealInitialStage, "1000000", "", "")
	env.seedDealFull(t, acc.ID, "Deal AmountTinggi", &uid, dealInitialStage, "9000000", "", "")
	env.seedDealFull(t, acc.ID, "Deal AmountNull", &uid, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=amount&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Deal AmountRendah")
	iH := strings.Index(body, "Deal AmountTinggi")
	iN := strings.Index(body, "Deal AmountNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iL < iH && iH < iN) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi < NULL(akhir), dapat posisi %d/%d/%d", iL, iH, iN)
	}
}

func TestDealsList_SortAmountPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Deal AmountV" + padded(i)
		val := itoa(int64(100000 + i*1000))
		env.seedDealFull(t, acc.ID, name, &uid, dealInitialStage, val, "", "")
		lastName = name
	}

	first := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=amount&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "sales", first, env.h.DealsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("Nilai terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/deals")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/deals?view=table&sort=amount&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "sales", second, env.h.DealsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("Nilai terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// Peluang (probability nullable smallint).
func TestDealsList_SortProbabilityAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal ProbRendah", &uid, dealInitialStage, "", "10", "")
	env.seedDealFull(t, acc.ID, "Deal ProbTinggi", &uid, dealInitialStage, "", "90", "")
	env.seedDealFull(t, acc.ID, "Deal ProbNull", &uid, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=probability&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Deal ProbRendah")
	iH := strings.Index(body, "Deal ProbTinggi")
	iN := strings.Index(body, "Deal ProbNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iL < iH && iH < iN) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi < NULL(akhir), dapat posisi %d/%d/%d", iL, iH, iN)
	}
}

// Perkiraan Tutup (expected_close_date nullable date, mirror Renewal Date
// Subscriptions).
func TestDealsList_SortCloseDateAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal CloseAwal", &uid, dealInitialStage, "", "", "2026-01-15")
	env.seedDealFull(t, acc.ID, "Deal CloseAkhir", &uid, dealInitialStage, "", "", "2026-12-01")
	env.seedDealFull(t, acc.ID, "Deal CloseNull", &uid, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=close&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Deal CloseAwal")
	iZ := strings.Index(body, "Deal CloseAkhir")
	iN := strings.Index(body, "Deal CloseNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut Awal < Akhir < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// Pemilik (deal_owner nullable, kunci sort = ownerName/COALESCE(name,email),
// mirror CSM Subscriptions & Pemilik Leads).
func TestDealsList_SortOwnerAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	memA := env.seedMember(t, "deal-owner-awal@x", "member", env.tenantID)
	memZ := env.seedMember(t, "deal-owner-zebra@x", "member", env.tenantID)

	env.seedDealFull(t, acc.ID, "Deal OwnerAwal", &memA.ID, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal OwnerZebra", &memZ.ID, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal OwnerNull", nil, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=owner&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Deal OwnerAwal")
	iZ := strings.Index(body, "Deal OwnerZebra")
	iN := strings.Index(body, "Deal OwnerNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut email-Awal < email-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// F3 ownership tetap dihormati di jalur sort (representatif, predikat
// ownership TAK berubah antar kolom — cukup 1 test lintas kolom yang sudah ada).
func TestDealsList_SortNameRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-deal-sort@x", "member", env.tenantID)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)

	env.seedDealFull(t, acc.ID, "Deal MilikSaya", &uid, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal MilikOrang", &lain.ID, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=name&dir=asc", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MilikSaya") {
		t.Errorf("sales harus melihat deal MILIKNYA walau sort aktif")
	}
	if strings.Contains(body, "MilikOrang") {
		t.Errorf("sort tak boleh menembus F3: sales melihat deal milik orang lain")
	}
}

// Kombinasi ?sort=name&mine=1 tetap menghormati mine_only (toggle Deal Saya),
// sama seperti F3 di atas tapi utk sumbu filter ORTOGONAL (BL-10), bukan
// ownership scope.
func TestDealsList_SortNameWithMineOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-deal-mine@x", "member", env.tenantID)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)

	env.seedDealFull(t, acc.ID, "Deal MineMilikSaya", &uid, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal MineMilikOrang", &lain.ID, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=name&dir=asc&mine=1", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MineMilikSaya") {
		t.Errorf("mine=1 harus tetap menampilkan deal milik aktor walau sort aktif")
	}
	if strings.Contains(body, "MineMilikOrang") {
		t.Errorf("mine=1 tak boleh menampilkan deal milik orang lain walau owner (scope_all)")
	}
}
