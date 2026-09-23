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
