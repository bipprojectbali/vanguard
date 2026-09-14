package handler

import (
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// dashboard_support_charts_test.go — BL-144: 2 chart inline section Support
// (membalik BL-98 KHUSUS Beranda). Dua sumbu:
//
//   - Gate BEDA per chart (bukan satu union): "Volume Tiket per Bulan" digate
//     crm:tickets (sales PUNYA — read via TicketsListFilterFor, sama KPI Tiket
//     Terbuka), "SLA per Prioritas" digate crm:sla (sales TAK PUNYA — nol baris
//     di policy.csv, lihat authz/business_defaults.go). Jadi sales melihat SATU
//     dari dua chart, bukan keduanya/tak satupun — pola sama BL-143 (CS charts).
//   - F3 kepemilikan Volume Tiket per Bulan: TicketsListFilterFor via kolom akun
//     (account_owner/assigned_csm/backup_csm), sama dgn KPI Tiket Terbuka di
//     section yg sama — dibuktikan via jumlah bulan yg terlihat (own vs all).

// TestDashboardSupportCharts_RenderedByCapability: admin/manager/csm/support
// (crm:tickets + crm:sla) melihat kedua chart baru; sales (crm:tickets read
// TANPA crm:sla) hanya melihat Volume Tiket per Bulan; role kustom tanpa
// crm:tickets maupun crm:sla tak melihat satupun.
func TestDashboardSupportCharts_RenderedByCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Support Chart", &uid, nil, nil)
	env.seedTicketRow(t, acc.ID, "Tiket chart")

	for _, role := range []string{"admin", "manager", "csm", "support"} {
		t.Run(role+" melihat kedua chart Support", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			for _, id := range []string{"chart-support-volume", "chart-support-sla"} {
				if !strings.Contains(body, `id="`+id+`"`) {
					t.Errorf("role %q harus melihat %q, body:\n%s", role, id, body)
				}
			}
		})
	}

	t.Run("sales hanya melihat Volume Tiket per Bulan (tanpa crm:sla)", func(t *testing.T) {
		body := env.dashboardBody(t, uid, "owner", "sales")
		if !strings.Contains(body, `id="chart-support-volume"`) {
			t.Errorf("role sales (crm:tickets read) harus tetap melihat chart-support-volume, body:\n%s", body)
		}
		if strings.Contains(body, `id="chart-support-sla"`) {
			t.Errorf("role sales (tanpa crm:sla) TAK boleh melihat chart-support-sla, body:\n%s", body)
		}
	})

	t.Run("role kustom tanpa crm:tickets/crm:sla tak melihat chart Support", func(t *testing.T) {
		env.loadBusinessRolesWith(t,
			authz.BusinessPerm{Role: "nosupchart", Obj: "crm:dashboard", Act: "read"},
		)
		body := env.dashboardBody(t, uid, "member", "nosupchart")
		for _, id := range []string{"chart-support-volume", "chart-support-sla"} {
			if strings.Contains(body, id) {
				t.Errorf("role tanpa kapabilitas Support TAK boleh melihat %q", id)
			}
		}
	})
}

// TestDashboardSupportCharts_VolumeScopedByOwnership (F3): Volume Tiket per
// Bulan menghormati data_scope via TicketsListFilterFor — sales (own) hanya
// menghitung tiket desa binaannya; manager (all) menghitung lintas-owner.
// Dibuktikan via jumlah baris bulan (period label) yang tampak di chart JSON.
func TestDashboardSupportCharts_VolumeScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "othersupchart@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Sup Chart Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Sup Chart Other", &other, nil, nil)
	tkMine := env.seedTicketRow(t, accMine.ID, "Tiket chart mine")
	tkOther := env.seedTicketRow(t, accOther.ID, "Tiket chart other")
	env.backdateTicketCreated(t, tkMine.ID, ymd(2026, 1, 10))
	env.backdateTicketCreated(t, tkOther.ID, ymd(2026, 2, 10))

	own := env.dashboardBody(t, uid, "owner", "sales")
	if !strings.Contains(own, "chart-support-volume") {
		t.Fatalf("sales harus melihat chart-support-volume, body:\n%s", own)
	}
	if !strings.Contains(own, "Jan 2026") {
		t.Errorf("sales own-scope: chart Volume harus memuat Jan 2026 (desa binaannya), body:\n%s", own)
	}
	if strings.Contains(own, "Feb 2026") {
		t.Errorf("sales own-scope: chart Volume TAK boleh memuat Feb 2026 (desa orang lain), body:\n%s", own)
	}

	all := env.dashboardBody(t, uid, "owner", "manager")
	if !strings.Contains(all, "Jan 2026") || !strings.Contains(all, "Feb 2026") {
		t.Errorf("manager all-scope: chart Volume harus memuat Jan & Feb 2026 (lintas-owner), body:\n%s", all)
	}
}
