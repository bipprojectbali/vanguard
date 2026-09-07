package handler

import (
	"testing"

	"go_starter/internal/ui"
)

// workspace_nav_test.go — menu ruang kerja yang diselaraskan dengan wireframe CRM:
// label Inggris, modul belum-jadi tampil disabled, Settings jadi grup bersarang.
// workspaceNav = fungsi murni atas argumen izin → diuji langsung tanpa authz.
//
// Urutan argumen izin: canMembers, canSettings, canAccounts, canContacts,
// canLeads, canDeals, canSalesActivity, canAllActivities, canPlans, canSubs,
// canSLA, canPlaybooks, canKB, canRoles, canTickets, canHealthScore,
// canSuccessPlans, canEngagements, canRenewals, canReports, canImplTasks,
// canTrainings.

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
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
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
// Sales & Subscriptions & Customer Success & Reports BUKAN lagi item flat
// disabled — kini grup bersarang (diuji terpisah di
// TestWorkspaceNav_SalesGroup/SubscriptionsGroup/CSGroup/ReportsGroup).
// Activities sudah berbackend (M7). Tak ada lagi modul top-level flat-disabled
// tersisa — daftar kosong disengaja, menandai migrasi terakhir (Reports, M8-1).
func TestWorkspaceNav_DisabledModules(t *testing.T) {
	// Dipertahankan sebagai penanda: bila modul wireframe baru mendarat sebagai
	// flat placeholder disabled, tambahkan labelnya ke sini. Saat ini kosong —
	// semua modul top-level (Sales/Subscriptions/CS/Reports) sudah jadi grup
	// bersarang, diuji masing-masing di TestWorkspaceNav_*Group.
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true)
	if _, ok := findItem(nav, "Dashboard"); !ok {
		t.Fatal("smoke check: Dashboard harus selalu ada")
	}
}

// TestWorkspaceNav_ActivitiesTopLevel: Activities top-level enabled (→ /activity-log)
// saat canAllActivities=true; disabled (tanpa Href) saat false. Backend M7.
// canAllActivities = canViewActivities || isPlatformRole (crm:activities read +
// platform bypass; BL-39 — objek berbeda dari canSalesActivity).
func TestWorkspaceNav_ActivitiesTopLevel(t *testing.T) {
	// Kasus 1: canSalesActivity=true → canAllActivities=true → Activities enabled.
	navCRM := workspaceNav("acme", false, false, false, false, false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	it, ok := findItem(navCRM, "Activities")
	if !ok {
		t.Fatal("Activities harus tampil saat canAllActivities=true")
	}
	if it.Disabled {
		t.Errorf("Activities harus enabled saat canAllActivities=true")
	}
	if it.Href != "/w/acme/activity-log" {
		t.Errorf("Activities href %q, want /w/acme/activity-log", it.Href)
	}

	// Kasus 2: canSalesActivity=false, canAllActivities=true (platform bypass —
	// super_admin/staff tanpa business_role tetap boleh lihat Activities).
	navPlatform := workspaceNav("acme", false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	itPlatform, ok := findItem(navPlatform, "Activities")
	if !ok {
		t.Fatal("Activities harus tampil saat canAllActivities=true (platform)")
	}
	if itPlatform.Disabled {
		t.Errorf("Activities harus enabled saat canAllActivities=true (platform bypass)")
	}
	if itPlatform.Href != "/w/acme/activity-log" {
		t.Errorf("Activities platform href %q, want /w/acme/activity-log", itPlatform.Href)
	}

	// Kasus 3: canAllActivities=false → Activities disabled, tanpa href.
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	itNone, ok := findItem(navNone, "Activities")
	if !ok {
		t.Fatal("Activities harus tetap tampil walau tanpa izin (peta jalan)")
	}
	if !itNone.Disabled {
		t.Errorf("Activities harus disabled saat canAllActivities=false")
	}
	if itNone.Href != "" {
		t.Errorf("Activities disabled tak boleh punya Href, got %q", itNone.Href)
	}
}

// TestWorkspaceNav_SalesGroup: Sales = grup bersarang yang SELALU tampil. Anak
// Leads/Deals/Quotes/Sales Activities enabled mengikuti izin (Quotes ikut canDeals
// — quote mewarisi izin baca deal; Sales Activities ikut canSalesActivity, objek
// crm:sales_activity).
func TestWorkspaceNav_SalesGroup(t *testing.T) {
	// Izin CRM → Leads, Deals, Quotes & Sales Activities enabled dengan href benar.
	nav := workspaceNav("acme", false, false, false, false, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
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
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
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
// tampil (peta jalan). Semua anak berbackend (Subscription Lists/Renewals/Churn
// via canSubs objek crm:subscriptions; Plans & Pricing via canPlans objek crm:plans)
// enabled mengikuti izinnya masing-masing — sejak M5-4 tak ada lagi placeholder.
func TestWorkspaceNav_SubscriptionsGroup(t *testing.T) {
	// canPlans & canSubs → grup tampil, keempat anak enabled + href benar.
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Subscriptions")
	if !ok {
		t.Fatal("grup Subscriptions harus selalu tampil (peta jalan)")
	}
	if grp.Href != "" {
		t.Errorf("header grup Subscriptions tak boleh jadi link, got Href %q", grp.Href)
	}
	wantEnabled := map[string]string{
		"Plans & Pricing":       "/w/acme/plans",
		"Subscription Lists":    "/w/acme/subscriptions",
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
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grpNone, ok := findItem(navNone, "Subscriptions")
	if !ok {
		t.Fatal("grup Subscriptions tetap tampil walau tanpa izin")
	}
	for _, label := range []string{"Plans & Pricing", "Subscription Lists", "Renewals", "Churn / Cancellations"} {
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
