package handler

import (
	"reflect"
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
// canSuccessPlans, canEngagements, canRenewals, canReports, canJourney.

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
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, false)
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

// TestWorkspaceNav_DisabledModules: modul berwireframe tapi tak berizin kini
// TAK DITAMPILKAN sama sekali (bukan Disabled=true teredam) — diganti (user,
// 17 Sep) dari pola lama "tampil mati agar posisi modul di peta jalan
// terlihat". Sales & Subscriptions & Customer Success & Reports adalah grup
// bersarang (diuji terpisah di
// TestWorkspaceNav_SalesGroup/SubscriptionsGroup/CSGroup/ReportsGroup).
// Activities top-level diuji di TestWorkspaceNav_ActivitiesTopLevel.
func TestWorkspaceNav_DisabledModules(t *testing.T) {
	// Dipertahankan sebagai smoke check + penanda: bila modul wireframe baru
	// mendarat sebagai flat placeholder, tambahkan assert-nya di sini.
	nav := workspaceNav("acme", true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, false)
	if _, ok := findItem(nav, "Dashboard"); !ok {
		t.Fatal("smoke check: Dashboard harus selalu ada")
	}
}

// TestWorkspaceNav_ActivitiesTopLevel: Activities top-level enabled (→ /activity-log)
// saat canAllActivities=true; TAK DITAMPILKAN sama sekali saat false (bukan
// disabled teredam — diganti user 17 Sep). Backend M7. canAllActivities =
// canViewActivities || isPlatformRole (crm:activities read + platform bypass;
// BL-39 — objek berbeda dari canSalesActivity).
func TestWorkspaceNav_ActivitiesTopLevel(t *testing.T) {
	// Kasus 1: canSalesActivity=true → canAllActivities=true → Activities enabled.
	navCRM := workspaceNav("acme", false, false, false, false, false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
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
	navPlatform := workspaceNav("acme", false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
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

	// Kasus 3: canAllActivities=false → Activities tak ditampilkan sama sekali.
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	if _, ok := findItem(navNone, "Activities"); ok {
		t.Error("Activities harus disembunyikan total saat canAllActivities=false (bukan disabled)")
	}
}

// TestWorkspaceNav_SalesGroup: Sales = grup bersarang, disembunyikan total
// bila tak ada izin ke satu pun anak (sama pola dgn Settings). Anak
// Leads/Deals/Quotes/Sales Activities enabled mengikuti izin (Quotes ikut canDeals
// — quote mewarisi izin baca deal; Sales Activities ikut canSalesActivity, objek
// crm:sales_activity).
func TestWorkspaceNav_SalesGroup(t *testing.T) {
	// Izin CRM → Leads, Deals, Quotes & Sales Activities enabled dengan href benar.
	nav := workspaceNav("acme", false, false, false, false, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
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

	// Tanpa izin ke satu pun anak → grup disembunyikan total (sama pola dgn
	// Settings): sidebar tak numpuk grup mati bagi role sempit.
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	if _, ok := findItem(navNone, "Sales"); ok {
		t.Error("grup Sales harus disembunyikan total saat tak ada izin ke satu pun anaknya")
	}
}

// TestWorkspaceNav_SubscriptionsGroup: Subscriptions = grup bersarang,
// disembunyikan total bila tak ada izin ke satu pun anak (sama pola dgn
// Settings). Semua anak berbackend (Subscription Lists/Renewals/Churn
// via canSubs objek crm:subscriptions; Plans & Pricing via canPlans objek crm:plans)
// enabled mengikuti izinnya masing-masing — sejak M5-4 tak ada lagi placeholder.
func TestWorkspaceNav_SubscriptionsGroup(t *testing.T) {
	// canPlans & canSubs → grup tampil, keempat anak enabled + href benar.
	nav := workspaceNav("acme", false, false, false, false, false, false, false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	grp, ok := findItem(nav, "Subscriptions")
	if !ok {
		t.Fatal("grup Subscriptions harus tampil saat ada izin ke minimal satu anak")
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

	// BL-91: URUTAN anak grup = Subscription Lists → Renewals → Churn /
	// Cancellations → Plans & Pricing (Churn sengaja di atas Plans & Pricing).
	wantOrder := []string{"Subscription Lists", "Renewals", "Churn / Cancellations", "Plans & Pricing"}
	gotOrder := make([]string, len(grp.Children))
	for i, ch := range grp.Children {
		gotOrder[i] = ch.Label
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Errorf("urutan anak grup Subscriptions salah\n want %v\n got  %v", wantOrder, gotOrder)
	}

	// Tanpa izin ke satu pun anak → grup disembunyikan total (sama pola dgn
	// Settings).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false)
	if _, ok := findItem(navNone, "Subscriptions"); ok {
		t.Error("grup Subscriptions harus disembunyikan total saat tak ada izin ke satu pun anaknya")
	}
}
