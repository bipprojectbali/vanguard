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
//   - F4 (masking ARR): visibilitas ARR = KAPABILITAS ter-matriks crm:subscriptions/arr
//     (BL-58) — default admin/manager; sales/csm/support melihat penanda flsHidden;
//     peran CUSTOM ber-grant melihat ARR. MRR terlihat sales/csm/manager (kebijakan
//     umum canSeeARR, beda dari ARR)
//     — Support (satu-satunya role dikecualikan) tak diuji lewat HTTP di sini
//     krn F2/F3 sudah memblokirnya total dari halaman ini; wiring maskARR MRR
//     diuji langsung di subscriptions_fls_test.go (audit FLS M9-1).
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
		PlanID:            &planID,
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
	// BL-88 PR2b: langganan nyata punya ≥1 subscription_items (agregasi laporan &
	// invarian 1-Active kini via item). Seed 1 item cermin parent agar seed langsung
	// setara jalur Won. account_id/parent_active diturunkan trigger dari parent.
	lineNo := int16(1)
	if _, err := e.q.AddSubscriptionItem(t.Context(), db.AddSubscriptionItemParams{
		SubscriptionID: s.ID,
		TenantID:       e.tenantID,
		PlanID:         &planID,
		Quantity:       1,
		UnitPrice:      numFrom(t, mrr),
		Subtotal:       numFrom(t, mrr),
		Mrr:            numFrom(t, mrr),
		Arr:            numFrom(t, arr),
		LineNo:         &lineNo,
	}); err != nil {
		t.Fatalf("seed subscription item: %v", err)
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

// Test detail halaman (TestSubscriptions_DetailFound, _DetailNotFoundWhenOutOfScope)
// & F4 masking ARR (_ARRMaskedForNonManager, _ARRCustomRoleCapability) dipindah
// ke subscriptions_detail_test.go (BL-154, ukuran file) — bersanding dengan test
// baru kartu-kartu redesign detail. seedSubscription di atas tetap di sini
// (dipakai kedua file).
