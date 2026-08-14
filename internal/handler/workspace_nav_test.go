package handler

import (
	"strings"
	"testing"

	"go_starter/internal/ui"
)

// workspace_nav_test.go — menu ruang kerja yang diselaraskan dengan wireframe CRM:
// label Inggris, modul belum-jadi tampil disabled, Settings jadi grup bersarang.
// workspaceNav = fungsi murni atas argumen izin → diuji langsung tanpa authz.
//
// Urutan argumen izin: canMembers, canSettings, canAccounts, canContacts,
// canLeads, canDeals, canSalesActivity, canPlans, canSubs, canSLA,
// canPlaybooks, canKB, canRoles, canTickets, canHealthScore, canEngagements,
// canRenewals.

// findItem mencari item top-level berlabel tertentu.
func findItem(nav []ui.NavItem, label string) (ui.NavItem, bool) {
	for _, it := range nav {
		if it.Label == label {
			return it, true
		}
	}
	return ui.NavItem{}, false
}

// TestWorkspaceNav_EnglishTopLevel: label & urutan top-level mengikuti wireframe
// (Dashboard, Accounts, Contacts, lalu modul disabled). Dashboard selalu ada dan
// jadi item pertama (penanda ruang kerja).
func TestWorkspaceNav_EnglishTopLevel(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
	if nav[0].Label != "Dashboard" || nav[0].Href != "/w/acme" {
		t.Fatalf("item pertama harus Dashboard→/w/acme, got %q→%q", nav[0].Label, nav[0].Href)
	}
	for _, label := range []string{"Accounts", "Contacts", "Sales", "Reports", "Settings"} {
		if _, ok := findItem(nav, label); !ok {
			t.Errorf("menu wireframe kurang item %q", label)
		}
	}
	// Label lama bahasa Indonesia tak boleh tersisa.
	for _, gone := range []string{"Beranda", "Desa", "Kontak", "Peran", "Anggota", "Pengaturan"} {
		if _, ok := findItem(nav, gone); ok {
			t.Errorf("label lama %q harus sudah diganti ke Inggris", gone)
		}
	}
}

// TestWorkspaceNav_DisabledModules: modul berwireframe tapi belum berbackend
// tampil TAPI mati — Disabled=true & tanpa Href (bukan link yang lalu 404).
func TestWorkspaceNav_DisabledModules(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
	// Sales & Subscriptions & Customer Success BUKAN lagi item flat disabled —
	// kini grup bersarang (diuji terpisah di
	// TestWorkspaceNav_SalesGroup/SubscriptionsGroup/CSGroup).
	for _, label := range []string{"Activities", "Reports"} {
		it, ok := findItem(nav, label)
		if !ok {
			t.Errorf("modul %q harus tampil (disabled), bukan hilang", label)
			continue
		}
		if !it.Disabled {
			t.Errorf("modul %q harus Disabled (backend belum ada)", label)
		}
		if it.Href != "" {
			t.Errorf("modul disabled %q tak boleh punya Href, got %q", label, it.Href)
		}
	}
}

// TestWorkspaceNav_SalesGroup: Sales = grup bersarang yang SELALU tampil. Anak
// Leads/Deals/Quotes/Sales Activities enabled mengikuti izin (Quotes ikut canDeals
// — quote mewarisi izin baca deal; Sales Activities ikut canSalesActivity, objek
// crm:sales_activity).
func TestWorkspaceNav_SalesGroup(t *testing.T) {
	// Izin CRM → Leads, Deals, Quotes & Sales Activities enabled dengan href benar.
	nav := workspaceNav("acme", false, false, false, false, true, true, true, false, false, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Sales")
	if !ok {
		t.Fatal("grup Sales harus selalu tampil (peta jalan)")
	}
	if grp.Href != "" {
		t.Errorf("header grup Sales tak boleh jadi link, got Href %q", grp.Href)
	}
	want := map[string]string{
		"Leads":            "/w/acme/leads",
		"Deals":            "/w/acme/deals",
		"Quotes":           "/w/acme/quotes",
		"Sales Activities": "/w/acme/activities",
	}
	for label, href := range want {
		ch, ok := findItem(grp.Children, label)
		if !ok {
			t.Errorf("grup Sales kurang anak %q", label)
			continue
		}
		if ch.Disabled || ch.Href != href {
			t.Errorf("anak %q harus enabled→%q, got disabled=%v href=%q", label, href, ch.Disabled, ch.Href)
		}
	}

	// Tanpa izin → grup tetap tampil, tapi anak disabled tanpa href
	// (menu tak menawarkan pintu yang lalu ditolak 403).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grpNone, ok := findItem(navNone, "Sales")
	if !ok {
		t.Fatal("grup Sales tetap tampil walau tanpa izin CRM")
	}
	for _, label := range []string{"Leads", "Deals", "Quotes", "Sales Activities"} {
		ch, ok := findItem(grpNone.Children, label)
		if !ok {
			t.Errorf("anak %q harus tetap tampil (disabled)", label)
			continue
		}
		if !ch.Disabled || ch.Href != "" {
			t.Errorf("tanpa izin, anak %q harus disabled tanpa href, got disabled=%v href=%q", label, ch.Disabled, ch.Href)
		}
	}
}

// TestWorkspaceNav_SubscriptionsGroup: Subscriptions = grup bersarang yang SELALU
// tampil (peta jalan). Semua anak berbackend (Active Subscriptions/Renewals/Churn
// via canSubs objek crm:subscriptions; Plans & Pricing via canPlans objek crm:plans)
// enabled mengikuti izinnya masing-masing — sejak M5-4 tak ada lagi placeholder.
func TestWorkspaceNav_SubscriptionsGroup(t *testing.T) {
	// canPlans & canSubs → grup tampil, keempat anak enabled + href benar.
	nav := workspaceNav("acme", false, false, false, false, false, false, false, true, true, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Subscriptions")
	if !ok {
		t.Fatal("grup Subscriptions harus selalu tampil (peta jalan)")
	}
	if grp.Href != "" {
		t.Errorf("header grup Subscriptions tak boleh jadi link, got Href %q", grp.Href)
	}
	wantEnabled := map[string]string{
		"Plans & Pricing":       "/w/acme/plans",
		"Active Subscriptions":  "/w/acme/subscriptions",
		"Renewals":              "/w/acme/subscriptions/renewals",
		"Churn / Cancellations": "/w/acme/subscriptions/churn",
	}
	for label, href := range wantEnabled {
		ch, ok := findItem(grp.Children, label)
		if !ok {
			t.Errorf("grup Subscriptions kurang anak %q", label)
			continue
		}
		if ch.Disabled || ch.Href != href {
			t.Errorf("anak %q harus enabled→%q, got disabled=%v href=%q", label, href, ch.Disabled, ch.Href)
		}
	}

	// Tanpa izin → grup tetap tampil, tapi semua anak berbackend disabled tanpa href
	// (menu tak menawarkan pintu yang lalu ditolak 403).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grpNone, ok := findItem(navNone, "Subscriptions")
	if !ok {
		t.Fatal("grup Subscriptions tetap tampil walau tanpa izin")
	}
	for _, label := range []string{"Plans & Pricing", "Active Subscriptions", "Renewals", "Churn / Cancellations"} {
		ch, ok := findItem(grpNone.Children, label)
		if !ok {
			t.Errorf("anak %q harus tetap tampil (disabled)", label)
			continue
		}
		if !ch.Disabled || ch.Href != "" {
			t.Errorf("tanpa izin, anak %q harus disabled tanpa href, got disabled=%v href=%q", label, ch.Disabled, ch.Href)
		}
	}
}

// TestWorkspaceNav_CSGroup: Customer Success = grup bersarang yang SELALU tampil
// (peta jalan Modul 6). "Health Score" (C1), "Engagements" (6.5), "Renewal
// Management" (6.6), "SLA Management" (A1), "Playbooks" (A2), "Knowledge Base"
// (A3) & "Tickets / Cases" (B2) berbackend — masing-masing enabled mengikuti
// izinnya. 2 anak SELALU disabled (Success Plans, Voice of Customer).
func TestWorkspaceNav_CSGroup(t *testing.T) {
	placeholders := []string{
		"Success Plans", "Voice of Customer",
	}

	// canSLA=true & canPlaybooks=true & canKB=true & canTickets=true &
	// canHealthScore=true & canEngagements=true → grup tampil, semua berbackend
	// enabled dengan href benar.
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, true, true, true, false, true, true, true, true)
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
	// tetap tampil, keempatnya disabled tanpa href (menu tak menawarkan pintu
	// yang lalu ditolak 403).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
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
}

// TestWorkspaceNav_SettingsGroupChildren: Settings = grup bersarang; anak enabled
// mengikuti izin masing-masing (sumber sama dengan gerbang halaman), plus dua
// placeholder disabled. Semua izin → keenam anak hadir dengan href benar.
func TestWorkspaceNav_SettingsGroupChildren(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
	grp, ok := findItem(nav, "Settings")
	if !ok {
		t.Fatal("grup Settings harus ada saat user punya izin pengaturan")
	}
	if grp.Href != "" {
		t.Errorf("header grup Settings tak boleh jadi link, got Href %q", grp.Href)
	}
	want := map[string]string{
		"Workspace Settings":  "/w/acme/settings",
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
		ch, ok := findItem(grp.Children, label)
		if !ok || !ch.Disabled {
			t.Errorf("placeholder %q harus ada & disabled", label)
		}
	}
}

// TestWorkspaceNav_SettingsGatingPerIzin: tiap anak enabled muncul HANYA jika
// izinnya diberikan — menu tak menawarkan pintu yang lalu ditolak 403.
func TestWorkspaceNav_SettingsGatingPerIzin(t *testing.T) {
	// Hanya canRoles → grup ada, tapi cuma Roles yang enabled (+placeholder).
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false)
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
	nav := workspaceNav("acme", false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false)
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
	nav := workspaceNav("beta", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
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
