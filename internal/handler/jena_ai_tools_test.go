package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/session"
)

// jena_ai_tools_test.go — F2/F3/F4 tiap tool Jena AI (jena_ai_tools.go).
// Dispatcher dipanggil LANGSUNG (bukan lewat AskWithTools/proxy) — cukup untuk
// membuktikan penegakan tiga sumbu, sama filosofi accounts_fls_test.go/
// accounts_detail_test.go yang dipakai sbg pola acuan.

// runJena menjalankan fn(ctx) dalam session (uid, businessRole) + Queries
// ber-scope — sama pola runAccount (accounts_test.go), tapi memanggil fn(ctx)
// langsung sebab jena* menerima context, bukan http.HandlerFunc.
func (e *testEnv) runJena(uid int64, businessRole string, fn func(ctx context.Context)) {
	e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", "owner", false,
			e.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, businessRole)
		session.SetBusinessDataScope(ctx, authz.DefaultDataScope(businessRole))
		fn(withQueries(ctx, e.q))
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// --- search_accounts --------------------------------------------------------

func TestJenaSearchAccounts_F2Denied(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)

	var out string
	env.runJena(uid, "", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchAccounts(ctx, json.RawMessage(`{"query":"suka"}`))
		if err != nil {
			t.Fatalf("jenaSearchAccounts: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data desa") {
		t.Errorf("business_role kosong harus ditolak F2, got %s", out)
	}
}

func TestJenaSearchAccounts_F3ScopesToOwn(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-sales@local", "member", 0).ID
	env.seedAccount(t, "Desa Milik Saya", &uid, nil, nil)
	env.seedAccount(t, "Desa Milik Orang Lain", &other, nil, nil)

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchAccounts(ctx, json.RawMessage(`{"query":""}`))
		if err != nil {
			t.Fatalf("jenaSearchAccounts: %v", err)
		}
	})
	if !strings.Contains(out, "Desa Milik Saya") {
		t.Errorf("hasil harus memuat desa milik sendiri:\n%s", out)
	}
	if strings.Contains(out, "Desa Milik Orang Lain") {
		t.Errorf("hasil TAK BOLEH memuat desa milik sales lain (F3):\n%s", out)
	}
}

// --- get_account_summary ----------------------------------------------------

func TestJenaGetAccountSummary_F3NotFoundCrossOwner(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-sales@local", "member", 0).ID
	acc := env.seedAccount(t, "Desa Bukan Milik Saya", &other, nil, nil)

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetAccountSummary(ctx, json.RawMessage(itoaJSON("account_id", acc.ID)))
		if err != nil {
			t.Fatalf("jenaGetAccountSummary: %v", err)
		}
	})
	if !strings.Contains(out, "desa tidak ditemukan") {
		t.Errorf("desa di luar cakupan Sales lain harus \"tidak ditemukan\" (F3), got %s", out)
	}
}

func TestJenaGetAccountSummary_F4BudgetMaskedForSupport(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Anggaran", &uid, nil, nil)
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE accounts SET village_budget=$1 WHERE id=$2`, "5000000", acc.ID); err != nil {
		t.Fatalf("set village_budget: %v", err)
	}

	// Support: data_scope none → tak akan lolos F3 sama sekali; buktikan lewat
	// desa TANPA kepemilikan Support sendiri (mustahil) sekaligus buktikan
	// F4 lewat peran yang F3-nya lolos tapi F4 disamarkan: pakai custom-scope
	// "all" via runAccountScope-style supaya F3 lolos & F4 murni diuji.
	var out string
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", "owner", false,
			env.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, "support")
		session.SetBusinessDataScope(ctx, authz.DataScopeAll) // F3 lolos manual, isolasi F4
		var err error
		out, err = env.h.jenaGetAccountSummary(withQueries(ctx, env.q), json.RawMessage(itoaJSON("account_id", acc.ID)))
		if err != nil {
			t.Fatalf("jenaGetAccountSummary: %v", err)
		}
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(out, flsHidden) {
		t.Errorf("village_budget harus tersamar untuk Support (F4), got %s", out)
	}
	if strings.Contains(out, "5.000.000") || strings.Contains(out, "5000000") {
		t.Errorf("village_budget TAK BOLEH bocor ke Support:\n%s", out)
	}
}

func TestJenaGetAccountSummary_F4BudgetVisibleForSales(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Anggaran Sales", &uid, nil, nil)
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE accounts SET village_budget=$1 WHERE id=$2`, "5000000", acc.ID); err != nil {
		t.Fatalf("set village_budget: %v", err)
	}

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetAccountSummary(ctx, json.RawMessage(itoaJSON("account_id", acc.ID)))
		if err != nil {
			t.Fatalf("jenaGetAccountSummary: %v", err)
		}
	})
	if strings.Contains(out, flsHidden) {
		t.Errorf("village_budget harus tampil untuk Sales (canSeeARR), got %s", out)
	}
}

// --- get_subscription_status -------------------------------------------------

func TestJenaGetSubscriptionStatus_NoSubscription(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Belum Berlangganan", &uid, nil, nil)

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetSubscriptionStatus(ctx, json.RawMessage(itoaJSON("account_id", acc.ID)))
		if err != nil {
			t.Fatalf("jenaGetSubscriptionStatus: %v", err)
		}
	})
	if !strings.Contains(out, "belum punya langganan") {
		t.Errorf("desa tanpa langganan harus balas pesan itu, got %s", out)
	}
}

// TestJenaGetSubscriptionStatus_ArrVisibleForSalesDefault: Sales punya grant
// crm:subscriptions/arr bawaan sejak BL-169 → MRR & ARR SAMA-SAMA tampil
// (satu kapabilitas, satu gerbang — fls.go/canSeeARR memanggil
// subscriptions_view.go/canSeeSubscriptionARR langsung).
func TestJenaGetSubscriptionStatus_ArrVisibleForSalesDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Langganan", &uid, nil, nil)
	planID := env.seedPlan(t, "Plan Uji", "PLAN-JENA", "1000000")
	env.seedSubscription(t, acc.ID, planID, &uid, "Active", "1000000", "12000000")

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetSubscriptionStatus(ctx, json.RawMessage(itoaJSON("account_id", acc.ID)))
		if err != nil {
			t.Fatalf("jenaGetSubscriptionStatus: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if parsed["mrr"] == flsHidden {
		t.Errorf("MRR harus tampil untuk Sales (grant arr bawaan, BL-169), got %v", parsed["mrr"])
	}
	if parsed["arr"] == flsHidden {
		t.Errorf("ARR harus tampil untuk Sales (grant arr bawaan, BL-169), got %v", parsed["arr"])
	}
}

// TestJenaGetSubscriptionStatus_MrrArrSameGateForCustomRole: peran custom
// TANPA grant crm:subscriptions/arr — MRR & ARR SAMA-SAMA tersamar (satu
// kapabilitas sejak BL-169, bukan dua gerbang independen spt sebelumnya).
func TestJenaGetSubscriptionStatus_MrrArrSameGateForCustomRole(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "jenanoarr", Obj: "crm:accounts", Act: "read"},
		authz.BusinessPerm{Role: "jenanoarr", Obj: "crm:subscriptions", Act: "read"},
	)
	acc := env.seedAccount(t, "Desa Langganan NoARR", &uid, nil, nil)
	planID := env.seedPlan(t, "Plan Uji NoARR", "PLAN-JENA-NOARR", "1000000")
	env.seedSubscription(t, acc.ID, planID, &uid, "Active", "1000000", "12000000")

	// Peran custom tak dikenal DefaultDataScope → DataScopeNone (F3 nihil
	// baris). Set eksplisit DataScopeAll agar F3 lolos & F4 (arr) murni diuji
	// (pola sama TestJenaGetAccountSummary_F4BudgetMaskedForSupport).
	var out string
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", "owner", false,
			env.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, "jenanoarr")
		session.SetBusinessDataScope(ctx, authz.DataScopeAll)
		var err error
		out, err = env.h.jenaGetSubscriptionStatus(withQueries(ctx, env.q), json.RawMessage(itoaJSON("account_id", acc.ID)))
		if err != nil {
			t.Fatalf("jenaGetSubscriptionStatus: %v", err)
		}
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if parsed["mrr"] != flsHidden {
		t.Errorf("MRR harus tersamar untuk peran custom tanpa grant arr, got %v", parsed["mrr"])
	}
	if parsed["arr"] != flsHidden {
		t.Errorf("ARR harus tersamar untuk peran custom tanpa grant arr, got %v", parsed["arr"])
	}
}

func TestJenaGetSubscriptionStatus_ArrVisibleForManager(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Langganan Manager", &uid, nil, nil)
	planID := env.seedPlan(t, "Plan Uji 2", "PLAN-JENA-2", "1000000")
	env.seedSubscription(t, acc.ID, planID, &uid, "Active", "1000000", "12000000")

	var out string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetSubscriptionStatus(ctx, json.RawMessage(itoaJSON("account_id", acc.ID)))
		if err != nil {
			t.Fatalf("jenaGetSubscriptionStatus: %v", err)
		}
	})
	if strings.Contains(out, flsHidden) {
		t.Errorf("Manager punya crm:subscriptions/arr, ARR & MRR harus tampil, got %s", out)
	}
}
