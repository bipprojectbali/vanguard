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

// subscriptions_churn_dashboard_test.go — BL-92 dasbor Churn didesain ulang. Yang
// dikunci: banner peringatan (token warning), 4 kartu KPI berjendela tetap & SELALU
// global (tab Tipe tak memengaruhi KPI), kolom Tenure (bulan), dan ekspor CSV read-only
// yang ter-scope kepemilikan (F3) persis tabel HTML.

// seedChurnedSubDated seperti seedChurnedSub tapi menetapkan start_date &
// cancellation_date eksplisit → Tenure/Avg Tenure deterministik.
func (e *testEnv) seedChurnedSubDated(
	t *testing.T, accountID, planID int64, owner *int64, churnType, lostMRR string, start, cancel time.Time,
) db.Subscription {
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
		PlanID:            planID,
		Status:            "Churned",
		StartDate:         pgtype.Date{Time: start, Valid: true},
		AutoRenew:         false,
		Mrr:               numFrom(t, lostMRR),
		Arr:               numFrom(t, "6000000"),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed dated churned subscription: %v", err)
	}
	ct, reason := churnType, "Budget"
	if err := e.q.ChurnSubscription(t.Context(), db.ChurnSubscriptionParams{
		Status:           "Churned",
		CancellationDate: pgtype.Date{Time: cancel, Valid: true},
		ChurnReason:      &reason,
		ChurnType:        &ct,
		LostValueMrr:     numFrom(t, lostMRR),
		UpdatedBy:        owner,
		ID:               s.ID,
	}); err != nil {
		t.Fatalf("churn dated subscription: %v", err)
	}
	return s
}

// TestSubscriptionChurnList_KPIs: 4 kartu KPI ter-render dengan nilai benar (Churn
// Rate 50%, Desa Churn 1 dari 1 aktif, Avg Tenure 3 bln, Churned MRR Rp 500.000).
// Dasbor read-only (tak ada route tulis / form churn).
func TestSubscriptionChurnList_KPIs(t *testing.T) {
	env, uid := setupAccounts(t)
	gone := env.seedAccount(t, "Desa Gone", &uid, nil, nil)
	live := env.seedAccount(t, "Desa Live", &uid, nil, nil)
	pGone := env.seedPlan(t, "Plan Gone", "PL-GONE", "1000000")
	pLive := env.seedPlan(t, "Plan Live", "PL-LIVE", "1000000")
	now := time.Now().In(appTZ)
	// 90 hari ≈ 3 bln (90/30.44 ≈ 2.96 → 3), cancel hari ini (masuk 30 hari & bulan ini).
	env.seedChurnedSubDated(t, gone.ID, pGone, &uid, "Voluntary", "500000", now.AddDate(0, 0, -90), now)
	env.seedSubscription(t, live.ID, pLive, &uid, "Active", "700000", "8400000")

	rec := env.runAccount(uid, "owner", "manager", churnListReq(""), env.h.SubscriptionChurnList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	for _, label := range []string{"Churn Rate", "Churned MRR", "Desa Churn", "Avg Tenure"} {
		if !strings.Contains(body, label) {
			t.Errorf("kartu KPI %q harus tampil", label)
		}
	}
	if !strings.Contains(body, "50,0%") {
		t.Error("Churn Rate harus 50,0% (1 churn dari 2 = active1+churn1)")
	}
	if !strings.Contains(body, "dari 1 aktif") {
		t.Error("Desa Churn harus sub 'dari 1 aktif'")
	}
	if !strings.Contains(body, "3 bln") {
		t.Error("Avg Tenure harus 3 bln (90 hari)")
	}
	if !strings.Contains(body, "Rp 500.000") {
		t.Error("Churned MRR bulan ini harus Rp 500.000 (manager lihat nilai)")
	}
}

// TestSubscriptionChurnList_KPIGlobalNotFilteredByType: KPI SELALU global — memfilter
// tab Tipe hanya menyaring tabel, bukan angka KPI. Dua churn beda tipe → KPI sama
// (Desa Churn 'dari' & Churn Rate) di semua tab, walau tabel berbeda.
func TestSubscriptionChurnList_KPIGlobalNotFilteredByType(t *testing.T) {
	env, uid := setupAccounts(t)
	vol := env.seedAccount(t, "Desa Vol", &uid, nil, nil)
	invol := env.seedAccount(t, "Desa Invol", &uid, nil, nil)
	pVol := env.seedPlan(t, "Plan Vol", "PL-VOL", "1000000")
	pInvol := env.seedPlan(t, "Plan Invol", "PL-INV", "1000000")
	now := time.Now().In(appTZ)
	env.seedChurnedSubDated(t, vol.ID, pVol, &uid, "Voluntary", "500000", now.AddDate(0, 0, -60), now)
	env.seedChurnedSubDated(t, invol.ID, pInvol, &uid, "Involuntary", "600000", now.AddDate(0, 0, -60), now)

	// Semua tab harus menampilkan Desa Churn = 2 (global), walau tabel disaring tipe.
	for _, typ := range []string{"", "Voluntary", "Involuntary"} {
		rec := env.runAccount(uid, "owner", "manager", churnListReq(typ), env.h.SubscriptionChurnList)
		if rec.Code != http.StatusOK {
			t.Fatalf("type %q: status %d", typ, rec.Code)
		}
		body := rec.Body.String()
		// Churned MRR = Rp 1.100.000 (500rb+600rb, GLOBAL). Bila KPI keliru disaring
		// tipe, tab Voluntary hanya Rp 500.000 → anggap gagal.
		if !strings.Contains(body, "Rp 1.100.000") {
			t.Errorf("type %q: KPI Churned MRR harus Rp 1.100.000 (global, tak disaring tipe)", typ)
		}
	}

	// Tabel TETAP disaring tipe: type=Voluntary → Desa Vol saja.
	recVol := env.runAccount(uid, "owner", "manager", churnListReq("Voluntary"), env.h.SubscriptionChurnList)
	bodyVol := recVol.Body.String()
	if !strings.Contains(bodyVol, "Desa Vol") || strings.Contains(bodyVol, "Desa Invol") {
		t.Error("tabel type=Voluntary harus hanya Desa Vol")
	}
}

// TestSubscriptionChurnList_TenureColumn: kolom Tenure ada di header & baris memuat
// masa langganan (bulan) dari cancellation_date − start_date.
func TestSubscriptionChurnList_TenureColumn(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Tenure", &uid, nil, nil)
	plan := env.seedPlan(t, "Plan Tenure", "PL-TEN", "1000000")
	now := time.Now().In(appTZ)
	env.seedChurnedSubDated(t, acc.ID, plan, &uid, "Voluntary", "500000", now.AddDate(0, 0, -180), now)

	rec := env.runAccount(uid, "owner", "manager", churnListReq(""), env.h.SubscriptionChurnList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Tenure") {
		t.Error("header tabel harus punya kolom Tenure")
	}
	// 180 hari / 30.44 ≈ 5.91 → 6 bln.
	if !strings.Contains(body, "6 bln") {
		t.Error("baris Tenure harus 6 bln (180 hari)")
	}
}

// TestSubscriptionChurnExport_CSV: ekspor CSV read-only — content-type text/csv,
// header kolom + baris churn ter-scope. Sales (own-scope) hanya dapat miliknya.
func TestSubscriptionChurnExport_CSV(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	mine := env.seedAccount(t, "Desa Mine", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Theirs", &other, nil, nil)
	pM := env.seedPlan(t, "Plan Mine", "PL-MINE", "1000000")
	pT := env.seedPlan(t, "Plan Theirs", "PL-THR", "1000000")
	now := time.Now().In(appTZ)
	env.seedChurnedSubDated(t, mine.ID, pM, &uid, "Voluntary", "500000", now.AddDate(0, 0, -30), now)
	env.seedChurnedSubDated(t, theirs.ID, pT, &other, "Voluntary", "500000", now.AddDate(0, 0, -30), now)

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/churn/export", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionChurnExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa,Paket,MRR Hilang,Alasan,Tipe,Tgl Churn,Tenure,CS") {
		t.Errorf("header CSV hilang/berubah\n%s", body)
	}
	if !strings.Contains(body, "Desa Mine") {
		t.Error("CSV sales harus memuat churn miliknya (Desa Mine)")
	}
	if strings.Contains(body, "Desa Theirs") {
		t.Error("CSV sales TAK boleh memuat churn milik anggota lain (Desa Theirs)")
	}
}

// TestSubscriptionChurnExport_GateRead & TypeFilter: gerbang read sama (support 403);
// ?type= menyaring baris CSV persis tabel.
func TestSubscriptionChurnExport_GateAndType(t *testing.T) {
	env, uid := setupAccounts(t)
	vol := env.seedAccount(t, "Desa Vol", &uid, nil, nil)
	invol := env.seedAccount(t, "Desa Invol", &uid, nil, nil)
	pVol := env.seedPlan(t, "Plan Vol", "PL-VOL", "1000000")
	pInvol := env.seedPlan(t, "Plan Invol", "PL-INV", "1000000")
	now := time.Now().In(appTZ)
	env.seedChurnedSubDated(t, vol.ID, pVol, &uid, "Voluntary", "500000", now.AddDate(0, 0, -30), now)
	env.seedChurnedSubDated(t, invol.ID, pInvol, &uid, "Involuntary", "600000", now.AddDate(0, 0, -30), now)

	// Support tanpa objek crm:subscriptions → 403.
	reqAll := accountsReq(http.MethodGet, "/w/test/subscriptions/churn/export", nil, "")
	if rec := env.runAccount(uid, "owner", "support", reqAll, env.h.SubscriptionChurnExport); rec.Code != http.StatusForbidden {
		t.Errorf("support harus 403, got %d", rec.Code)
	}

	// type=Involuntary → hanya Desa Invol di CSV.
	reqInv := accountsReq(http.MethodGet, "/w/test/subscriptions/churn/export?type=Involuntary", nil, "")
	rec := env.runAccount(uid, "owner", "manager", reqInv, env.h.SubscriptionChurnExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Invol") || strings.Contains(body, "Desa Vol") {
		t.Error("CSV type=Involuntary harus hanya Desa Invol")
	}
}
