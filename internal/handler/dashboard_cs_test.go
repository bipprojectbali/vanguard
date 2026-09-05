package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// dashboard_cs_test.go — section "Customer Success" Beranda (Modul 1, BL-59c).
// Dipisah dari dashboard_test.go (ukuran file). Tiga sumbu dijaga:
//
//   - Visibilitas per-kapabilitas: role dgn kapabilitas CS penuh (admin/manager/
//     csm) melihat heading "Customer Success" + KPI/panel inti; role KUSTOM tanpa
//     kapabilitas CS apa pun TAK melihat heading (tak berdiri kosong).
//   - Komposisi union parsial: role KUSTOM crm:health-only melihat butir Health
//     saja (Desa Berisiko), tanpa Adoption/Engagement/Onboarding.
//   - F3 kepemilikan: "Desa Berisiko" menghormati data_scope (own vs all) via
//     AccountsListFilterFor — sama dgn modul Health.
//
// Setup/helper reuse dashboard_test.go (dashboardBody, dashboardKPIValue),
// accounts_test.go (setupAccounts, seedAccount, seedMember), health_score_test.go
// (seedHealthScore), reports_cs_test.go (seedEngagement).

// TestDashboardCS_DomainVisibleByCapability: role dgn kapabilitas CS penuh melihat
// heading "Customer Success" + KPI (Desa Berisiko, Adoption Rate, Engagement
// Jatuh Tempo) + panel Onboarding. Role KUSTOM tanpa kapabilitas CS apa pun TAK
// melihat heading.
func TestDashboardCS_DomainVisibleByCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa CS Dom", &uid, nil, nil)
	env.seedHealthScore(t, acc.ID, 30, "Critical")
	env.seedEngagement(t, acc.ID, "touch_point", "planned", &uid)

	kpis := []string{"Desa Berisiko", "Adoption Rate", "Engagement Jatuh Tempo (7 hari)"}
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
			if !strings.Contains(body, "chart-onboarding") {
				t.Errorf("role %q harus melihat panel Progres Onboarding (crm:journey)", role)
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

// TestDashboardCS_CustomRoleHealthOnly: role KUSTOM dgn hanya crm:dashboard +
// crm:health (read) melihat section Customer Success berisi HANYA butir Health
// (Desa Berisiko), TANPA Adoption Rate (crm:adoption), Engagement Jatuh Tempo
// (crm:engagements), & panel Onboarding (crm:journey) — bukti komposisi union
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
	if strings.Contains(body, ">Engagement Jatuh Tempo (7 hari)</p>") {
		t.Error("role kustom tanpa crm:engagements TAK boleh melihat KPI Engagement Jatuh Tempo")
	}
	if strings.Contains(body, "chart-onboarding") {
		t.Error("role kustom tanpa crm:journey TAK boleh melihat panel Progres Onboarding")
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
