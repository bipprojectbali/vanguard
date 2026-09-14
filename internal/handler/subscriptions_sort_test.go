package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_sort_test.go — BL-157a (fondasi sort per kolom): daftar
// Subscriptions terurut "Desa" (village_name) via ?sort=village&dir=asc|desc.
// Empat sifat WAJIB benar bersama:
//   - urutan asc/desc benar-benar terbalik satu sama lain;
//   - ?sort= tak dikenal tak error, jatuh ke default (created_at DESC);
//   - keyset (?after=) cursor TEKS menjangkau baris yang "hilang" di halaman
//     pertama, sama seperti cursor timestamp (crm_master_pagination_test.go);
//   - kombinasi dengan status/F3 tetap menghormati filter yang sama (WHERE
//     tak diduplikasi salah antara ListSubscriptions & ListSubscriptionsSortByVillage).
//
// (Kontrak URL header/tab/pager sort di sisi VIEW diuji di
// panel/subscriptions_sort_test.go.)

func TestSubscriptionsList_SortVillageAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accZ := env.seedAccount(t, "Desa Zebra", &uid, nil, nil)
	planZ := env.seedPlanRow(t, "Paket Z", "PKT-Z", "Core")
	env.seedSubscription(t, accZ.ID, planZ.ID, &uid, "Active", "100000", "1200000")

	accA := env.seedAccount(t, "Desa Awal", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket A", "PKT-A", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "200000", "2400000")

	accM := env.seedAccount(t, "Desa Mekar", &uid, nil, nil)
	planM := env.seedPlanRow(t, "Paket M", "PKT-M", "Core")
	env.seedSubscription(t, accM.ID, planM.ID, &uid, "Active", "300000", "3600000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=village&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Desa Awal")
	iMekar := strings.Index(body, "Desa Mekar")
	iZebra := strings.Index(body, "Desa Zebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga desa harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestSubscriptionsList_SortVillageDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	accZ := env.seedAccount(t, "Desa Zebra", &uid, nil, nil)
	planZ := env.seedPlanRow(t, "Paket Z", "PKT-Z", "Core")
	env.seedSubscription(t, accZ.ID, planZ.ID, &uid, "Active", "100000", "1200000")

	accA := env.seedAccount(t, "Desa Awal", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket A", "PKT-A", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "200000", "2400000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=village&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Desa Awal")
	iZebra := strings.Index(body, "Desa Zebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua desa harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestSubscriptionsList_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Fallback", &uid, nil, nil)
	plan := env.seedPlanRow(t, "Paket F", "PKT-F", "Core")
	env.seedSubscription(t, acc.ID, plan.ID, &uid, "Active", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=mrr", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Desa Fallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// Baris "tertua" secara alfabet (village_name) hanya terjangkau lewat halaman
// kedua saat sort aktif — cermin TestCRMMaster_BarisTertuaTerjangkauLewatHalamanKedua
// tapi untuk cursor TEKS (hex-encoded), bukan timestamp.
func TestSubscriptionsList_SortVillagePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa V" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		plan := env.seedPlanRow(t, "Paket "+padded(i), "PKT-"+padded(i), "Core")
		env.seedSubscription(t, acc.ID, plan.ID, &uid, "Active", "100000", "1200000")
		lastName = name
	}

	first := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=village&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "admin", first, env.h.SubscriptionsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("desa terakhir alfabet seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/subscriptions")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/subscriptions?sort=village&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "admin", second, env.h.SubscriptionsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("desa terakhir alfabet harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// Kombinasi ?sort=village&status= tetap menghormati F3 (ownership): sort tak
// menembus cakupan kepemilikan, hanya mengurut ulang DI DALAM cakupan yang sama.
func TestSubscriptionsList_SortVillageRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-sort@x", "member", env.tenantID)

	mine := env.seedAccount(t, "Desa MilikSaya", &uid, nil, nil)
	planMine := env.seedPlanRow(t, "Paket Mine", "PKT-MINESORT", "Core")
	env.seedSubscription(t, mine.ID, planMine.ID, &uid, "Active", "100000", "1200000")

	theirs := env.seedAccount(t, "Desa MilikOrang", &lain.ID, nil, nil)
	planTheirs := env.seedPlanRow(t, "Paket Theirs", "PKT-THEIRSSORT", "Add-on")
	env.seedSubscription(t, theirs.ID, planTheirs.ID, &lain.ID, "Active", "300000", "3600000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=village&dir=asc", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MilikSaya") {
		t.Errorf("sales harus melihat langganan MILIKNYA walau sort aktif")
	}
	if strings.Contains(body, "MilikOrang") {
		t.Errorf("sort tak boleh menembus F3: sales melihat langganan milik orang lain")
	}
}

// sortAfter memungut cursor dari tautan "Berikutnya" untuk path daftar ini,
// dibaca dari HTML. Berbeda dari nextAfter (crm_master_pagination_test.go):
// path di sini SELALU didahului param lain (?status=Active&...&after=), jadi
// dicari via "after=" polos, bukan "path?after=".
func sortAfter(html, path string) string {
	marker := "after="
	i := strings.Index(html, path+"?")
	if i < 0 {
		return ""
	}
	rest := html[i:]
	j := strings.Index(rest, marker)
	if j < 0 {
		return ""
	}
	rest = rest[j+len(marker):]
	end := strings.IndexAny(rest, `&"'`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func padded(i int) string {
	s := "000" + itoa(int64(i))
	return s[len(s)-3:]
}

// ── Kolom lanjutan BL-157a (Paket/MRR/Masa Berlaku/Renewal Date/CSM) ──

// seedSubscriptionNoPlan = varian seedSubscription dgn plan_id NULL (BL-88 PR2b:
// nullable) — untuk uji posisi NULL kolom "Paket" (plan_name via LEFT JOIN plans).
func (e *testEnv) seedSubscriptionNoPlan(t *testing.T, accountID int64, owner *int64, status, mrr, arr string) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            nil,
		Status:            status,
		AutoRenew:         false,
		Mrr:               numFrom(t, mrr),
		Arr:               numFrom(t, arr),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed subscription no-plan: %v", err)
	}
	lineNo := int16(1)
	if _, err := e.q.AddSubscriptionItem(t.Context(), db.AddSubscriptionItemParams{
		SubscriptionID: s.ID, TenantID: e.tenantID, PlanID: nil, Quantity: 1,
		UnitPrice: numFrom(t, mrr), Subtotal: numFrom(t, mrr), Mrr: numFrom(t, mrr),
		Arr: numFrom(t, arr), LineNo: &lineNo,
	}); err != nil {
		t.Fatalf("seed subscription item no-plan: %v", err)
	}
	return s
}

// seedSubscriptionNullMrr = varian seedSubscription dgn s.mrr NULL di level
// subscriptions (item tetap perlu unit_price/subtotal NOT NULL — dipisah nilai
// dummy "0", tak terkait pengujian: sort mrr membaca s.mrr, bukan item).
func (e *testEnv) seedSubscriptionNullMrr(t *testing.T, accountID, planID int64, owner *int64, status string) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            &planID,
		Status:            status,
		AutoRenew:         false,
		Mrr:               pgtype.Numeric{},
		Arr:               pgtype.Numeric{},
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed subscription null mrr: %v", err)
	}
	lineNo := int16(1)
	if _, err := e.q.AddSubscriptionItem(t.Context(), db.AddSubscriptionItemParams{
		SubscriptionID: s.ID, TenantID: e.tenantID, PlanID: &planID, Quantity: 1,
		UnitPrice: numFrom(t, "0"), Subtotal: numFrom(t, "0"), LineNo: &lineNo,
	}); err != nil {
		t.Fatalf("seed subscription item null mrr: %v", err)
	}
	return s
}

// seedSubscriptionEndDate = varian seedSubscription dgn end_date TERISI —
// seedSubscription biasa TAK PERNAH mengisi end_date (nol-nilai
// CreateSubscriptionParams = NULL), jadi baris "renewal non-null" perlu jalur
// sendiri; baris "renewal NULL" cukup pakai seedSubscription apa adanya.
func (e *testEnv) seedSubscriptionEndDate(t *testing.T, accountID, planID int64, owner *int64, status, mrr, arr string, end time.Time) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            &planID,
		Status:            status,
		EndDate:           pgtype.Date{Time: end, Valid: true},
		AutoRenew:         false,
		Mrr:               numFrom(t, mrr),
		Arr:               numFrom(t, arr),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed subscription end-date: %v", err)
	}
	lineNo := int16(1)
	if _, err := e.q.AddSubscriptionItem(t.Context(), db.AddSubscriptionItemParams{
		SubscriptionID: s.ID, TenantID: e.tenantID, PlanID: &planID, Quantity: 1,
		UnitPrice: numFrom(t, mrr), Subtotal: numFrom(t, mrr), Mrr: numFrom(t, mrr),
		Arr: numFrom(t, arr), LineNo: &lineNo,
	}); err != nil {
		t.Fatalf("seed subscription item end-date: %v", err)
	}
	return s
}

// Masa Berlaku (raw status, keputusan user — BUKAN band urgensi derivasi).
// status=all agar ketiga status berbeda tampil bersamaan (default tab = Active).
func TestSubscriptionsList_SortStatusAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa StatusActive", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket StA", "PKT-STA", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "100000", "1200000")

	accS := env.seedAccount(t, "Desa StatusSuspended", &uid, nil, nil)
	planS := env.seedPlanRow(t, "Paket StS", "PKT-STS", "Core")
	env.seedSubscription(t, accS.ID, planS.ID, &uid, "Suspended", "100000", "1200000")

	accT := env.seedAccount(t, "Desa StatusTrial", &uid, nil, nil)
	planT := env.seedPlanRow(t, "Paket StT", "PKT-STT", "Core")
	env.seedSubscription(t, accT.ID, planT.ID, &uid, "Trial", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=status&dir=asc&status=all", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa StatusActive")
	iS := strings.Index(body, "Desa StatusSuspended")
	iT := strings.Index(body, "Desa StatusTrial")
	if iA < 0 || iS < 0 || iT < 0 {
		t.Fatalf("ketiga baris status harus tampil:\n%s", body)
	}
	if !(iA < iS && iS < iT) {
		t.Errorf("dir=asc harus urut Active < Suspended < Trial, dapat posisi %d/%d/%d", iA, iS, iT)
	}
}

func TestSubscriptionsList_SortStatusDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa StatusActive", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket StA", "PKT-STA", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "100000", "1200000")

	accT := env.seedAccount(t, "Desa StatusTrial", &uid, nil, nil)
	planT := env.seedPlanRow(t, "Paket StT", "PKT-STT", "Core")
	env.seedSubscription(t, accT.ID, planT.ID, &uid, "Trial", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=status&dir=desc&status=all", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa StatusActive")
	iT := strings.Index(body, "Desa StatusTrial")
	if iA < 0 || iT < 0 {
		t.Fatalf("kedua baris status harus tampil:\n%s", body)
	}
	if !(iT < iA) {
		t.Errorf("dir=desc harus urut Trial < Active, dapat posisi %d/%d", iT, iA)
	}
}

// F3 ownership representatif utk salah satu query BARU (predikat ownership TAK
// berubah antar kolom — cukup 1 test tambahan di luar village yang sudah ada).
func TestSubscriptionsList_SortStatusRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-status-sort@x", "member", env.tenantID)

	mine := env.seedAccount(t, "Desa StatusMine", &uid, nil, nil)
	planMine := env.seedPlanRow(t, "Paket StatusMine", "PKT-STMINE", "Core")
	env.seedSubscription(t, mine.ID, planMine.ID, &uid, "Active", "100000", "1200000")

	theirs := env.seedAccount(t, "Desa StatusTheirs", &lain.ID, nil, nil)
	planTheirs := env.seedPlanRow(t, "Paket StatusTheirs", "PKT-STTHEIRS", "Add-on")
	env.seedSubscription(t, theirs.ID, planTheirs.ID, &lain.ID, "Active", "300000", "3600000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=status&dir=asc", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "StatusMine") {
		t.Errorf("sales harus melihat langganan MILIKNYA walau sort=status aktif")
	}
	if strings.Contains(body, "StatusTheirs") {
		t.Errorf("sort=status tak boleh menembus F3: sales melihat langganan milik orang lain")
	}
}

// Paket (plan_name nullable via LEFT JOIN plans, BL-88 PR2b). asc→NULL akhir,
// desc→NULL awal (default Postgres, keputusan user) — satu test tiap arah
// sekaligus menegaskan urutan DAN posisi NULL.
func TestSubscriptionsList_SortPlanAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa PlanAwal", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket Awal", "PKT-PLA", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "100000", "1200000")

	accZ := env.seedAccount(t, "Desa PlanZebra", &uid, nil, nil)
	planZ := env.seedPlanRow(t, "Paket Zebra", "PKT-PLZ", "Core")
	env.seedSubscription(t, accZ.ID, planZ.ID, &uid, "Active", "100000", "1200000")

	accN := env.seedAccount(t, "Desa PlanNull", &uid, nil, nil)
	env.seedSubscriptionNoPlan(t, accN.ID, &uid, "Active", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=plan&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa PlanAwal")
	iZ := strings.Index(body, "Desa PlanZebra")
	iN := strings.Index(body, "Desa PlanNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut Awal < Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

func TestSubscriptionsList_SortPlanDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa PlanAwal", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket Awal", "PKT-PLA", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "100000", "1200000")

	accZ := env.seedAccount(t, "Desa PlanZebra", &uid, nil, nil)
	planZ := env.seedPlanRow(t, "Paket Zebra", "PKT-PLZ", "Core")
	env.seedSubscription(t, accZ.ID, planZ.ID, &uid, "Active", "100000", "1200000")

	accN := env.seedAccount(t, "Desa PlanNull", &uid, nil, nil)
	env.seedSubscriptionNoPlan(t, accN.ID, &uid, "Active", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=plan&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa PlanAwal")
	iZ := strings.Index(body, "Desa PlanZebra")
	iN := strings.Index(body, "Desa PlanNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iZ && iZ < iA) {
		t.Errorf("dir=desc harus urut NULL(awal) < Zebra < Awal, dapat posisi %d/%d/%d", iN, iZ, iA)
	}
}

// MRR (nullable numeric) + pagination representatif utk cursor bertipe numeric.
func TestSubscriptionsList_SortMrrAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	accLow := env.seedAccount(t, "Desa MrrRendah", &uid, nil, nil)
	planLow := env.seedPlanRow(t, "Paket MrrRendah", "PKT-MRRL", "Core")
	env.seedSubscription(t, accLow.ID, planLow.ID, &uid, "Active", "100000", "1200000")

	accHigh := env.seedAccount(t, "Desa MrrTinggi", &uid, nil, nil)
	planHigh := env.seedPlanRow(t, "Paket MrrTinggi", "PKT-MRRH", "Core")
	env.seedSubscription(t, accHigh.ID, planHigh.ID, &uid, "Active", "900000", "10800000")

	accN := env.seedAccount(t, "Desa MrrNull", &uid, nil, nil)
	planN := env.seedPlanRow(t, "Paket MrrNull", "PKT-MRRN", "Core")
	env.seedSubscriptionNullMrr(t, accN.ID, planN.ID, &uid, "Active")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=mrr&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Desa MrrRendah")
	iH := strings.Index(body, "Desa MrrTinggi")
	iN := strings.Index(body, "Desa MrrNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iL < iH && iH < iN) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi < NULL(akhir), dapat posisi %d/%d/%d", iL, iH, iN)
	}
}

func TestSubscriptionsList_SortMrrPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa MrrV" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		plan := env.seedPlanRow(t, "Paket Mrr"+padded(i), "PKT-MRRV"+padded(i), "Core")
		// MRR menaik seiring i agar urutan asc deterministik & unik.
		mrr := itoa(int64(100000 + i*1000))
		env.seedSubscription(t, acc.ID, plan.ID, &uid, "Active", mrr, mrr)
		lastName = name
	}

	first := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=mrr&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "admin", first, env.h.SubscriptionsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("MRR terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/subscriptions")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/subscriptions?sort=mrr&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "admin", second, env.h.SubscriptionsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("MRR terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// Renewal Date (end_date nullable) + pagination representatif utk cursor date.
func TestSubscriptionsList_SortRenewalAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	accEarly := env.seedAccount(t, "Desa RenewAwal", &uid, nil, nil)
	planEarly := env.seedPlanRow(t, "Paket RenewAwal", "PKT-RNA", "Core")
	env.seedSubscriptionEndDate(t, accEarly.ID, planEarly.ID, &uid, "Active", "100000", "1200000",
		time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))

	accLate := env.seedAccount(t, "Desa RenewAkhir", &uid, nil, nil)
	planLate := env.seedPlanRow(t, "Paket RenewAkhir", "PKT-RNZ", "Core")
	env.seedSubscriptionEndDate(t, accLate.ID, planLate.ID, &uid, "Active", "100000", "1200000",
		time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))

	accN := env.seedAccount(t, "Desa RenewNull", &uid, nil, nil)
	planN := env.seedPlanRow(t, "Paket RenewNull", "PKT-RNN", "Core")
	env.seedSubscription(t, accN.ID, planN.ID, &uid, "Active", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=renewal&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iE := strings.Index(body, "Desa RenewAwal")
	iL := strings.Index(body, "Desa RenewAkhir")
	iN := strings.Index(body, "Desa RenewNull")
	if iE < 0 || iL < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iE < iL && iL < iN) {
		t.Errorf("dir=asc harus urut Awal < Akhir < NULL(akhir), dapat posisi %d/%d/%d", iE, iL, iN)
	}
}

func TestSubscriptionsList_SortRenewalPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= pageSize; i++ {
		name := "Desa RenewV" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		plan := env.seedPlanRow(t, "Paket Renew"+padded(i), "PKT-RNV"+padded(i), "Core")
		env.seedSubscriptionEndDate(t, acc.ID, plan.ID, &uid, "Active", "100000", "1200000",
			base.AddDate(0, 0, i))
		lastName = name
	}

	first := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=renewal&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "admin", first, env.h.SubscriptionsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("tanggal terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/subscriptions")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/subscriptions?sort=renewal&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "admin", second, env.h.SubscriptionsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("tanggal terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// CSM (subscription_owner nullable, kunci sort = ownerName/COALESCE(name,email)).
func TestSubscriptionsList_SortCsmAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	memA := env.seedMember(t, "csm-awal@x", "member", env.tenantID)
	memZ := env.seedMember(t, "csm-zebra@x", "member", env.tenantID)

	accA := env.seedAccount(t, "Desa CsmAwal", &memA.ID, nil, nil)
	planA := env.seedPlanRow(t, "Paket CsmAwal", "PKT-CSMA", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &memA.ID, "Active", "100000", "1200000")

	accZ := env.seedAccount(t, "Desa CsmZebra", &memZ.ID, nil, nil)
	planZ := env.seedPlanRow(t, "Paket CsmZebra", "PKT-CSMZ", "Core")
	env.seedSubscription(t, accZ.ID, planZ.ID, &memZ.ID, "Active", "100000", "1200000")

	accN := env.seedAccount(t, "Desa CsmNull", &uid, nil, nil)
	planN := env.seedPlanRow(t, "Paket CsmNull", "PKT-CSMN", "Core")
	env.seedSubscription(t, accN.ID, planN.ID, nil, "Active", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?sort=csm&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa CsmAwal")
	iZ := strings.Index(body, "Desa CsmZebra")
	iN := strings.Index(body, "Desa CsmNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut email-Awal < email-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}
