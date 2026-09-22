package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// dashboard_subscription_charts_test.go — BL-142: 2 chart inline section
// Langganan (membalik BL-98 KHUSUS Beranda). Tiga sumbu:
//
//   - F4 LEBIH KETAT dari KPI MRR: "Revenue per Paket" digate
//     canSeeSubscriptionARR(ctx) SETELAH canViewSubscriptions — admin/manager
//     (punya crm:subscriptions/arr) melihatnya; sales/csm (crm:subscriptions
//     TANPA arr) TIDAK, walau section & KPI MRR (tersamar) tetap tampil.
//   - "Renewal per Bulan" (COUNT saja, aman F4) union canViewSubscriptions ATAU
//     canViewChurn — sama cakupan KPI MRR/Churn Rate di section ini.
//   - F3 kepemilikan utk Revenue per Paket: EXISTS subscription_owner (query
//     ReportRevenueByPlan). Dibuktikan lewat nama paket TEST-ONLY unik
//     (bukan kata umum spt "Website" di BL-141 — risiko false-positive dari
//     copy halaman lain nihil) muncul/tak muncul di kategori chart sesuai
//     kepemilikan. Peran custom dgn cakupan 'own' via runAccountScope (pola
//     BL-58) krn peran bawaan ber-arr (admin/manager) defaultnya cakupan 'all'.

// TestDashboardSubscriptionCharts_RevenueGatedByARR: admin/manager/sales/csm
// (crm:subscriptions + arr — grant arr meluas ke Sales & CSM sejak BL-169)
// melihat KEDUA chart baru; support (tanpa kapabilitas domain ini sama
// sekali) tak melihat satupun. Kasus "punya crm:subscriptions TANPA arr"
// (F4 lebih ketat dari union Renewal) diuji lewat peran custom di
// TestDashboardSubscriptionCharts_RevenueGatedByARR_CustomRoleNoARR — peran
// bawaan tak lagi merepresentasikan kombinasi itu sejak BL-169.
func TestDashboardSubscriptionCharts_RevenueGatedByARR(t *testing.T) {
	env, uid := setupAccounts(t)
	plan := env.seedPlan(t, "Paket Chart Gate", "PLAN-CHART-GATE", "1000000")
	acc := env.seedAccount(t, "Desa Chart Sub", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil)

	for _, role := range []string{"admin", "manager", "sales", "csm"} {
		t.Run(role+" melihat kedua chart Langganan", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			for _, id := range []string{"chart-sub-revenue", "chart-sub-renewal"} {
				if !strings.Contains(body, `id="`+id+`"`) {
					t.Errorf("role %q harus melihat %q, body:\n%s", role, id, body)
				}
			}
		})
	}

	t.Run("support tak melihat chart Langganan sama sekali", func(t *testing.T) {
		body := env.dashboardBody(t, uid, "owner", "support")
		for _, id := range []string{"chart-sub-revenue", "chart-sub-renewal"} {
			if strings.Contains(body, id) {
				t.Errorf("support (tanpa kapabilitas Langganan) TAK boleh melihat %q", id)
			}
		}
	})
}

// TestDashboardSubscriptionCharts_RevenueGatedByARR_CustomRoleNoARR: peran
// custom dgn crm:subscriptions read TANPA grant arr — kombinasi yang sejak
// BL-169 tak lagi diwakili peran bawaan manapun (Sales/CSM kini default
// ber-arr) — tetap melihat Renewal per Bulan (union canSubs||canChurn, COUNT
// saja, aman F4) tapi TIDAK melihat Revenue per Paket (F4 lebih ketat).
func TestDashboardSubscriptionCharts_RevenueGatedByARR_CustomRoleNoARR(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "subsnoarr", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "subsnoarr", Obj: "crm:subscriptions", Act: "read"},
	)
	plan := env.seedPlan(t, "Paket Chart NoARR", "PLAN-CHART-NOARR", "1000000")
	acc := env.seedAccount(t, "Desa Chart NoARR", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil)

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "subsnoarr", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="chart-sub-renewal"`) {
		t.Errorf("peran custom (crm:subscriptions tanpa arr) tetap harus melihat chart-sub-renewal, body:\n%s", body)
	}
	if strings.Contains(body, `id="chart-sub-revenue"`) {
		t.Errorf("peran custom (crm:subscriptions TANPA arr) TAK boleh melihat chart-sub-revenue (F4), body:\n%s", body)
	}
}

// TestDashboardSubscriptionCharts_RenewalVisibleForChurnOnly (BL-98 union):
// peran KUSTOM crm:churn-only (tanpa crm:subscriptions) tetap melihat chart
// Renewal per Bulan (union canSubs||canChurn) tapi TAK melihat Revenue per
// Paket (seluruh blok canSubs dilewati, arr tak relevan).
func TestDashboardSubscriptionCharts_RenewalVisibleForChurnOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "churnonly", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "churnonly", Obj: "crm:churn", Act: "read"},
	)
	plan := env.seedPlan(t, "Paket Churn Only", "PLAN-CHURN-ONLY", "1000000")
	acc := env.seedAccount(t, "Desa Churn Only", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil)

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "churnonly", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="chart-sub-renewal"`) {
		t.Errorf("peran churn-only harus tetap melihat chart-sub-renewal (union), body:\n%s", body)
	}
	if strings.Contains(body, `id="chart-sub-revenue"`) {
		t.Error("peran churn-only (tanpa crm:subscriptions) TAK boleh melihat chart-sub-revenue")
	}
}

// TestDashboardSubscriptionCharts_RevenueScopedByOwnership (F3): Revenue per
// Paket menghormati data_scope (EXISTS subscription_owner, ReportRevenueByPlan).
// Peran custom "arrown" (crm:subscriptions read+arr, cakupan 'own' via
// runAccountScope — peran bawaan ber-arr defaultnya 'all', pola sama BL-58)
// hanya melihat paket miliknya sendiri; manager (all-scope, arr bawaan)
// melihat kedua paket.
func TestDashboardSubscriptionCharts_RevenueScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherrevchart@local", "member", 0).ID
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "arrown", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "arrown", Obj: "crm:subscriptions", Act: "read"},
		authz.BusinessPerm{Role: "arrown", Obj: "crm:subscriptions", Act: "arr"},
	)
	planMine := env.seedPlan(t, "Paket ARR Mine", "PLAN-ARR-MINE", "1000000")
	planOther := env.seedPlan(t, "Paket ARR Other", "PLAN-ARR-OTHER", "1000000")
	accMine := env.seedAccount(t, "Desa ARR Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa ARR Other", &other, nil, nil)
	env.seedDashboardSub(t, accMine.ID, planMine, &uid, "Active", "6000000", nil)
	env.seedDashboardSub(t, accOther.ID, planOther, &other, "Active", "6000000", nil)

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	own := env.runAccountScope(uid, "member", "arrown", "own", req, env.h.WorkspaceHome).Body.String()
	if !strings.Contains(own, "Paket ARR Mine") {
		t.Errorf("own-scope harus melihat paket miliknya sendiri (Paket ARR Mine), body:\n%s", own)
	}
	if strings.Contains(own, "Paket ARR Other") {
		t.Errorf("own-scope TAK boleh melihat paket milik orang lain (Paket ARR Other), body:\n%s", own)
	}

	all := env.dashboardBody(t, uid, "owner", "manager")
	if !strings.Contains(all, "Paket ARR Mine") || !strings.Contains(all, "Paket ARR Other") {
		t.Errorf("manager all-scope harus melihat kedua paket, body:\n%s", all)
	}
}
