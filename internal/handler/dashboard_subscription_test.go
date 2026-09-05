package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// dashboard_subscription_test.go — section "Langganan" Beranda (Modul 1,
// BL-59b). Dipisah dari dashboard_test.go (ukuran file). Tiga sumbu dijaga:
//
//   - Visibilitas per-kapabilitas: role dgn ≥1 kapabilitas Subscription
//     (admin/manager/sales/csm) melihat heading "Langganan"; support tidak.
//   - Komposisi union parsial: role KUSTOM crm:renewals-only melihat butir
//     Renewal saja, tanpa MRR/Revenue.
//   - F3 kepemilikan: "Langganan Aktif" menghormati data_scope (own vs all).
//
// Setup/helper reuse dashboard_test.go (seedDashboardSub, dashboardBody,
// dashboardKPIValue) & accounts_test.go (setupAccounts).

// TestDashboardSubscription_DomainVisibleByCapability: role dgn kapabilitas
// Subscription bawaan melihat heading "Langganan" + KPI inti; support (tanpa
// kapabilitas apa pun di domain ini) TAK melihatnya (heading tak berdiri kosong).
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
			for _, kpi := range []string{"MRR", "Renewal Rate", "Churn Rate"} {
				if !strings.Contains(body, ">"+kpi+"</p>") {
					t.Errorf("role %q harus melihat KPI %q di section Langganan", role, kpi)
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

// TestDashboardSubscription_CustomRoleRenewalsOnly: role KUSTOM dgn hanya
// crm:dashboard + crm:renewals (read) melihat section Langganan berisi HANYA
// butir Renewal (Renewal Rate + Jatuh Tempo 30 Hari), TANPA MRR (crm:subscriptions)
// & TANPA chart Revenue/MRR-movement — bukti komposisi union parsial per-kapabilitas.
func TestDashboardSubscription_CustomRoleRenewalsOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "renewalonly", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "renewalonly", Obj: "crm:renewals", Act: "read"},
	)
	plan := env.seedPlan(t, "Paket Custom", "PLAN-CUST-SUB", "1000000")
	acc := env.seedAccount(t, "Desa Custom Sub", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil)

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "renewalonly", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">Langganan</h2>") {
		t.Errorf("role kustom (crm:renewals) harus melihat section Langganan, body:\n%s", body)
	}
	if !strings.Contains(body, ">Renewal Rate</p>") {
		t.Error("role kustom (crm:renewals) harus melihat KPI Renewal Rate")
	}
	if !strings.Contains(body, ">Jatuh Tempo 30 Hari</p>") {
		t.Error("role kustom (crm:renewals) harus melihat KPI Jatuh Tempo 30 Hari")
	}
	if strings.Contains(body, ">MRR</p>") {
		t.Error("role kustom tanpa crm:subscriptions TAK boleh melihat KPI MRR")
	}
	if strings.Contains(body, "chart-revenue-plan") {
		t.Error("role kustom tanpa crm:plans TAK boleh melihat chart Revenue per Paket")
	}
	if strings.Contains(body, "chart-mrr-movement") {
		t.Error("role kustom tanpa crm:churn TAK boleh melihat chart MRR Baru vs Churn")
	}
}

// TestDashboardSubscription_ActiveScopedByOwnership (F3): KPI "Langganan Aktif"
// menghormati data_scope — sales (own) hanya menghitung langganan miliknya;
// manager (all) menghitung lintas-owner. Filter = SubscriptionsListFilterFor,
// sama dgn ListSubscriptions (bukan logic scope duplikat).
func TestDashboardSubscription_ActiveScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "othersub@local", "member", 0).ID
	plan := env.seedPlan(t, "Paket Scope", "PLAN-SCOPE-SUB", "1000000")
	accMine := env.seedAccount(t, "Desa Sub Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Sub Other", &other, nil, nil)
	env.seedDashboardSub(t, accMine.ID, plan, &uid, "Active", "6000000", nil)
	env.seedDashboardSub(t, accOther.ID, plan, &other, "Active", "6000000", nil)

	const label = "Langganan Aktif"
	own := env.dashboardBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales own-scope: %q = %q, want \"1\" (hanya miliknya)", label, got)
	}
	all := env.dashboardBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager all-scope: %q = %q, want \"2\" (lintas-owner)", label, got)
	}
}
