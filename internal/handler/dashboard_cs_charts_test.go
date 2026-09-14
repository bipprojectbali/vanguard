package handler

import (
	"strings"
	"testing"
)

// dashboard_cs_charts_test.go — BL-143: 2 chart inline section Customer Success
// (membalik BL-98 KHUSUS Beranda). Dua sumbu:
//
//   - Gate BEDA per chart (bukan satu union): "Progres Onboarding" digate
//     crm:journey (sales PUNYA — read eksplisit di policy.csv), "Engagement per
//     CSM" digate crm:engagements (sales TAK PUNYA — nol baris di policy.csv).
//     Jadi sales melihat SATU dari dua chart, bukan keduanya/tak satupun —
//     berbeda dari pola union tunggal BL-141/BL-142.
//   - F3 kepemilikan Engagement per CSM: EngagementsListFilterFor via kolom
//     akun (account_owner/assigned_csm/backup_csm), BUKAN AccountsListFilterFor
//     spt KPI Desa Berisiko di section yg sama. Dibuktikan lewat email CSM
//     TEST-ONLY unik (Name NULL by default di seedMember → ownerDisplay jatuh
//     ke email) sbg kategori bar — bukan email viewer ("test@local", yg SELALU
//     tampil di chip user AppShell terlepas data chart, jadi tak valid dipakai
//     sbg bukti).

// TestDashboardCSCharts_RenderedByCapability: admin/manager/csm (crm:journey +
// crm:engagements) melihat kedua chart baru; sales (crm:journey read TANPA
// crm:engagements) hanya melihat Progres Onboarding; support (tanpa
// kapabilitas CS apa pun) tak melihat satupun.
func TestDashboardCSCharts_RenderedByCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa CS Chart", &uid, nil, nil)
	env.seedEngagement(t, acc.ID, "check_in", "done", &uid)

	for _, role := range []string{"admin", "manager", "csm"} {
		t.Run(role+" melihat kedua chart Customer Success", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			for _, id := range []string{"chart-cs-onboarding", "chart-cs-engagement"} {
				if !strings.Contains(body, `id="`+id+`"`) {
					t.Errorf("role %q harus melihat %q, body:\n%s", role, id, body)
				}
			}
		})
	}

	t.Run("sales hanya melihat Progres Onboarding (tanpa crm:engagements)", func(t *testing.T) {
		body := env.dashboardBody(t, uid, "owner", "sales")
		if !strings.Contains(body, `id="chart-cs-onboarding"`) {
			t.Errorf("role sales (crm:journey read) harus tetap melihat chart-cs-onboarding, body:\n%s", body)
		}
		if strings.Contains(body, `id="chart-cs-engagement"`) {
			t.Errorf("role sales (tanpa crm:engagements) TAK boleh melihat chart-cs-engagement, body:\n%s", body)
		}
	})

	t.Run("support tak melihat chart Customer Success sama sekali", func(t *testing.T) {
		body := env.dashboardBody(t, uid, "owner", "support")
		for _, id := range []string{"chart-cs-onboarding", "chart-cs-engagement"} {
			if strings.Contains(body, id) {
				t.Errorf("support (tanpa kapabilitas Customer Success) TAK boleh melihat %q", id)
			}
		}
	})
}

// TestDashboardCSCharts_EngagementScopedByOwnership (F3): Engagement per CSM
// menghormati data_scope via EngagementsListFilterFor (kolom akun, bukan kolom
// engagement) — csm own-scope hanya melihat CSM di desa yg ditugaskan padanya
// (assigned_csm = viewer); manager all-scope melihat lintas-desa.
func TestDashboardCSCharts_EngagementScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherengboss@local", "member", 0).ID
	csmMine := env.seedMember(t, "csmchartmine@local", "member", 0).ID
	csmOther := env.seedMember(t, "csmchartother@local", "member", 0).ID

	accMine := env.seedAccount(t, "Desa Eng Mine", nil, &uid, nil)
	accOther := env.seedAccount(t, "Desa Eng Other", nil, &other, nil)
	env.seedEngagement(t, accMine.ID, "check_in", "done", &csmMine)
	env.seedEngagement(t, accOther.ID, "check_in", "done", &csmOther)

	own := env.dashboardBody(t, uid, "owner", "csm")
	if !strings.Contains(own, "csmchartmine@local") {
		t.Errorf("csm own-scope harus melihat CSM di desa binaannya (csmchartmine@local), body:\n%s", own)
	}
	if strings.Contains(own, "csmchartother@local") {
		t.Errorf("csm own-scope TAK boleh melihat CSM di desa orang lain (csmchartother@local), body:\n%s", own)
	}

	all := env.dashboardBody(t, uid, "owner", "manager")
	if !strings.Contains(all, "csmchartmine@local") || !strings.Contains(all, "csmchartother@local") {
		t.Errorf("manager all-scope harus melihat kedua CSM, body:\n%s", all)
	}
}
