package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// dashboard_subscription_test.go — section "Langganan" Beranda (Modul 1,
// BL-59b + BL-98). Dipisah dari dashboard_test.go (ukuran file). Tiga sumbu:
//
//   - Visibilitas per-kapabilitas: role dgn ≥1 kapabilitas Subscription
//     (admin/manager/sales/csm) melihat heading "Langganan"; support tidak.
//   - Komposisi union parsial (BL-98): role KUSTOM crm:churn-only melihat butir
//     Churn Rate saja, tanpa MRR (crm:subscriptions).
//   - F3 kepemilikan: "Churn Rate" menghormati data_scope (own vs all) — MRR
//     tersamar F4 utk sales/csm, jadi churn% (dari ReportRetention) yang dipakai
//     membuktikan cakupan kepemilikan.
//
// BL-98: section kini MAKS 2 KPI (MRR · Churn Rate) TANPA chart domain; ARR,
// Langganan Aktif, Renewal Rate/Jatuh Tempo dipindah ke Subscription Report.
//
// Setup/helper reuse dashboard_test.go (seedDashboardSub, dashboardBody,
// dashboardKPIValue) & accounts_test.go (setupAccounts).

// TestDashboardSubscription_DomainVisibleByCapability: role dgn kapabilitas
// Subscription bawaan melihat heading "Langganan" + 2 KPI ramping; support
// (tanpa kapabilitas apa pun di domain ini) TAK melihatnya.
func TestDashboardSubscription_DomainVisibleByCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	plan := env.seedPlan(t, "Paket Beranda", "PLAN-DOM-SUB", "1000000")
	acc := env.seedAccount(t, "Desa Sub Dom", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil)

	for _, role := range []string{"admin", "manager", "sales", "csm"} {
		t.Run(role+" melihat section Langganan", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			if !strings.Contains(body, ">Langganan</h2>") {
				t.Errorf("role %q harus melihat heading section Langganan, body:\n%s", role, body)
			}
			for _, kpi := range []string{"MRR", "Churn Rate"} {
				if !strings.Contains(body, ">"+kpi+"</p>") {
					t.Errorf("role %q harus melihat KPI %q di section Langganan", role, kpi)
				}
			}
			// BL-98: butir yang dipindah ke Report tak boleh muncul di Beranda.
			for _, dropped := range []string{">Renewal Rate</p>", ">Langganan Aktif</p>"} {
				if strings.Contains(body, dropped) {
					t.Errorf("role %q: KPI %q sudah dipindah ke Report, tak boleh di Beranda", role, dropped)
				}
			}
		})
	}

	t.Run("support tak melihat section Langganan", func(t *testing.T) {
		body := env.dashboardBody(t, uid, "owner", "support")
		if strings.Contains(body, ">Langganan</h2>") {
			t.Errorf("support (tanpa kapabilitas Subscription) TAK boleh melihat section Langganan, body:\n%s", body)
		}
	})
}

// TestDashboardSubscription_CustomRoleChurnOnly (BL-98): role KUSTOM dgn hanya
// crm:dashboard + crm:churn (read) melihat section Langganan berisi HANYA Churn
// Rate, TANPA MRR (crm:subscriptions) — bukti komposisi union parsial per-
// kapabilitas setelah dirampingkan.
func TestDashboardSubscription_CustomRoleChurnOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "churnonly", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "churnonly", Obj: "crm:churn", Act: "read"},
	)
	plan := env.seedPlan(t, "Paket Custom", "PLAN-CUST-SUB", "1000000")
	acc := env.seedAccount(t, "Desa Custom Sub", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil)

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "churnonly", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">Langganan</h2>") {
		t.Errorf("role kustom (crm:churn) harus melihat section Langganan, body:\n%s", body)
	}
	if !strings.Contains(body, ">Churn Rate</p>") {
		t.Error("role kustom (crm:churn) harus melihat KPI Churn Rate")
	}
	if strings.Contains(body, ">MRR</p>") {
		t.Error("role kustom tanpa crm:subscriptions TAK boleh melihat KPI MRR")
	}
}

// TestDashboardSubscription_ChurnScopedByOwnership (F3): KPI "Churn Rate"
// menghormati data_scope — sales (own) hanya menghitung langganan miliknya;
// manager (all) menghitung lintas-owner. Filter = SubscriptionsListFilterFor,
// sama dgn ListSubscriptions. Dipakai churn% (bukan MRR — MRR tersamar F4 utk
// sales) sebagai bukti cakupan kepemilikan.
//
// Skenario: langganan CHURNED milik sendiri + langganan ACTIVE milik orang lain.
//   - sales (own): churned=1, active=0 → 1/1 = "100,0%".
//   - manager (all): churned=1, active=1 → 1/2 = "50,0%".
func TestDashboardSubscription_ChurnScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "othersub@local", "member", 0).ID
	plan := env.seedPlan(t, "Paket Scope", "PLAN-SCOPE-SUB", "1000000")
	accMine := env.seedAccount(t, "Desa Sub Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Sub Other", &other, nil, nil)
	env.seedDashboardSub(t, accMine.ID, plan, &uid, "Churned", "6000000", nil)
	env.seedDashboardSub(t, accOther.ID, plan, &other, "Active", "6000000", nil)

	const label = "Churn Rate"
	own := env.dashboardBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "100,0%" {
		t.Errorf("sales own-scope: %q = %q, want \"100,0%%\" (churned 1 / total 1 miliknya)", label, got)
	}
	all := env.dashboardBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "50,0%" {
		t.Errorf("manager all-scope: %q = %q, want \"50,0%%\" (churned 1 / total 2 lintas-owner)", label, got)
	}
}
