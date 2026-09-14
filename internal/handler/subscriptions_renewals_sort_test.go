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

// subscriptions_renewals_sort_test.go — BL-157g: sort per kolom dasbor Renewals
// (5 dari 7 kolom: village/plan/date/type/mrr — Status & Sisa Hari SENGAJA absen,
// lihat komentar renewalSortableColumns di subscriptions_renewals.go). Mirror
// subscriptions_sort_test.go (BL-157a), diadaptasi utk dua hal yang TAK ADA di
// Subscriptions: (1) window_filter — WAJIB diuji ikut terbawa di query sort BARU
// (klon manual, rawan filter tercecer), (2) end_date di sini DIJAMIN terisi
// (ListRenewals: WHERE end_date IS NOT NULL) → kolom "date" pola non-nullable
// (beda dgn "renewal" Subscriptions yang nullable).

// seedRenewalFull = varian seedRenewalSub dgn kontrol PENUH (plan nullable,
// auto_renew, mrr nullable) — seedRenewalSub (subscriptions_renewals_test.go)
// terlalu kaku (Mrr/PreviousValue/AutoRenew tetap) utk asersi urutan sort.
type renewalFullParams struct {
	AccountID     int64
	PlanID        *int64
	Owner         *int64
	Status        string // default "Active"
	EndDate       time.Time
	RenewalStatus string
	RenewalType   string
	AutoRenew     bool
	Mrr           string // "" => NULL
}

func (e *testEnv) seedRenewalFull(t *testing.T, p renewalFullParams) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	var rs, rt *string
	if p.RenewalStatus != "" {
		rs = &p.RenewalStatus
	}
	if p.RenewalType != "" {
		rt = &p.RenewalType
	}
	var mrr pgtype.Numeric
	if p.Mrr != "" {
		mrr = numFrom(t, p.Mrr)
	}
	status := p.Status
	if status == "" {
		status = "Active"
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: p.Owner,
		AccountID:         p.AccountID,
		PlanID:            p.PlanID,
		Status:            status,
		EndDate:           pgtype.Date{Time: p.EndDate, Valid: true},
		AutoRenew:         p.AutoRenew,
		Mrr:               mrr,
		RenewalStatus:     rs,
		RenewalType:       rt,
		CreatedBy:         p.Owner,
	})
	if err != nil {
		t.Fatalf("seed renewal full: %v", err)
	}
	return s
}

func renewalsSortReq(window, sort, dir string) *http.Request {
	target := "/w/test/subscriptions/renewals?window=" + window + "&sort=" + sort + "&dir=" + dir
	return accountsReq(http.MethodGet, target, nil, "")
}

// ── Desa (village_name, non-nullable text) ──

func TestSubscriptionRenewals_SortVillageAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accZ := env.seedAccount(t, "Desa RVZebra", &uid, nil, nil)
	accA := env.seedAccount(t, "Desa RVAwal", &uid, nil, nil)
	accM := env.seedAccount(t, "Desa RVMekar", &uid, nil, nil)
	planID := env.seedPlan(t, "Plan RV", "PL-RV", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accZ.ID, PlanID: &planID, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accA.ID, PlanID: &planID, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accM.ID, PlanID: &planID, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "village", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa RVAwal")
	iM := strings.Index(body, "Desa RVMekar")
	iZ := strings.Index(body, "Desa RVZebra")
	if iA < 0 || iM < 0 || iZ < 0 {
		t.Fatalf("ketiga desa harus tampil:\n%s", body)
	}
	if !(iA < iM && iM < iZ) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iA, iM, iZ)
	}
}

func TestSubscriptionRenewals_SortVillageDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accZ := env.seedAccount(t, "Desa RVDZebra", &uid, nil, nil)
	accA := env.seedAccount(t, "Desa RVDAwal", &uid, nil, nil)
	planID := env.seedPlan(t, "Plan RVD", "PL-RVD", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accZ.ID, PlanID: &planID, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accA.ID, PlanID: &planID, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "village", "desc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa RVDAwal")
	iZ := strings.Index(body, "Desa RVDZebra")
	if iA < 0 || iZ < 0 {
		t.Fatalf("kedua desa harus tampil:\n%s", body)
	}
	if !(iZ < iA) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZ, iA)
	}
}

// F3 ownership representatif — sort tak boleh menembus cakupan kepemilikan.
func TestSubscriptionRenewals_SortVillageRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "renewal-sort-f3@x", "member", 0).ID
	now := time.Now()
	mine := env.seedAccount(t, "Desa RVMine", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa RVTheirs", &other, nil, nil)
	pMine := env.seedPlan(t, "Plan RVMine", "PL-RVM", "1000000")
	pTheirs := env.seedPlan(t, "Plan RVTheirs", "PL-RVT", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: mine.ID, PlanID: &pMine, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: theirs.ID, PlanID: &pTheirs, Owner: &other, EndDate: now.AddDate(0, 0, 10)})

	rec := env.runAccount(uid, "owner", "sales", renewalsSortReq("all", "village", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "RVMine") {
		t.Error("sales harus melihat renewal miliknya walau sort aktif")
	}
	if strings.Contains(body, "RVTheirs") {
		t.Error("sort tak boleh menembus F3: sales melihat renewal milik orang lain")
	}
}

// window_filter representatif — query sort BARU (klon manual) wajib TETAP
// menghormati jendela aktif, bukan cuma ownership. Risiko utama BL-157g: filter
// window tercecer saat kloning ORDER BY baru.
func TestSubscriptionRenewals_SortVillageRespectsWindow(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	due := env.seedAccount(t, "Desa RVDue", &uid, nil, nil)
	grace := env.seedAccount(t, "Desa RVGrace", &uid, nil, nil)
	pDue := env.seedPlan(t, "Plan RVDue", "PL-RVDU", "1000000")
	pGrace := env.seedPlan(t, "Plan RVGrace", "PL-RVGR", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: due.ID, PlanID: &pDue, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: grace.ID, PlanID: &pGrace, Owner: &uid, EndDate: now.AddDate(0, 0, -5)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("due", "village", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa RVDue") {
		t.Error("window=due + sort=village harus tetap memuat baris due")
	}
	if strings.Contains(body, "Desa RVGrace") {
		t.Error("window=due + sort=village TAK boleh memuat baris grace — filter jendela tercecer di query sort")
	}
}

// ── Paket (plan_name, nullable text — BL-88 PR2b multi-plan) ──

func TestSubscriptionRenewals_SortPlanAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accA := env.seedAccount(t, "Desa RPAwal", &uid, nil, nil)
	accZ := env.seedAccount(t, "Desa RPZebra", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa RPNull", &uid, nil, nil)
	pA := env.seedPlan(t, "Paket RPAwal", "PL-RPA", "1000000")
	pZ := env.seedPlan(t, "Paket RPZebra", "PL-RPZ", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accA.ID, PlanID: &pA, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accZ.ID, PlanID: &pZ, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accN.ID, PlanID: nil, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "plan", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa RPAwal")
	iZ := strings.Index(body, "Desa RPZebra")
	iN := strings.Index(body, "Desa RPNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut Awal < Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

func TestSubscriptionRenewals_SortPlanDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accA := env.seedAccount(t, "Desa RPDAwal", &uid, nil, nil)
	accZ := env.seedAccount(t, "Desa RPDZebra", &uid, nil, nil)
	accN := env.seedAccount(t, "Desa RPDNull", &uid, nil, nil)
	pA := env.seedPlan(t, "Paket RPDAwal", "PL-RPDA", "1000000")
	pZ := env.seedPlan(t, "Paket RPDZebra", "PL-RPDZ", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accA.ID, PlanID: &pA, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accZ.ID, PlanID: &pZ, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accN.ID, PlanID: nil, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "plan", "desc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa RPDAwal")
	iZ := strings.Index(body, "Desa RPDZebra")
	iN := strings.Index(body, "Desa RPDNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iZ && iZ < iA) {
		t.Errorf("dir=desc harus urut NULL(awal) < Zebra < Awal, dapat posisi %d/%d/%d", iN, iZ, iA)
	}
}

// ── Tgl Perpanjang (end_date, non-nullable — dijamin ListRenewals) ──

func TestSubscriptionRenewals_SortDateAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	accE := env.seedAccount(t, "Desa RDAwal", &uid, nil, nil)
	accL := env.seedAccount(t, "Desa RDAkhir", &uid, nil, nil)
	pE := env.seedPlan(t, "Plan RDAwal", "PL-RDA", "1000000")
	pL := env.seedPlan(t, "Plan RDAkhir", "PL-RDL", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accE.ID, PlanID: &pE, Owner: &uid, EndDate: base.AddDate(0, 0, 15)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accL.ID, PlanID: &pL, Owner: &uid, EndDate: base.AddDate(0, 11, 0)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "date", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iE := strings.Index(body, "Desa RDAwal")
	iL := strings.Index(body, "Desa RDAkhir")
	if iE < 0 || iL < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iE < iL) {
		t.Errorf("dir=asc harus urut tanggal Awal < Akhir, dapat posisi %d/%d", iE, iL)
	}
}

func TestSubscriptionRenewals_SortDateDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	accE := env.seedAccount(t, "Desa RDDAwal", &uid, nil, nil)
	accL := env.seedAccount(t, "Desa RDDAkhir", &uid, nil, nil)
	pE := env.seedPlan(t, "Plan RDDAwal", "PL-RDDA", "1000000")
	pL := env.seedPlan(t, "Plan RDDAkhir", "PL-RDDL", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accE.ID, PlanID: &pE, Owner: &uid, EndDate: base.AddDate(0, 0, 15)})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accL.ID, PlanID: &pL, Owner: &uid, EndDate: base.AddDate(0, 11, 0)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "date", "desc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iE := strings.Index(body, "Desa RDDAwal")
	iL := strings.Index(body, "Desa RDDAkhir")
	if iE < 0 || iL < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iL < iE) {
		t.Errorf("dir=desc harus urut tanggal Akhir < Awal, dapat posisi %d/%d", iL, iE)
	}
}

// Pagination representatif utk cursor bertipe pgtype.Date (baru di modul ini —
// beda dari "renewal" Subscriptions yang nullable, ::date cast tetap sama tipe).
func TestSubscriptionRenewals_SortDatePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa RDPag" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		plan := env.seedPlan(t, "Plan RDPag"+padded(i), "PL-RDP"+padded(i), "1000000")
		env.seedRenewalFull(t, renewalFullParams{AccountID: acc.ID, PlanID: &plan, Owner: &uid, EndDate: base.AddDate(0, 0, i)})
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "date", "asc"), env.h.SubscriptionRenewals)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("tanggal terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/subscriptions/renewals")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/subscriptions/renewals?window=all&sort=date&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.SubscriptionRenewals)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("tanggal terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// ── Jenis (renewal_type COALESCE auto_renew — TAK PERNAH NULL) ──

func TestSubscriptionRenewals_SortTypeAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accAuto := env.seedAccount(t, "Desa RTAuto", &uid, nil, nil)
	accManual := env.seedAccount(t, "Desa RTManual", &uid, nil, nil)
	accUpsell := env.seedAccount(t, "Desa RTUpsell", &uid, nil, nil)
	pAuto := env.seedPlan(t, "Plan RTAuto", "PL-RTA", "1000000")
	pManual := env.seedPlan(t, "Plan RTManual", "PL-RTM", "1000000")
	pUpsell := env.seedPlan(t, "Plan RTUpsell", "PL-RTU", "1000000")

	// Auto: renewal_type kosong + auto_renew=true → fallback "Auto".
	env.seedRenewalFull(t, renewalFullParams{AccountID: accAuto.ID, PlanID: &pAuto, Owner: &uid, EndDate: now.AddDate(0, 0, 10), AutoRenew: true})
	// Manual: renewal_type kosong + auto_renew=false → fallback "Manual".
	env.seedRenewalFull(t, renewalFullParams{AccountID: accManual.ID, PlanID: &pManual, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})
	// Upsell: renewal_type terisi eksplisit, menang atas auto_renew.
	env.seedRenewalFull(t, renewalFullParams{AccountID: accUpsell.ID, PlanID: &pUpsell, Owner: &uid, EndDate: now.AddDate(0, 0, 10), RenewalType: "Upsell"})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "type", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAuto := strings.Index(body, "Desa RTAuto")
	iManual := strings.Index(body, "Desa RTManual")
	iUpsell := strings.Index(body, "Desa RTUpsell")
	if iAuto < 0 || iManual < 0 || iUpsell < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iAuto < iManual && iManual < iUpsell) {
		t.Errorf("dir=asc harus urut Auto < Manual < Upsell, dapat posisi %d/%d/%d", iAuto, iManual, iUpsell)
	}
}

func TestSubscriptionRenewals_SortTypeDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accAuto := env.seedAccount(t, "Desa RTDAuto", &uid, nil, nil)
	accUpsell := env.seedAccount(t, "Desa RTDUpsell", &uid, nil, nil)
	pAuto := env.seedPlan(t, "Plan RTDAuto", "PL-RTDA", "1000000")
	pUpsell := env.seedPlan(t, "Plan RTDUpsell", "PL-RTDU", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accAuto.ID, PlanID: &pAuto, Owner: &uid, EndDate: now.AddDate(0, 0, 10), AutoRenew: true})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accUpsell.ID, PlanID: &pUpsell, Owner: &uid, EndDate: now.AddDate(0, 0, 10), RenewalType: "Upsell"})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "type", "desc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAuto := strings.Index(body, "Desa RTDAuto")
	iUpsell := strings.Index(body, "Desa RTDUpsell")
	if iAuto < 0 || iUpsell < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iUpsell < iAuto) {
		t.Errorf("dir=desc harus urut Upsell < Auto, dapat posisi %d/%d", iUpsell, iAuto)
	}
}

// ── Prev → Kini (mrr, nullable numeric) ──

func TestSubscriptionRenewals_SortMrrAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	accLow := env.seedAccount(t, "Desa RMRendah", &uid, nil, nil)
	accHigh := env.seedAccount(t, "Desa RMTinggi", &uid, nil, nil)
	accNull := env.seedAccount(t, "Desa RMNull", &uid, nil, nil)
	pLow := env.seedPlan(t, "Plan RMRendah", "PL-RMR", "1000000")
	pHigh := env.seedPlan(t, "Plan RMTinggi", "PL-RMT", "1000000")
	pNull := env.seedPlan(t, "Plan RMNull", "PL-RMN", "1000000")

	env.seedRenewalFull(t, renewalFullParams{AccountID: accLow.ID, PlanID: &pLow, Owner: &uid, EndDate: now.AddDate(0, 0, 10), Mrr: "100000"})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accHigh.ID, PlanID: &pHigh, Owner: &uid, EndDate: now.AddDate(0, 0, 10), Mrr: "900000"})
	env.seedRenewalFull(t, renewalFullParams{AccountID: accNull.ID, PlanID: &pNull, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "mrr", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Desa RMRendah")
	iH := strings.Index(body, "Desa RMTinggi")
	iN := strings.Index(body, "Desa RMNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iL < iH && iH < iN) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi < NULL(akhir), dapat posisi %d/%d/%d", iL, iH, iN)
	}
}

func TestSubscriptionRenewals_SortMrrPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa RMPag" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		plan := env.seedPlan(t, "Plan RMPag"+padded(i), "PL-RMP"+padded(i), "1000000")
		mrr := itoa(int64(100000 + i*1000))
		env.seedRenewalFull(t, renewalFullParams{AccountID: acc.ID, PlanID: &plan, Owner: &uid, EndDate: now.AddDate(0, 0, 10), Mrr: mrr})
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "mrr", "asc"), env.h.SubscriptionRenewals)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("MRR terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/subscriptions/renewals")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/subscriptions/renewals?window=all&sort=mrr&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.SubscriptionRenewals)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("MRR terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// ── ?sort= tak dikenal / tak sortable (Status, Sisa Hari) → fallback default ──

func TestSubscriptionRenewals_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	acc := env.seedAccount(t, "Desa RFallback", &uid, nil, nil)
	planID := env.seedPlan(t, "Plan RFallback", "PL-RFB", "1000000")
	env.seedRenewalFull(t, renewalFullParams{AccountID: acc.ID, PlanID: &planID, Owner: &uid, EndDate: now.AddDate(0, 0, 10)})

	rec := env.runAccount(uid, "owner", "manager", renewalsSortReq("all", "status", "asc"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Desa RFallback") {
		t.Errorf("?sort= tak sortable (mis. status) harus tetap render daftar (jatuh ke default)")
	}
}
