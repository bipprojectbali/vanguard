package handler

import (
	"strings"
	"testing"

	"go_starter/internal/ui"
)

// TestWorkspaceNav_CSGroup: Customer Success = grup bersarang yang SELALU tampil
// (peta jalan Modul 6). "Health Score" (6.1), "Implementation Tracker"
// (6.2.1.1), "Training Schedule" (6.2.1.2), "Success Plans" (6.3),
// "Engagements" (6.5), "Renewal Management" (6.6), "SLA Management" (A1),
// "Playbooks" (A2), "Knowledge Base" (A3) & "Tickets / Cases" (B2) berbackend —
// masing-masing enabled mengikuti izinnya. Tidak ada anak yang selalu disabled.
func TestWorkspaceNav_CSGroup(t *testing.T) {
	placeholders := []string{}

	// canSLA=true & canPlaybooks=true & canKB=true & canTickets=true &
	// canHealthScore=true & canSuccessPlans=true & canEngagements=true &
	// canRenewals=true → grup tampil, semua berbackend enabled dengan href benar.
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, true, true, true, false, true, true, true, true, true, false, true, true, true)
	grp, ok := findItem(nav, "Customer Success")
	if !ok {
		t.Fatal("grup Customer Success harus selalu tampil (peta jalan)")
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
	implTasks, ok := findItem(grp.Children, "Implementation Tracker")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Implementation Tracker")
	}
	if implTasks.Disabled || implTasks.Href != "/w/acme/impl-tasks" {
		t.Errorf("Implementation Tracker harus enabled→/w/acme/impl-tasks, got disabled=%v href=%q", implTasks.Disabled, implTasks.Href)
	}
	trainings, ok := findItem(grp.Children, "Training Schedule")
	if !ok {
		t.Fatal("grup Customer Success kurang anak Training Schedule")
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
	for _, label := range placeholders {
		ch, ok := findItem(grp.Children, label)
		if !ok {
			t.Errorf("grup Customer Success kurang placeholder %q", label)
			continue
		}
		if !ch.Disabled || ch.Href != "" {
			t.Errorf("placeholder %q harus disabled tanpa href, got disabled=%v href=%q", label, ch.Disabled, ch.Href)
		}
	}

	// canSLA=false & canPlaybooks=false & canKB=false & canTickets=false → grup
	// tetap tampil, semuanya disabled tanpa href (menu tak menawarkan pintu
	// yang lalu ditolak 403).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grpNone, ok := findItem(navNone, "Customer Success")
	if !ok {
		t.Fatal("grup Customer Success tetap tampil walau tanpa izin")
	}
	slaNone, ok := findItem(grpNone.Children, "SLA Management")
	if !ok {
		t.Fatal("anak SLA Management harus tetap tampil (disabled)")
	}
	if !slaNone.Disabled || slaNone.Href != "" {
		t.Errorf("tanpa izin, SLA Management harus disabled tanpa href, got disabled=%v href=%q", slaNone.Disabled, slaNone.Href)
	}
	playbooksNone, ok := findItem(grpNone.Children, "Playbooks")
	if !ok {
		t.Fatal("anak Playbooks harus tetap tampil (disabled)")
	}
	if !playbooksNone.Disabled || playbooksNone.Href != "" {
		t.Errorf("tanpa izin, Playbooks harus disabled tanpa href, got disabled=%v href=%q", playbooksNone.Disabled, playbooksNone.Href)
	}
	kbNone, ok := findItem(grpNone.Children, "Knowledge Base")
	if !ok {
		t.Fatal("anak Knowledge Base harus tetap tampil (disabled)")
	}
	if !kbNone.Disabled || kbNone.Href != "" {
		t.Errorf("tanpa izin, Knowledge Base harus disabled tanpa href, got disabled=%v href=%q", kbNone.Disabled, kbNone.Href)
	}
	ticketsNone, ok := findItem(grpNone.Children, "Tickets / Cases")
	if !ok {
		t.Fatal("anak Tickets / Cases harus tetap tampil (disabled)")
	}
	if !ticketsNone.Disabled || ticketsNone.Href != "" {
		t.Errorf("tanpa izin, Tickets / Cases harus disabled tanpa href, got disabled=%v href=%q", ticketsNone.Disabled, ticketsNone.Href)
	}
	healthScoreNone, ok := findItem(grpNone.Children, "Health Score")
	if !ok {
		t.Fatal("anak Health Score harus tetap tampil (disabled)")
	}
	if !healthScoreNone.Disabled || healthScoreNone.Href != "" {
		t.Errorf("tanpa izin, Health Score harus disabled tanpa href, got disabled=%v href=%q", healthScoreNone.Disabled, healthScoreNone.Href)
	}
	successPlansNone, ok := findItem(grpNone.Children, "Success Plans")
	if !ok {
		t.Fatal("anak Success Plans harus tetap tampil (disabled)")
	}
	if !successPlansNone.Disabled || successPlansNone.Href != "" {
		t.Errorf("tanpa izin, Success Plans harus disabled tanpa href, got disabled=%v href=%q", successPlansNone.Disabled, successPlansNone.Href)
	}
	engagementsNone, ok := findItem(grpNone.Children, "Engagements")
	if !ok {
		t.Fatal("anak Engagements harus tetap tampil (disabled)")
	}
	if !engagementsNone.Disabled || engagementsNone.Href != "" {
		t.Errorf("tanpa izin, Engagements harus disabled tanpa href, got disabled=%v href=%q", engagementsNone.Disabled, engagementsNone.Href)
	}
	renewalNone, ok := findItem(grpNone.Children, "Renewal Management")
	if !ok {
		t.Fatal("anak Renewal Management harus tetap tampil (disabled)")
	}
	if !renewalNone.Disabled || renewalNone.Href != "" {
		t.Errorf("tanpa izin, Renewal Management harus disabled tanpa href, got disabled=%v href=%q", renewalNone.Disabled, renewalNone.Href)
	}
	implTasksNone, ok := findItem(grpNone.Children, "Implementation Tracker")
	if !ok {
		t.Fatal("anak Implementation Tracker harus tetap tampil (disabled)")
	}
	if !implTasksNone.Disabled || implTasksNone.Href != "" {
		t.Errorf("tanpa izin, Implementation Tracker harus disabled tanpa href, got disabled=%v href=%q", implTasksNone.Disabled, implTasksNone.Href)
	}
	trainingsNone, ok := findItem(grpNone.Children, "Training Schedule")
	if !ok {
		t.Fatal("anak Training Schedule harus tetap tampil (disabled)")
	}
	if !trainingsNone.Disabled || trainingsNone.Href != "" {
		t.Errorf("tanpa izin, Training Schedule harus disabled tanpa href, got disabled=%v href=%q", trainingsNone.Disabled, trainingsNone.Href)
	}
	journeyNone, ok := findItem(grpNone.Children, "Customer Journey")
	if !ok {
		t.Fatal("anak Customer Journey harus tetap tampil (disabled)")
	}
	if !journeyNone.Disabled || journeyNone.Href != "" {
		t.Errorf("tanpa izin, Customer Journey harus disabled tanpa href, got disabled=%v href=%q", journeyNone.Disabled, journeyNone.Href)
	}
}

// TestWorkspaceNav_ReportsGroup: Reports = grup bersarang yang SELALU tampil
// (peta jalan wireframe 8). Sales Reports (8.1), Customer Success Reports
// (8.2), Support Reports (8.3), Subscription Reports (8.4) berbackend,
// enabled mengikuti SATU izin canReports (objek crm:reports read). Custom
// Reports (8.5, Report Builder) ditunda v1.1 (skema.md §10) — item nav-nya
// TIDAK ADA sama sekali, bukan disabled.
func TestWorkspaceNav_ReportsGroup(t *testing.T) {
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false)
	grp, ok := findItem(nav, "Reports")
	if !ok {
		t.Fatal("grup Reports harus selalu tampil (peta jalan)")
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

	// canReports=false → grup tetap tampil, semua anak disabled tanpa href
	// (menu tak menawarkan pintu yang lalu ditolak 403).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grpNone, ok := findItem(navNone, "Reports")
	if !ok {
		t.Fatal("grup Reports tetap tampil walau tanpa izin")
	}
	for _, label := range []string{"Sales Reports", "Customer Success Reports", "Support Reports", "Subscription Reports"} {
		ch, ok := findItem(grpNone.Children, label)
		if !ok {
			t.Errorf("anak %q harus tetap tampil (disabled)", label)
			continue
		}
		if !ch.Disabled || ch.Href != "" {
			t.Errorf("tanpa izin, anak %q harus disabled tanpa href, got disabled=%v href=%q", label, ch.Disabled, ch.Href)
		}
	}
	if _, ok := findItem(grpNone.Children, "Custom Reports"); ok {
		t.Error("Custom Reports (8.5) harus tetap tidak ada walau tanpa izin")
	}
}

// TestWorkspaceNav_SettingsGroupChildren: Settings = grup bersarang; anak enabled
// mengikuti izin masing-masing (sumber sama dengan gerbang halaman). Semua izin →
// ketiga anak hadir dengan href benar. Placeholder "Automation / Workflow" &
// "Integrations" SENGAJA disembunyikan (BL-55) — fiturnya belum ada, kurangi menu
// mati. "Workspace Settings" (/settings) SENGAJA tak lagi di sini — identitas &
// siklus hidup workspace pindah ke /dev sebagai wewenang platform (BL-53).
func TestWorkspaceNav_SettingsGroupChildren(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
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
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false)
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
	nav := workspaceNav("acme", false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	if _, ok := findItem(nav, "Settings"); ok {
		t.Error("tanpa izin pengaturan, grup Settings harus disembunyikan")
	}
	// Modul disabled tetap tampil untuk semua (peta jalan produk).
	if _, ok := findItem(nav, "Sales"); !ok {
		t.Error("modul disabled tetap tampil walau tanpa izin pengaturan")
	}
}

// TestWorkspaceNav_HrefSlugBenar: semua anchor enabled menunjuk slug yang benar
// (regresi wsPath — menu bergantung slug sejak 0004).
func TestWorkspaceNav_HrefSlugBenar(t *testing.T) {
	nav := workspaceNav("beta", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
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
