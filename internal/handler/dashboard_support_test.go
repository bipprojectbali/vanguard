package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// dashboard_support_test.go — section "Support" Beranda (Modul 1, BL-59d +
// BL-98). Dipisah dari dashboard_test.go (ukuran file). Tiga sumbu dijaga:
//
//   - Visibilitas per-kapabilitas: role dgn kapabilitas Support (admin/manager/
//     csm/support) melihat heading "Support" + KPI inti; role KUSTOM tanpa
//     kapabilitas Support apa pun TAK melihat heading.
//   - Komposisi union parsial (BL-98): role KUSTOM crm:tickets-only melihat butir
//     Tiket Terbuka saja, tanpa Kepatuhan SLA (crm:sla).
//   - F3 kepemilikan: "Tiket Terbuka" menghormati data_scope (own vs all) via
//     TicketsListFilterFor — sama dgn daftar /tickets.
//
// BL-98: section kini MAKS 2 KPI (Tiket Terbuka · Kepatuhan SLA) TANPA chart
// domain; KPI Terlambat/Langgar & Rata Waktu, chart Tiket-per-Prioritas & Beban-
// Agen dipindah ke Support Report.
//
// Setup/helper reuse dashboard_test.go (dashboardBody, dashboardKPIValue),
// accounts_test.go (setupAccounts, seedAccount, seedMember), tickets_test.go
// (seedTicketRow).

// TestDashboardSupport_DomainVisibleByCapability: role dgn kapabilitas Support
// melihat heading "Support" + 2 KPI ramping (Tiket Terbuka, Kepatuhan SLA).
// Role KUSTOM tanpa kapabilitas Support apa pun TAK melihat heading.
func TestDashboardSupport_DomainVisibleByCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Support Dom", &uid, nil, nil)
	env.seedTicketRow(t, acc.ID, "Tiket dashboard")

	kpis := []string{"Tiket Terbuka", "Kepatuhan SLA"}
	// admin/manager/csm/support semua punya crm:tickets + crm:sla read.
	for _, role := range []string{"admin", "manager", "csm", "support"} {
		t.Run(role+" melihat section Support", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			if !strings.Contains(body, ">Support</h2>") {
				t.Errorf("role %q harus melihat heading section Support, body:\n%s", role, body)
			}
			for _, kpi := range kpis {
				if !strings.Contains(body, ">"+kpi+"</p>") {
					t.Errorf("role %q harus melihat KPI %q di section Support", role, kpi)
				}
			}
			// BL-98: butir yang dipindah ke Report tak boleh muncul di Beranda.
			for _, dropped := range []string{">Terlambat / Langgar SLA</p>", ">Rata Waktu Penyelesaian</p>"} {
				if strings.Contains(body, dropped) {
					t.Errorf("role %q: KPI %q sudah dipindah ke Report", role, dropped)
				}
			}
			for _, chart := range []string{"chart-tickets-priority", "chart-agent-workload"} {
				if strings.Contains(body, chart) {
					t.Errorf("role %q: panel %q sudah dipindah ke Report (BL-98)", role, chart)
				}
			}
		})
	}

	t.Run("role kustom tanpa kapabilitas Support tak melihat section", func(t *testing.T) {
		env.loadBusinessRolesWith(t,
			authz.BusinessPerm{Role: "nosup", Obj: "crm:dashboard", Act: "read"},
		)
		req := accountsReq(http.MethodGet, "/w/test/", nil, "")
		rec := env.runAccountScope(uid, "member", "nosup", "all", req, env.h.WorkspaceHome)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), ">Support</h2>") {
			t.Errorf("role tanpa kapabilitas Support TAK boleh melihat section Support")
		}
	})
}

// TestDashboardSupport_CustomRoleTicketsOnly (BL-98): role KUSTOM dgn hanya
// crm:dashboard + crm:tickets (read) melihat section Support berisi HANYA Tiket
// Terbuka, TANPA Kepatuhan SLA (crm:sla) — bukti komposisi union parsial.
func TestDashboardSupport_CustomRoleTicketsOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "ticketsonly", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "ticketsonly", Obj: "crm:tickets", Act: "read"},
	)
	acc := env.seedAccount(t, "Desa Tickets Only", &uid, nil, nil)
	env.seedTicketRow(t, acc.ID, "Tiket only")

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "ticketsonly", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">Support</h2>") {
		t.Errorf("role kustom (crm:tickets) harus melihat section Support, body:\n%s", body)
	}
	if !strings.Contains(body, ">Tiket Terbuka</p>") {
		t.Error("role kustom (crm:tickets) harus melihat KPI Tiket Terbuka")
	}
	if strings.Contains(body, ">Kepatuhan SLA</p>") {
		t.Error("role kustom tanpa crm:sla TAK boleh melihat KPI Kepatuhan SLA")
	}
}

// TestDashboardSupport_OpenScopedByOwnership (F3): KPI "Tiket Terbuka" menghormati
// data_scope — sales (own) hanya menghitung tiket desa binaannya; manager (all)
// menghitung lintas-owner. Filter = TicketsListFilterFor (ownership via kolom
// accounts account_owner/assigned_csm/backup_csm), sama dgn daftar /tickets.
func TestDashboardSupport_OpenScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "othersup@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Sup Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Sup Other", &other, nil, nil)
	env.seedTicketRow(t, accMine.ID, "Tiket mine")   // terbuka (status baru)
	env.seedTicketRow(t, accOther.ID, "Tiket other") // terbuka (status baru)

	const label = "Tiket Terbuka"
	own := env.dashboardBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales own-scope: %q = %q, want \"1\" (hanya desa binaannya)", label, got)
	}
	all := env.dashboardBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager all-scope: %q = %q, want \"2\" (lintas-owner)", label, got)
	}
}
