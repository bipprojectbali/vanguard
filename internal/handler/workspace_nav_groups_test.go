package handler

import (
	"strings"
	"testing"

	"go_starter/internal/ui"
)

// TestWorkspaceNav_CSGroup: Customer Success = grup bersarang, disembunyikan
// total bila tak ada izin ke satu pun anak (sama pola dgn Settings).
// "Health Score" (6.1), "Customer Journey" (6.2),
// "Training Schedule" (6.2.1.2, BL-168), "Success Plans" (6.3), "Engagements"
// (6.5), "Renewal Management" (6.6), "SLA Management" (A1),
// "Playbooks" (A2), "Knowledge Base" (A3) & "Tickets / Cases" (B2) berbackend —
// masing-masing enabled mengikuti izinnya. Tidak ada anak yang selalu disabled.
func TestWorkspaceNav_CSGroup(t *testing.T) {
	// canSLA=true & canPlaybooks=true & canKB=true & canTickets=true &
	// canHealthScore=true & canSuccessPlans=true & canEngagements=true &
	// canRenewals=true → grup tampil, semua berbackend enabled dengan href benar.
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, true, true, true, false, true, true, true, true, true, false, false, false, false, true, true)
	grp, ok := findItem(nav, "Customer Success")
	if !ok {
		t.Fatal("grup Customer Success harus tampil saat ada izin ke minimal satu anak")
	}
	if grp.Href != "" {
		t.Errorf("header grup Customer Success tak boleh jadi link, got Href %q", grp.Href)
	}
	sla, ok := findItem(grp.Children, "SLA Management")
	if !ok {
		t.Fatal("grup Customer Success kurang anak SLA Management")
	}
	if sla.Disabled || sla.Href != "/w/acme/sla-policies" {
		t.Errorf("SLA Management harus enabled→/w/acme/sla-policies, got disabled=%v href=%q", sla.Disabled, sla.Href)
	}
	playbooks, ok := findItem(grp.Children, "Playbooks")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Playbooks")
	}
	if playbooks.Disabled || playbooks.Href != "/w/acme/playbooks" {
		t.Errorf("Playbooks harus enabled→/w/acme/playbooks, got disabled=%v href=%q", playbooks.Disabled, playbooks.Href)
	}
	kb, ok := findItem(grp.Children, "Knowledge Base")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Knowledge Base")
	}
	if kb.Disabled || kb.Href != "/w/acme/kb-articles" {
		t.Errorf("Knowledge Base harus enabled→/w/acme/kb-articles, got disabled=%v href=%q", kb.Disabled, kb.Href)
	}
	tickets, ok := findItem(grp.Children, "Tickets / Cases")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Tickets / Cases")
	}
	if tickets.Disabled || tickets.Href != "/w/acme/tickets" {
		t.Errorf("Tickets / Cases harus enabled→/w/acme/tickets, got disabled=%v href=%q", tickets.Disabled, tickets.Href)
	}
	healthScore, ok := findItem(grp.Children, "Health Score")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Health Score")
	}
	if healthScore.Disabled || healthScore.Href != "/w/acme/health-scores" {
		t.Errorf("Health Score harus enabled→/w/acme/health-scores, got disabled=%v href=%q", healthScore.Disabled, healthScore.Href)
	}
	successPlans, ok := findItem(grp.Children, "Success Plans")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Success Plans")
	}
	if successPlans.Disabled || successPlans.Href != "/w/acme/success-plans" {
		t.Errorf("Success Plans harus enabled→/w/acme/success-plans, got disabled=%v href=%q", successPlans.Disabled, successPlans.Href)
	}
	engagements, ok := findItem(grp.Children, "Engagements")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Engagements")
	}
	if engagements.Disabled || engagements.Href != "/w/acme/engagements" {
		t.Errorf("Engagements harus enabled→/w/acme/engagements, got disabled=%v href=%q", engagements.Disabled, engagements.Href)
	}
	renewal, ok := findItem(grp.Children, "Renewal Management")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Renewal Management")
	}
	if renewal.Disabled || renewal.Href != "/w/acme/renewal-management" {
		t.Errorf("Renewal Management harus enabled→/w/acme/renewal-management, got disabled=%v href=%q", renewal.Disabled, renewal.Href)
	}
	// BL-78 (level A): Implementation Tracker (6.2.1.1) TIDAK punya item nav —
	// route/handler/tabel tetap hidup, cuma disembunyikan dari menu.
	if _, ok := findItem(grp.Children, "Implementation Tracker"); ok {
		t.Error("Implementation Tracker harus ABSEN dari menu CS (BL-78 level A)")
	}
	// Training Schedule (6.2.1.2) DIKEMBALIKAN ke menu (BL-168, 17 Sep) — gate
	// SEJAK BL-169 objek Casbin sendiri (crm:adoption), TERPISAH dari canJourney.
	trainings, ok := findItem(grp.Children, "Training Schedule")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Training Schedule (BL-168)")
	}
	if trainings.Disabled || trainings.Href != "/w/acme/trainings" {
		t.Errorf("Training Schedule harus enabled→/w/acme/trainings, got disabled=%v href=%q", trainings.Disabled, trainings.Href)
	}
	journey, ok := findItem(grp.Children, "Customer Journey")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Customer Journey")
	}
	if journey.Disabled || journey.Href != "/w/acme/journey" {
		t.Errorf("Customer Journey harus enabled→/w/acme/journey, got disabled=%v href=%q", journey.Disabled, journey.Href)
	}

	// Tanpa izin ke satu pun anak → grup disembunyikan total (sama pola dgn
	// Settings).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	if _, ok := findItem(navNone, "Customer Success"); ok {
		t.Error("grup Customer Success harus disembunyikan total saat tak ada izin ke satu pun anaknya")
	}
}

// TestWorkspaceNav_CSGroupGatingPerIzin: anak tanpa izinnya sendiri TAK
// ditampilkan sama sekali (bukan disabled teredam) — hanya canSLA=true di
// sini, jadi hanya SLA Management yang muncul di grup CS.
func TestWorkspaceNav_CSGroupGatingPerIzin(t *testing.T) {
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Customer Success")
	if !ok {
		t.Fatal("canSLA saja tetap memunculkan grup Customer Success")
	}
	if _, ok := findItem(grp.Children, "SLA Management"); !ok {
		t.Error("canSLA harus memunculkan anak SLA Management")
	}
	for _, forbidden := range []string{"Health Score", "Customer Journey", "Training Schedule",
		"Success Plans", "Engagements", "Renewal Management", "Playbooks", "Tickets / Cases", "Knowledge Base"} {
		if _, ok := findItem(grp.Children, forbidden); ok {
			t.Errorf("tanpa izinnya, anak %q tak boleh muncul", forbidden)
		}
	}
}

// TestWorkspaceNav_ReportsGroup: Reports = grup bersarang, disembunyikan
// total bila tak ada izin ke satu pun anak (sama pola dgn Settings). Sales
// Reports (8.1), Customer Success Reports (8.2), Support Reports (8.3),
// Subscription Reports (8.4) berbackend, masing-masing objek Casbin sendiri
// sejak BL-169. Custom Reports (8.5, Report Builder) ditunda v1.1
// (skema.md §10) — item nav-nya TIDAK ADA sama sekali, bukan disabled.
func TestWorkspaceNav_ReportsGroup(t *testing.T) {
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, false, false)
	grp, ok := findItem(nav, "Reports")
	if !ok {
		t.Fatal("grup Reports harus tampil saat ada izin ke minimal satu anak")
	}
	if grp.Href != "" {
		t.Errorf("header grup Reports tak boleh jadi link, got Href %q", grp.Href)
	}
	wantEnabled := map[string]string{
		"Sales Reports":            "/w/acme/reports/sales",
		"Customer Success Reports": "/w/acme/reports/customer-success",
		"Support Reports":          "/w/acme/reports/support",
		"Subscription Reports":     "/w/acme/reports/subscriptions",
	}
	for label, href := range wantEnabled {
		ch, ok := findItem(grp.Children, label)
		if !ok {
			t.Errorf("grup Reports kurang anak %q", label)
			continue
		}
		if ch.Disabled || ch.Href != href {
			t.Errorf("anak %q harus enabled→%q, got disabled=%v href=%q", label, href, ch.Disabled, ch.Href)
		}
	}
	if _, ok := findItem(grp.Children, "Custom Reports"); ok {
		t.Error("Custom Reports (8.5) ditunda — item nav-nya harus dihapus total, bukan disabled")
	}

	// Tanpa izin ke satu pun anak → grup disembunyikan total (sama pola dgn
	// Settings).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	if _, ok := findItem(navNone, "Reports"); ok {
		t.Error("grup Reports harus disembunyikan total saat tak ada izin ke satu pun anaknya")
	}
}

// TestWorkspaceNav_ReportsGroupGatingPerIzin: anak tanpa izinnya sendiri TAK
// ditampilkan sama sekali — hanya canReportsSales=true di sini (BL-169: tiap
// domain Report objek Casbin sendiri).
func TestWorkspaceNav_ReportsGroupGatingPerIzin(t *testing.T) {
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false)
	grp, ok := findItem(nav, "Reports")
	if !ok {
		t.Fatal("canReportsSales saja tetap memunculkan grup Reports")
	}
	if _, ok := findItem(grp.Children, "Sales Reports"); !ok {
		t.Error("canReportsSales harus memunculkan anak Sales Reports")
	}
	for _, forbidden := range []string{"Customer Success Reports", "Support Reports", "Subscription Reports"} {
		if _, ok := findItem(grp.Children, forbidden); ok {
			t.Errorf("tanpa izinnya, anak %q tak boleh muncul", forbidden)
		}
	}
}

// TestWorkspaceNav_SalesGroupGatingPerIzin: anak tanpa izinnya sendiri TAK
// ditampilkan sama sekali — hanya canLeads=true di sini (Quotes ikut canDeals,
// jadi ikut absen).
func TestWorkspaceNav_SalesGroupGatingPerIzin(t *testing.T) {
	nav := workspaceNav("acme", false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Sales")
	if !ok {
		t.Fatal("canLeads saja tetap memunculkan grup Sales")
	}
	if _, ok := findItem(grp.Children, "Leads"); !ok {
		t.Error("canLeads harus memunculkan anak Leads")
	}
	for _, forbidden := range []string{"Deals", "Quotes", "Sales Activities"} {
		if _, ok := findItem(grp.Children, forbidden); ok {
			t.Errorf("tanpa izinnya, anak %q tak boleh muncul", forbidden)
		}
	}
}

// TestWorkspaceNav_SubscriptionsGroupGatingPerIzin: anak tanpa izinnya sendiri
// TAK ditampilkan sama sekali — hanya canPlans=true di sini (Subscription
// Lists/Renewals/Churn ikut canSubs, jadi ikut absen).
func TestWorkspaceNav_SubscriptionsGroupGatingPerIzin(t *testing.T) {
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Subscriptions")
	if !ok {
		t.Fatal("canPlans saja tetap memunculkan grup Subscriptions")
	}
	if _, ok := findItem(grp.Children, "Plans & Pricing"); !ok {
		t.Error("canPlans harus memunculkan anak Plans & Pricing")
	}
	for _, forbidden := range []string{"Subscription Lists", "Renewals", "Churn / Cancellations"} {
		if _, ok := findItem(grp.Children, forbidden); ok {
			t.Errorf("tanpa izinnya, anak %q tak boleh muncul", forbidden)
		}
	}
}

// TestWorkspaceNav_SettingsGroupChildren: Settings = grup bersarang; anak enabled
// mengikuti izin masing-masing (sumber sama dengan gerbang halaman). Semua izin →
// ketiga anak hadir dengan href benar. Placeholder "Automation / Workflow" &
// "Integrations" SENGAJA disembunyikan (BL-55) — fiturnya belum ada, kurangi menu
// mati. "Workspace Settings" (/settings) SENGAJA tak lagi di sini — identitas &
// siklus hidup workspace pindah ke /dev sebagai wewenang platform (BL-53).
func TestWorkspaceNav_SettingsGroupChildren(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
	grp, ok := findItem(nav, "Settings")
	if !ok {
		t.Fatal("grup Settings harus ada saat user punya izin pengaturan")
	}
	if grp.Href != "" {
		t.Errorf("header grup Settings tak boleh jadi link, got Href %q", grp.Href)
	}
	if _, ok := findItem(grp.Children, "Workspace Settings"); ok {
		t.Error("Workspace Settings harus tidak ada — pindah ke /dev (BL-53)")
	}
	want := map[string]string{
		"User Management":     "/w/acme/members",
		"Roles & Permissions": "/w/acme/roles",
		"Customization":       "/w/acme/codes",
	}
	for label, href := range want {
		ch, ok := findItem(grp.Children, label)
		if !ok {
			t.Errorf("grup Settings kurang anak %q", label)
			continue
		}
		if ch.Href != href {
			t.Errorf("anak %q href %q, want %q", label, ch.Href, href)
		}
		if ch.Disabled {
			t.Errorf("anak %q seharusnya enabled", label)
		}
	}
	for _, label := range []string{"Automation / Workflow", "Integrations"} {
		if _, ok := findItem(grp.Children, label); ok {
			t.Errorf("placeholder %q harus tidak ada — disembunyikan (BL-55)", label)
		}
	}
}

// TestWorkspaceNav_SettingsGatingPerIzin: tiap anak enabled muncul HANYA jika
// izinnya diberikan — menu tak menawarkan pintu yang lalu ditolak 403.
func TestWorkspaceNav_SettingsGatingPerIzin(t *testing.T) {
	// Hanya canRoles → grup ada, tapi cuma Roles yang enabled.
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Settings")
	if !ok {
		t.Fatal("canRoles saja tetap memunculkan grup Settings")
	}
	if _, ok := findItem(grp.Children, "Roles & Permissions"); !ok {
		t.Error("canRoles harus memunculkan anak Roles & Permissions")
	}
	for _, forbidden := range []string{"Workspace Settings", "User Management", "Customization"} {
		if _, ok := findItem(grp.Children, forbidden); ok {
			t.Errorf("tanpa izinnya, anak %q tak boleh muncul", forbidden)
		}
	}
}

// TestWorkspaceNav_NoSettingsGroupWithoutPerms: member tanpa izin pengaturan tak
// melihat grup Settings sama sekali (grup kosong = tak menawarkan apa pun).
func TestWorkspaceNav_NoSettingsGroupWithoutPerms(t *testing.T) {
	nav := workspaceNav("acme", false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	if _, ok := findItem(nav, "Settings"); ok {
		t.Error("tanpa izin pengaturan, grup Settings harus disembunyikan")
	}
	// Grup CRM (Sales dkk) ikut disembunyikan di sini — argumen izinnya juga
	// semua false, bukan cuma pengaturan (lihat TestWorkspaceNav_SalesGroup
	// untuk assert khusus perilaku hide-grup Sales).
	if _, ok := findItem(nav, "Sales"); ok {
		t.Error("grup Sales harus ikut disembunyikan (tak ada izin ke satu pun anaknya)")
	}
}

// TestWorkspaceNav_HrefSlugBenar: semua anchor enabled menunjuk slug yang benar
// (regresi wsPath — menu bergantung slug sejak 0004).
func TestWorkspaceNav_HrefSlugBenar(t *testing.T) {
	nav := workspaceNav("beta", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
	var check func(items []ui.NavItem)
	check = func(items []ui.NavItem) {
		for _, it := range items {
			if len(it.Children) > 0 {
				check(it.Children)
				continue
			}
			if it.Disabled || it.Href == "" {
				continue
			}
			if !strings.HasPrefix(it.Href, "/w/beta") {
				t.Errorf("href %q (%s) tak berprefiks /w/beta", it.Href, it.Label)
			}
		}
	}
	check(nav)
}
