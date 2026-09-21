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

// health_score_sort_test.go — BL-157i: sort per kolom tabel Health Score (7 dari
// 9 kolom: village/score/adoption/engagement/support/trend/renewal — Status &
// kolom aksi SENGAJA absen, lihat komentar healthScoreSortableColumns di
// health_score.go). Mirror subscriptions_renewals_sort_test.go/
// sales_deals_sort_test.go, diadaptasi: (1) populasi Health Score = desa
// PELANGGAN (BL-114, butuh ≥1 langganan hidup) — seedHealthFull SENDIRI TAK
// cukup, tiap desa yang diharapkan tampil wajib juga makeCustomer/seedHealthSub;
// (2) 4 kolom smallint nullable (score/adoption/engagement/support) — hanya
// "score" diuji lengkap (asc/desc/null/pagination), 3 lainnya representatif 1
// arah (pola query identik, klon manual, risiko sama).

// --- helper ------------------------------------------------------------

// i16p/strp = pointer literal utk field nullable CreateCustomerSuccessParams.
func i16p(v int16) *int16    { return &v }
func strp2(v string) *string { return &v }

type healthFullParams struct {
	AccountID  int64
	Score      *int16
	Status     string // "" => NULL
	Adoption   *int16
	Engagement *int16
	Support    *int16
	Trend      *string
}

// seedHealthFull menaruh satu baris customer_success dengan kontrol PENUH atas
// kolom nullable — seedHealthScore (health_score_test.go) hanya kontrol
// score+status, tak cukup utk uji nulls-first/last kolom adoption/engagement/
// support/trend.
func (e *testEnv) seedHealthFull(t *testing.T, p healthFullParams) {
	t.Helper()
	var status *string
	if p.Status != "" {
		status = &p.Status
	}
	_, err := e.q.CreateCustomerSuccess(t.Context(), db.CreateCustomerSuccessParams{
		TenantID:           e.tenantID,
		AccountID:          p.AccountID,
		OverallHealthScore: p.Score,
		HealthStatus:       status,
		AdoptionScore:      p.Adoption,
		EngagementScore:    p.Engagement,
		SupportScore:       p.Support,
		ScoreTrend:         p.Trend,
		UsageDataSource:    "Manual", // BL-27: kolom wajib (CHECK)
	})
	if err != nil {
		t.Fatalf("seed health full: %v", err)
	}
}

// seedHealthSub menaruh satu langganan Active langsung lewat pool (bypass
// makeCustomer) — dipakai saat butuh end_date eksplisit (kolom "renewal") atau
// desa TANPA end_date (renewal_end_date NULL) walau tetap jadi pelanggan
// (populasi segmen active). Plan unik per akun agar lolos idx_subs_one_active.
// TANPA subscription_items (mirror seedRenewalFull) — tak dibutuhkan agregasi
// health score.
func (e *testEnv) seedHealthSub(t *testing.T, accountID int64, endDate time.Time) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	planID := e.seedPlan(t, "Paket HS"+itoa(accountID), "PL-HS"+itoa(accountID), "100000")
	var ed pgtype.Date
	if !endDate.IsZero() {
		ed = pgtype.Date{Time: endDate, Valid: true}
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:   e.tenantID,
		EntityCode: &code,
		AccountID:  accountID,
		PlanID:     &planID,
		Status:     "Active",
		EndDate:    ed,
		AutoRenew:  false,
		Mrr:        numFrom(t, "100000"),
		Arr:        numFrom(t, "1200000"),
	})
	if err != nil {
		t.Fatalf("seed health sub: %v", err)
	}
	return s
}

func healthSortReq(sort, dir string) *http.Request {
	target := "/w/test/health-scores?sort=" + sort + "&dir=" + dir
	return accountsReq(http.MethodGet, target, nil, "")
}

// ── Desa (village_name, non-nullable text) ──

func TestHealthScore_SortVillageAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accZ := env.seedAccount(t, "Desa HVZebra", &uid, nil, nil)
	accA := env.seedAccount(t, "Desa HVAwal", &uid, nil, nil)
	accM := env.seedAccount(t, "Desa HVMekar", &uid, nil, nil)
	env.makeCustomer(t, accZ.ID, "Active")
	env.makeCustomer(t, accA.ID, "Active")
	env.makeCustomer(t, accM.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("village", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa HVAwal")
	iM := strings.Index(body, "Desa HVMekar")
	iZ := strings.Index(body, "Desa HVZebra")
	if iA < 0 || iM < 0 || iZ < 0 {
		t.Fatalf("ketiga desa harus tampil:\n%s", body)
	}
	if !(iA < iM && iM < iZ) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iA, iM, iZ)
	}
}

func TestHealthScore_SortVillageDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	accZ := env.seedAccount(t, "Desa HVDZebra", &uid, nil, nil)
	accA := env.seedAccount(t, "Desa HVDAwal", &uid, nil, nil)
	env.makeCustomer(t, accZ.ID, "Active")
	env.makeCustomer(t, accA.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("village", "desc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa HVDAwal")
	iZ := strings.Index(body, "Desa HVDZebra")
	if iA < 0 || iZ < 0 {
		t.Fatalf("kedua desa harus tampil:\n%s", body)
	}
	if !(iZ < iA) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZ, iA)
	}
}

// Pagination representatif utk cursor bertipe TEXT non-nullable.
func TestHealthScore_SortVillagePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa HVPag" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		env.makeCustomer(t, acc.ID, "Active")
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", healthSortReq("village", "asc"), env.h.HealthScoreList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("desa terakhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/health-scores")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/health-scores?sort=village&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.HealthScoreList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("desa terakhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// F3 ownership representatif — sort tak boleh menembus cakupan kepemilikan.
func TestHealthScore_SortVillageRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "health-sort-f3@x", "member", 0).ID
	mine := env.seedAccount(t, "Desa HMine", nil, &uid, nil)
	theirs := env.seedAccount(t, "Desa HTheirs", nil, &other, nil)
	env.makeCustomer(t, mine.ID, "Active")
	env.makeCustomer(t, theirs.ID, "Active")

	rec := env.runAccount(uid, "member", "csm", healthSortReq("village", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "HMine") {
		t.Error("CSM harus melihat desa binaannya walau sort aktif")
	}
	if strings.Contains(body, "HTheirs") {
		t.Error("sort tak boleh menembus F3: CSM melihat desa milik user lain")
	}
}

// Kombinasi sort+tab+segment — risiko utama BL-157i: filter status/segment
// tercecer saat kloning 7 query baru.
func TestHealthScore_SortVillageRespectsTabAndSegment(t *testing.T) {
	env, uid := setupAccounts(t)
	sehat := env.seedAccount(t, "Desa HCombSehat", &uid, nil, nil)
	kritis := env.seedAccount(t, "Desa HCombKritis", &uid, nil, nil)
	churned := env.seedAccount(t, "Desa HCombChurned", &uid, nil, nil)
	env.seedHealthScore(t, sehat.ID, 85, "Healthy")
	env.makeCustomer(t, sehat.ID, "Active")
	env.seedHealthScore(t, kritis.ID, 20, "Critical")
	env.makeCustomer(t, kritis.ID, "Active")
	env.makeCustomer(t, churned.ID, "Cancelled")

	target := "/w/test/health-scores?tab=sehat&segment=active&sort=village&dir=asc"
	rec := env.runAccount(uid, "owner", "manager", accountsReq(http.MethodGet, target, nil, ""), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "HCombSehat") {
		t.Error("tab=sehat + sort=village harus tetap memuat baris Healthy")
	}
	if strings.Contains(body, "HCombKritis") {
		t.Error("tab=sehat TAK boleh memuat baris Critical — filter tab tercecer di query sort")
	}
	if strings.Contains(body, "HCombChurned") {
		t.Error("segment=active TAK boleh memuat desa churned — filter segment tercecer di query sort")
	}
}

// ── Skor (overall_health_score, smallint nullable) ──

func TestHealthScore_SortScoreAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	accLow := env.seedAccount(t, "Desa HSRendah", &uid, nil, nil)
	accHigh := env.seedAccount(t, "Desa HSTinggi", &uid, nil, nil)
	accNull := env.seedAccount(t, "Desa HSNull", &uid, nil, nil)
	env.seedHealthScore(t, accLow.ID, 20, "Critical")
	env.makeCustomer(t, accLow.ID, "Active")
	env.seedHealthScore(t, accHigh.ID, 90, "Healthy")
	env.makeCustomer(t, accHigh.ID, "Active")
	env.makeCustomer(t, accNull.ID, "Active") // tanpa customer_success → score NULL

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("score", "asc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Desa HSRendah")
	iH := strings.Index(body, "Desa HSTinggi")
	iN := strings.Index(body, "Desa HSNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iL < iH && iH < iN) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi < NULL(akhir), dapat posisi %d/%d/%d", iL, iH, iN)
	}
}

func TestHealthScore_SortScoreDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	accLow := env.seedAccount(t, "Desa HSDRendah", &uid, nil, nil)
	accHigh := env.seedAccount(t, "Desa HSDTinggi", &uid, nil, nil)
	accNull := env.seedAccount(t, "Desa HSDNull", &uid, nil, nil)
	env.seedHealthScore(t, accLow.ID, 20, "Critical")
	env.makeCustomer(t, accLow.ID, "Active")
	env.seedHealthScore(t, accHigh.ID, 90, "Healthy")
	env.makeCustomer(t, accHigh.ID, "Active")
	env.makeCustomer(t, accNull.ID, "Active")

	rec := env.runAccount(uid, "owner", "manager", healthSortReq("score", "desc"), env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, "Desa HSDRendah")
	iH := strings.Index(body, "Desa HSDTinggi")
	iN := strings.Index(body, "Desa HSDNull")
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iH && iH < iL) {
		t.Errorf("dir=desc harus urut NULL(awal) < Tinggi < Rendah, dapat posisi %d/%d/%d", iN, iH, iL)
	}
}

// Pagination representatif utk cursor bertipe SMALLINT NULLABLE.
func TestHealthScore_SortScorePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa HSPag" + padded(i)
		acc := env.seedAccount(t, name, &uid, nil, nil)
		env.seedHealthScore(t, acc.ID, int16(i), "Healthy")
		env.makeCustomer(t, acc.ID, "Active")
		lastName = name
	}

	rec1 := env.runAccount(uid, "owner", "manager", healthSortReq("score", "asc"), env.h.HealthScoreList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("skor tertinggi (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/health-scores")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/health-scores?sort=score&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "manager", second, env.h.HealthScoreList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("skor tertinggi harus muncul di halaman kedua — daftar masih terpotong")
	}
}

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
