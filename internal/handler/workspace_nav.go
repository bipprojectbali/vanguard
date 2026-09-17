package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav.go — menu sidebar ruang kerja, diselaraskan dengan wireframe CRM
// (label Inggris, urutan modul persis wireframe). Dipisah dari render.go (yang
// sudah lewat ambang file-health): "menu apa yang tampil" adalah concern sendiri
// yang berubah tiap modul baru mendarat. Orkestrator ini SENDIRI dipecah lagi per
// grup (workspace_nav_sales.go/_subscriptions.go/_cs.go/_settings.go) begitu
// jumlah grup nested membuatnya lewat ambang file-health (rule #8) — jangan
// gabungkan grup baru ke sini, tambah file grup baru.
//
// Aturan tampil, semuanya demi "nol menu hantu — pintu yang tampil tak
// pernah ditolak saat diketuk":
//   - Modul BERBACKEND flat (Accounts, Contacts) muncul mengikuti IZIN yang
//     SAMA dengan gerbang halamannya — bukan sekadar keberadaan route.
//   - Grup bersarang (Sales, Subscriptions, Customer Success, Reports,
//     Settings) disembunyikan TOTAL bila user tak berhak ke SATU PUN anaknya
//     (grup kosong tak menawarkan apa pun) — anak yang tersisa enabled
//     mengikuti izin gerbang halamannya masing-masing. Dulu keempat grup CRM
//     ini "selalu tampil" (peta jalan) walau semua anak teredam; diganti
//     (user, 17 Sep) karena sidebar jadi numpuk grup mati bagi role sempit
//     (mis. Subscriptions/Customer Success semua nonaktif) — sekarang sama
//     pola dengan Settings yang dari awal begini.
//   - Anak TANPA izin di dalam grup (dan item top-level Activities) juga tak
//     ditampilkan sama sekali (bukan Disabled=true teredam) — diganti (user,
//     17 Sep) dari pola lama "tampil mati agar posisi modul di peta jalan
//     terlihat"; sekarang konsisten dgn aturan grup di atas. `NavItem.Disabled`
//     (internal/ui/shellnav.go navDisabled) tetap ada di struct untuk item
//     yang memang belum berbackend, tapi tak ada lagi pemakainya saat ini.

// workspaceNav membangun menu ruang kerja untuk slug + izin yang sudah dihitung
// handler. Semua href bergantung SLUG (0004) → menu tak bisa jadi var paket.
//
// Izin dioper terpisah (bukan satu `canManage`) karena keduanya TIDAK identik —
// di mode single admin boleh menyunting workspace tapi keanggotaan dinilai
// sendiri — jadi menyatukannya akan membuat salah satu menu berbohong.
// workspaceNav membangun menu ruang kerja. canSalesActivity = gate CRM bisnis
// (crm:sales_activity) — untuk "Sales Activities" dalam grup Sales.
// canAllActivities = gate halaman lintas-context (/activity-log) — LEBIH LUAS:
// CRM role ATAU platform role (super_admin/staff butuh visibilitas sistem).
func workspaceNav(slug string, canMembers, canSettings, canAccounts, canContacts, canLeads, canDeals, canSalesActivity, canAllActivities, canPlans, canSubs, canSLA, canPlaybooks, canKB, canRoles, canTickets, canHealthScore, canSuccessPlans, canEngagements, canRenewals, canReportsSales, canReportsCS, canReportsSupport, canReportsSubscriptions, canJourney, canTrainings bool) []ui.NavItem {
	items := []ui.NavItem{
		{Label: "Dashboard", Href: wsPath(slug, ""), Icon: lucide.House(html.Class("size-4"))},
	}
	// Accounts (hub desa) — pemegang peran CRM (F2 read crm:accounts). Sumber izin
	// SAMA dengan gerbang AccountsList.
	if canAccounts {
		items = append(items, ui.NavItem{
			Label: "Accounts", Href: wsPath(slug, "/accounts"),
			Icon: lucide.MapPin(html.Class("size-4")),
		})
	}
	// Contacts (orang lintas-desa) — objek Casbin terpisah (crm:contacts), izinnya
	// dinilai sendiri. Sumber izin SAMA dengan gerbang ContactsAll.
	if canContacts {
		items = append(items, ui.NavItem{
			Label: "Contacts", Href: wsPath(slug, "/contacts"),
			Icon: lucide.Contact(html.Class("size-4")),
		})
	}
	// Sales jadi GRUP bersarang (wireframe 4): Leads/Deals/Quotes/Sales Activities
	// enabled per izin. nil (tak berhak ke satu pun anak) → grup disembunyikan.
	if grp := workspaceSalesGroup(slug, canLeads, canDeals, canSalesActivity); grp != nil {
		items = append(items, *grp)
	}
	// Subscriptions jadi GRUP bersarang (wireframe 5): semua anak berbackend,
	// enabled per izin. nil → grup disembunyikan.
	if grp := workspaceSubscriptionsGroup(slug, canPlans, canSubs); grp != nil {
		items = append(items, *grp)
	}
	// Customer Success jadi GRUP bersarang (wireframe 6): semua anak berbackend,
	// enabled per izin. nil → grup disembunyikan.
	if grp := workspaceCSGroup(slug, canSLA, canPlaybooks, canKB, canTickets, canHealthScore, canSuccessPlans, canEngagements, canRenewals, canJourney, canTrainings); grp != nil {
		items = append(items, *grp)
	}
	// Activities top-level = daftar lintas-context (sales+cs+general), M7.
	// Gate crm:activities read (canViewActivities) ATAU platform role — BEDA objek
	// dari Sales Activities (crm:sales_activity): csm/support memegang crm:activities
	// jadi menu ini kini enabled bagi mereka (BL-39). Tanpa izin → item tak
	// ditampilkan sama sekali (bukan disabled — nol menu hantu, sama pola dgn
	// Settings/grup lain).
	if canAllActivities {
		items = append(items, ui.NavItem{
			Label: "Activities",
			Icon:  lucide.Activity(html.Class("size-4")),
			Href:  wsPath(slug, "/activity-log"),
		})
	}
	// Reports jadi GRUP bersarang (wireframe 8): 4 halaman berbackend (M8-1),
	// masing-masing objek Casbin sendiri sejak BL-169 (dulu satu canReports
	// utk semuanya). nil → grup disembunyikan.
	if grp := workspaceReportsGroup(slug, canReportsSales, canReportsCS, canReportsSupport, canReportsSubscriptions); grp != nil {
		items = append(items, *grp)
	}
	if grp := workspaceSettingsGroup(slug, canMembers, canRoles, canSettings); grp != nil {
		items = append(items, *grp)
	}
	return items
}
