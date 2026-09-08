package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// dashboard_cs_test.go — section "Customer Success" Beranda (Modul 1, BL-59c +
// BL-98). Dipisah dari dashboard_test.go (ukuran file). Tiga sumbu dijaga:
//
//   - Visibilitas per-kapabilitas: role dgn kapabilitas CS (admin/manager/csm)
//     melihat heading "Customer Success" + KPI inti; role KUSTOM tanpa kapabilitas
//     CS apa pun TAK melihat heading (tak berdiri kosong).
//   - Komposisi union parsial (BL-98): role KUSTOM crm:health-only melihat butir
//     Health saja (Desa Berisiko), tanpa Adoption Rate (crm:adoption).
//   - F3 kepemilikan: "Desa Berisiko" menghormati data_scope (own vs all) via
//     AccountsListFilterFor — sama dgn modul Health.
//
// BL-98: section kini MAKS 2 KPI (Desa Berisiko · Adoption Rate) TANPA chart
// domain; KPI Engagement Jatuh Tempo & panel Onboarding dipindah ke CS Report.
//
// Setup/helper reuse dashboard_test.go (dashboardBody, dashboardKPIValue),
// accounts_test.go (setupAccounts, seedAccount, seedMember), health_score_test.go
// (seedHealthScore).

// TestDashboardCS_DomainVisibleByCapability: role dgn kapabilitas CS melihat
// heading "Customer Success" + 2 KPI ramping (Desa Berisiko, Adoption Rate).
// Role KUSTOM tanpa kapabilitas CS apa pun TAK melihat heading.
func TestDashboardCS_DomainVisibleByCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa CS Dom", &uid, nil, nil)
	env.seedHealthScore(t, acc.ID, 30, "Critical")

	kpis := []string{"Desa Berisiko", "Adoption Rate"}
	for _, role := range []string{"admin", "manager", "csm"} {
		t.Run(role+" melihat section Customer Success", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			if !strings.Contains(body, ">Customer Success</h2>") {
				t.Errorf("role %q harus melihat heading section Customer Success, body:\n%s", role, body)
			}
			for _, kpi := range kpis {
				if !strings.Contains(body, ">"+kpi+"</p>") {
					t.Errorf("role %q harus melihat KPI %q di section Customer Success", role, kpi)
				}
			}
			// BL-98: butir yang dipindah ke Report tak boleh muncul di Beranda.
			if strings.Contains(body, ">Engagement Jatuh Tempo (7 hari)</p>") {
				t.Errorf("role %q: KPI Engagement Jatuh Tempo sudah dipindah ke Report", role)
			}
			if strings.Contains(body, "chart-onboarding") {
				t.Errorf("role %q: panel Onboarding sudah dipindah ke Report (BL-98)", role)
			}
		})
	}

	t.Run("role kustom tanpa kapabilitas CS tak melihat section", func(t *testing.T) {
		env.loadBusinessRolesWith(t,
			authz.BusinessPerm{Role: "nocs", Obj: "crm:dashboard", Act: "read"},
		)
		req := accountsReq(http.MethodGet, "/w/test/", nil, "")
		rec := env.runAccountScope(uid, "member", "nocs", "all", req, env.h.WorkspaceHome)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), ">Customer Success</h2>") {
			t.Errorf("role tanpa kapabilitas CS TAK boleh melihat section Customer Success")
		}
	})
}

// TestDashboardCS_CustomRoleHealthOnly (BL-98): role KUSTOM dgn hanya
// crm:dashboard + crm:health (read) melihat section Customer Success berisi HANYA
// Desa Berisiko, TANPA Adoption Rate (crm:adoption) — bukti komposisi union
// parsial per-kapabilitas.
func TestDashboardCS_CustomRoleHealthOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "healthonly", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "healthonly", Obj: "crm:health", Act: "read"},
	)
	acc := env.seedAccount(t, "Desa Health Only", &uid, nil, nil)
	env.seedHealthScore(t, acc.ID, 30, "Critical")

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "healthonly", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">Customer Success</h2>") {
		t.Errorf("role kustom (crm:health) harus melihat section Customer Success, body:\n%s", body)
	}
	if !strings.Contains(body, ">Desa Berisiko</p>") {
		t.Error("role kustom (crm:health) harus melihat KPI Desa Berisiko")
	}
	if strings.Contains(body, ">Adoption Rate</p>") {
		t.Error("role kustom tanpa crm:adoption TAK boleh melihat KPI Adoption Rate")
	}
}

// TestDashboardCS_BerisikoScopedByOwnership (F3): KPI "Desa Berisiko" menghormati
// data_scope — sales (own) hanya menghitung desa binaannya; manager (all)
// menghitung lintas-owner. Filter = AccountsListFilterFor, sama dgn modul Health.
func TestDashboardCS_BerisikoScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "othercs@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa CS Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa CS Other", &other, nil, nil)
	env.seedHealthScore(t, accMine.ID, 30, "Critical")  // berisiko (kritis)
	env.seedHealthScore(t, accOther.ID, 30, "Critical") // berisiko (kritis)

	const label = "Desa Berisiko"
	own := env.dashboardBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales own-scope: %q = %q, want \"1\" (hanya desa binaannya)", label, got)
	}
	all := env.dashboardBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager all-scope: %q = %q, want \"2\" (lintas-owner)", label, got)
	}
}
