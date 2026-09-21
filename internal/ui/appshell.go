package ui

// appshell.go — kerangka layout dashboard: tipe data ShellData + struktur
// halaman (head, backdrop, hamburger, sidebar, area konten).
//
// Bagian sidebar yang berdiri sendiri dipisah agar tiap file punya satu alasan
// berubah: shellbrand.go (identitas workspace + panel), shellnav.go (menu &
// active-state), shelluser.go (footer identitas user).

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"

	"go_starter/internal/changelog"
)

// NavItem = satu entri menu sidebar. Tiga bentuk, saling eksklusif:
//   - link biasa: Href terisi, Children/Disabled kosong;
//   - modul belum jadi: Disabled=true (Href diabaikan) → dirender non-link
//     teredam + badge "Segera hadir" (wireframe menampilkan peta jalan penuh,
//     tapi pintunya belum terbuka);
//   - grup bersarang: Children terisi (Href diabaikan) → header collapsible +
//     anak-anak indent (mis. Settings).
type NavItem struct {
	Label    string
	Href     string
	Icon     g.Node    // ikon lucide (mis. lucide.Users(...))
	Disabled bool      // true → item mati (modul wireframe belum dibangun)
	Children []NavItem // non-kosong → grup bersarang, bukan link
}

// ShellData = konteks AppShell (panel /dev, /admin, /user). Terpisah dari
// LayoutData (landing/login).
type ShellData struct {
	Title         string
	BrandLabel    string // konteks panel di sidebar (mis. "go_starter /dev")
	WorkspaceName string // nama workspace AKTIF — brand utama; "" utk platform tanpa konteks
	CurrentPath   string // untuk active-state menu
	UserEmail     string
	AvatarURL     string
	CSSPath       string
	Nav           []NavItem
	QuickLinks    []NavItem // pintasan lintas-panel (sesuai role), di footer sidebar

	// Panel = identitas visual shell yang sedang dibuka (lihat panelkind.go).
	// Ketiga panel memakai AppShell yang sama; tanpa ini user tak tahu sedang
	// di mana — dan /dev menampilkan data lintas-workspace.
	Panel Panel

	// Notifications = entri Notifikasi + jumlah yang perlu perhatian. Blok
	// TERSENDIRI, bukan bagian Nav/QuickLinks: notifikasi milik USER (bisa datang
	// dari workspace mana pun, termasuk yang belum ia masuki), sedangkan Nav =
	// menu panel dan QuickLinks = pintasan PINDAH panel. nil → tak dirender.
	Notifications *NavBadge

	// Workspaces = semua workspace milik user (model membership). >1 → brand jadi
	// dropdown switcher. ActiveTenantID menandai yang sedang dipakai.
	Workspaces         []WorkspaceOption
	ActiveTenantID     int64
	CanCreateWorkspace bool // kuota belum penuh → tampilkan "Buat workspace baru"

	// Changelog = catatan rilis berfokus CRM (paket internal/changelog).
	// ChangelogVersion = versi terbaru → acuan badge "ada pembaruan" (dibanding
	// versi terakhir-dilihat di localStorage oleh static/changelog.js).
	// ChangelogReleases = seluruh rilis yang ditampilkan modal. Dioper dari
	// handler (view murni-data), bukan diimpor logika di view.
	ChangelogVersion  string
	ChangelogReleases []changelog.Release

	// WSBase = prefix workspace AKTIF ("/w/{slug}"), "" di luar konteks
	// workspace (/dev, /notifications). Dipakai RegionSearchModal (BL-163)
	// merakit URL @post yang benar — modal dirender SEKALI di sini tapi
	// endpoint-nya ter-NEST di bawah rute workspace (routes.go), jadi tak
	// bisa ditulis sebagai path absolut "/regions/search" begitu saja.
	WSBase string

	// ShowJenaAI (BL-162 PoC) = gabungan permission Casbin (ai:chat/use) +
	// konfigurasi proxy tersedia, dihitung handler (render_shell.go) — view
	// tak boleh cek authz/config sendiri. false → JenaAIWidget tak render
	// apa pun (bukan CSS disembunyikan; lihat internal/ui/jena_ai.go).
	ShowJenaAI    bool
	JenaAIPostURL string // wsPath(slug, "jena-ai/ask") — view tak rakit path sendiri.
}

// NavBadge = entri menu dengan penghitung. Count 0 → badge disembunyikan (angka
// nol bukan informasi; hanya menambah bising).
type NavBadge struct {
	Item  NavItem
	Count int64
}

// WorkspaceOption = satu workspace di switcher sidebar.
type WorkspaceOption struct {
	TenantID int64
	Name     string
	Role     string
}

// AppShell membungkus konten dengan layout dashboard: sidebar (menu + brand +
// footer user) + area konten. TANPA header — user pindah ke bawah sidebar.
// Sidebar bisa di-collapse jadi rail ikon (state persisten via sidebar.js +
// localStorage). Drawer mobile via signal Datastar; tombol buka = floating
// hamburger (karena header dihilangkan).
func AppShell(d ShellData, content ...g.Node) g.Node {
	cssPath := d.CSSPath
	if cssPath == "" {
		cssPath = "/static/app.css"
	}
	head := append(headNodes(cssPath),
		// sidebar.js SINKRON (bukan defer): set data-sidebar sebelum paint → no flicker.
		h.Script(h.Src("/static/sidebar.js")),
		// changelog.js DEFER: badge "ada pembaruan" dikelola setelah DOM siap.
		h.Script(h.Src("/static/changelog.js"), h.Defer()),
		// regiontree.js DEFER: cascading tab "Wilayah" modal RegionSearch
		// (perluasan BL-163) — select kosong tetap valid sebelum JS jalan,
		// tak ada risiko FOUC (beda dgn sidebar.js/theme.js yang set atribut
		// <html> sebelum paint).
		h.Script(h.Src("/static/regiontree.js"), h.Defer()),
		// ai-widget.js DEFER: auto-scroll #jena-thread + Escape-to-close, murni
		// kosmetik JS — jenaOpen mulai false lewat signal Datastar, tak ada FOUC.
		h.Script(h.Src("/static/ai-widget.js"), h.Defer()),
	)
	return c.HTML5(c.HTML5Props{
		Title:    d.Title,
		Language: "id",
		Head:     head,
		Body: []g.Node{
			// Latar dasar = base-200; sidebar & card = base-100 (permukaan
			// menonjol). Hierarki relatif ini benar otomatis di semua tema.
			// overflow-x-hidden WAJIB: jenaPanel (jena_ai.go) docked kanan pakai
			// `translate-x-full` (POSITIF) untuk closed state — position:fixed yang
			// digeser melewati tepi viewport tetap dihitung browser ke
			// documentElement.scrollWidth (beda dari sidebar kiri yang
			// -translate-x-full/negatif, tak pernah memperluas scrollWidth). Tanpa
			// ini halaman jadi bisa di-scroll horizontal & auto-scroll mendorong
			// tombol trigger kiri-bawah ikut bergeser dari pandangan.
			h.Class("min-h-screen overflow-x-hidden bg-base-200 text-base-content"),
			data.Signals(map[string]any{
				"sidebarOpen": false, "logoutConfirm": false, "changelogOpen": false,
				// jenaLoading = data-indicator bawaan Datastar (jena_ai.go jenaForm):
				// true selama @post ask masih in-flight, otomatis false lagi saat
				// selesai (sukses/gagal) — dipakai toggle bubble pending+loading dots.
				// jenaPending = teks pertanyaan yang sedang diproses, disalin dari
				// input SEBELUM form di-reset, ditampilkan optimistic sebelum jawaban
				// server tiba (lihat jenaPendingBubble).
				"jenaOpen": false, "jenaLoading": false, "jenaPending": "",
				regionSearchSignal: false, regionSearchTabSignal: "kode",
			}),

			// Backdrop mobile — inline display:none agar tak FOUC sebelum Datastar aktif.
			h.Div(
				h.Class("fixed inset-0 z-30 bg-black/50 md:hidden"),
				g.Attr("style", "display:none"),
				data.Show("$sidebarOpen"),
				data.On("click", "$sidebarOpen = false"),
			),

			// Floating hamburger — hanya mobile, hanya saat drawer tertutup.
			h.Button(
				h.Type("button"),
				h.Class("btn btn-outline btn-sm fixed top-4 left-4 z-20 md:hidden"),
				g.Attr("aria-label", "Buka menu"),
				data.Show("!$sidebarOpen"),
				data.On("click", "$sidebarOpen = true"),
				lucide.Menu(h.Class("size-5")),
			),

			shellSidebar(d),

			// Konten. Margin kiri menyesuaikan lebar sidebar (rail saat collapsed).
			// pt-16 di mobile memberi ruang untuk floating hamburger (top-4, fixed)
			// agar tak menimpa judul konten; md: kembali ke padding normal (sidebar
			// tetap tampil, hamburger hilang → tak perlu ruang ekstra).
			h.Div(
				h.Class("app-content flex min-h-screen flex-col"),
				h.Main(h.Class("flex-1 px-4 pb-6 pt-16 sm:px-6 md:p-6"), g.Group(content)),
			),

			// Modal konfirmasi logout (dipicu tombol Keluar).
			ConfirmModal("logoutConfirm", "Keluar?",
				"Anda akan keluar dari sesi ini.", "Keluar", "/logout"),

			// Modal Pembaruan (dipicu tombol Pembaruan di footer sidebar).
			ChangelogModal(d.ChangelogReleases),

			// Modal global "Cari Kode Desa/Kecamatan" (BL-163) — dipicu
			// ui.RegionSearchTrigger() dari halaman mana pun (form Akun, impor
			// CSV, dst). d.WSBase "" (di luar konteks workspace, mis. /dev) →
			// modal tetap dirender (markup konsisten di semua panel) tapi tak
			// dipicu di sana (trigger belum dipasang di halaman /dev mana pun).
			RegionSearchModal(d.WSBase),

			// Jena AI (BL-162 PoC) — tombol floating + panel chat. show=false
			// (izin/konfigurasi tak lengkap) → tak render apa pun sama sekali.
			JenaAIWidget(d.ShowJenaAI, d.JenaAIPostURL),

			// Slot toast global "flash" — satu-satunya target patchFlash
			// (internal/handler/dev_users_flash.go) yang dipakai JenaAIAsk untuk
			// SEMUA jalur non-sukses (belum dikonfigurasi/kosong/terlalu
			// panjang/error provider). Tanpa slot ini di AppShell, PatchElements SSE
			// tak menemukan id="flash" di DOM /w/{slug} (hanya ada di halaman
			// mandiri /dev/users) → patch jadi no-op senyap (cuma console warning
			// PatchElementsNoTargetsFound), user tak melihat apa pun sama sekali
			// walau backend sudah membalas 200. Ditaruh SEKALI di sini (bukan
			// per-halaman panel seperti toast PRG ?ok=/?err= tiap modul CRM) karena
			// Jena AI sendiri global lintas-halaman, bukan konten spesifik satu page.
			ToastSlot("flash"),
		},
	})
}

// shellSidebar = panel navigasi. Kolom flex: brand+toggle (atas), menu (tengah,
// flex-1), footer user (bawah). Class `app-sidebar` dipakai CSS collapse rail.
func shellSidebar(d ShellData) g.Node {
	return h.Aside(
		h.Class("app-sidebar fixed inset-y-0 left-0 z-40 flex flex-col border-r border-base-300 "+
			"bg-base-100 text-base-content -translate-x-full transition-transform md:translate-x-0"),
		// ClassOn mengutip nama class otomatis → key ber-hyphen mustahil salah
		// (gotcha #5). Bandingkan data.Class mentah yang butuh kutip manual.
		ClassOn("translate-x-0", "$sidebarOpen"),

		// Aksen panel di tepi paling atas — terlihat lebih dulu dari apa pun,
		// dan tetap ada saat sidebar collapse (chip ikut tersembunyi).
		panelEdge(d.Panel),

		// Header sidebar: brand + tombol collapse (desktop). Brand = nama workspace
		// (utama) + konteks panel (sub-label kecil). Platform tanpa nama → BrandLabel.
		// app-shellhead = kait collapse (BL-64): saat rail 4rem, brand tersembunyi
		// (.app-brand) → header disetel justify-center + padding-inline:0 di
		// input.css agar tombol collapse TEPAT di tengah rail (bukan mepet kiri
		// sisa px-4). Tanpa kait ini, justify-between bawaan menyisakan tombol
		// menempel tepi.
		h.Div(
			h.Class("app-shellhead h-16 flex items-center gap-2 px-4 border-b border-base-300"),
			shellBrand(d),
			h.Button(
				h.Type("button"),
				h.Class("btn btn-ghost btn-sm hidden md:inline-flex"),
				g.Attr("data-sidebar-toggle", "true"),
				g.Attr("aria-label", "Perkecil sidebar"),
				lucide.PanelLeft(h.Class("size-5")),
			),
		),

		// Menu (tengah, mengisi ruang). activeHref dihitung SEKALI (longest-match)
		// agar hanya satu item menyala walau ada sub-route (mis. /admin/workspace).
		h.Nav(
			h.Class("flex-1 px-3 py-4 flex flex-col gap-1 overflow-y-auto"),
			navList(d.Nav, d.CurrentPath),
		),

		// Notifikasi — blok sendiri di atas pintasan panel (lihat ShellData).
		notifBlock(d),

		// Pintasan lintas-panel (sesuai role) — di atas blok user.
		quickLinks(d),

		// Footer user (bawah): avatar + email + logout.
		sidebarUser(d),
	)
}
