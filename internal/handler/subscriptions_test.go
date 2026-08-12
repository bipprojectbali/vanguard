package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// subscriptions_test.go — daftar & detail Active Subscriptions (Modul 5, M5-3b
// GET-only) di sisi handler. Tiga sumbu dijaga di sini:
//
//   - F2 (Casbin bisnis): GET butuh crm:subscriptions read (admin/manager/sales/csm
//     lolos; support & "" ditolak 403 — support TAK punya objek ini).
//   - F3 (ownership): sales/csm data_scope 'own' → hanya langganan yang dimilikinya
//     (subscription_owner = uid); di luar cakupan → 404 (keberadaan tak diungkap).
//   - F4 (masking ARR): ARR hanya admin/manager; sales/csm melihat penanda flsHidden.
//     MRR terlihat semua viewer.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji di
// rls_test.go. Setup & helper request memakai ulang setupAccounts/accountsReq/
// runAccount (memuat kedua enforcer + session ber-scope).

// seedSubscription menaruh satu langganan langsung lewat pool (bypass handler)
// dengan pemilik & nilai tertentu — untuk menguji F3/F4 tanpa merangkai create.
// Langganan berbeda diseed pada account+plan BERBEDA agar tak melanggar
// idx_subs_one_active (satu Active per tenant+account+plan).
func (e *testEnv) seedSubscription(
	t *testing.T, accountID, planID int64, owner *int64, status, mrr, arr string,
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
		Status:            status,
		ApprovalStatus:    nil, // seed langsung: jalur non-approval (approval_status NULL)
		AutoRenew:         false,
		Mrr:               numFrom(t, mrr),
		Arr:               numFrom(t, arr),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return s
}

// --- F2: gerbang read ------------------------------------------------------

// TestSubscriptions_GateRead: siapa boleh MEMBUKA daftar (act read).
// admin/manager/sales/csm lolos; support & "" ditolak 403 dengan penjelasan
// butuh peran CRM (support TAK punya objek crm:subscriptions).
func TestSubscriptions_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", true},
		{"csm", true},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/subscriptions", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.SubscriptionsList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menjelaskan butuh peran CRM")
				}
			}
		})
	}
}

// --- F3: ownership ---------------------------------------------------------

// TestSubscriptions_ListScopedByOwnership: sales (data_scope 'own') hanya melihat
// langganan miliknya di daftar; langganan milik anggota lain tak muncul. Manager
// ('all') melihat keduanya.
func TestSubscriptions_ListScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	planA := env.seedPlan(t, "Paket A", "PLAN-A", "1000000")
	planB := env.seedPlan(t, "Paket B", "PLAN-B", "2000000")
	accMine := env.seedAccount(t, "Desa Milikku", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Orang", &other, nil, nil)
	env.seedSubscription(t, accMine.ID, planA, &uid, "Active", "500000", "6000000")
	env.seedSubscription(t, accOther.ID, planB, &other, "Active", "700000", "8400000")

	// Sales own-scope → hanya "Desa Milikku".
	req := accountsReq(http.MethodGet, "/w/test/subscriptions", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Milikku") {
		t.Error("sales harus melihat langganan miliknya (Desa Milikku)")
	}
	if strings.Contains(body, "Desa Orang") {
		t.Error("sales TAK boleh melihat langganan milik anggota lain (Desa Orang)")
	}

	// Manager all-scope → keduanya.
	reqM := accountsReq(http.MethodGet, "/w/test/subscriptions", nil, "")
	recM := env.runAccount(uid, "owner", "manager", reqM, env.h.SubscriptionsList)
	bodyM := recM.Body.String()
	if !strings.Contains(bodyM, "Desa Milikku") || !strings.Contains(bodyM, "Desa Orang") {
		t.Error("manager (all-scope) harus melihat semua langganan")
	}
}

// TestSubscriptions_DetailFound: pemilik (sales own-scope) membuka detail
// langganannya → 200, body memuat nama desa & paket.
func TestSubscriptions_DetailFound(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Detail", "PLAN-DET", "1000000")
	acc := env.seedAccount(t, "Desa Detail", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Detail") {
		t.Error("detail harus memuat nama desa")
	}
	if !strings.Contains(body, "Paket Detail") {
		t.Error("detail harus memuat nama paket")
	}
}

// TestSubscriptions_DetailNotFoundWhenOutOfScope: sales membuka detail langganan
// milik anggota lain → 404 (di luar cakupan; keberadaan baris tak diungkap sbg 403).
func TestSubscriptions_DetailNotFoundWhenOutOfScope(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	planID := env.seedPlan(t, "Paket Orang", "PLAN-OTH", "1000000")
	acc := env.seedAccount(t, "Desa Orang", &other, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &other, "Active", "500000", "6000000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
	if rec.Code != http.StatusNotFound {
		t.Errorf("langganan di luar cakupan harus 404, got %d", rec.Code)
	}
}

// --- F4: masking ARR -------------------------------------------------------

// TestSubscriptions_ARRMaskedForNonManager: ARR disamarkan (flsHidden) untuk
// sales & csm, tampil apa adanya untuk manager. MRR terlihat di KEDUA kasus.
// Diuji di detail (satu halaman memuat MRR & ARR sekaligus).
func TestSubscriptions_ARRMaskedForNonManager(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Nilai", "PLAN-VAL", "1000000")
	acc := env.seedAccount(t, "Desa Nilai", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "5000000", "60000000")

	const wantMRR = "Rp 5.000.000"
	const wantARR = "Rp 60.000.000"

	openDetail := func(role string) string {
		req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
		rec := env.runAccount(uid, "owner", role, req, env.h.SubscriptionDetail)
		if rec.Code != http.StatusOK {
			t.Fatalf("role %q detail status = %d, want 200\n%s", role, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	// Non-manager (sales & csm): ARR tersamar, MRR terlihat.
	for _, role := range []string{"sales", "csm"} {
		body := openDetail(role)
		if !strings.Contains(body, flsHidden) {
			t.Errorf("role %q: ARR harus tersamar (%s)", role, flsHidden)
		}
		if strings.Contains(body, wantARR) {
			t.Errorf("role %q: ARR mentah %q tak boleh bocor", role, wantARR)
		}
		if !strings.Contains(body, wantMRR) {
			t.Errorf("role %q: MRR %q harus terlihat", role, wantMRR)
		}
	}

	// Manager: ARR & MRR keduanya terlihat.
	bodyM := openDetail("manager")
	if !strings.Contains(bodyM, wantARR) {
		t.Errorf("manager: ARR %q harus terlihat", wantARR)
	}
	if !strings.Contains(bodyM, wantMRR) {
		t.Errorf("manager: MRR %q harus terlihat", wantMRR)
	}
}
