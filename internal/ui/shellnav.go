package ui

// shellnav.go — rendering menu sidebar: daftar item, active-state, badge.
// Dipisah dari appshell.go (kerangka layout) karena aturan "item mana yang
// menyala" adalah logika tersendiri yang berubah karena alasan berbeda dari
// struktur shell-nya.

import (
	"strconv"
	"strings"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// navList merender daftar item + menandai satu yang aktif (longest-match dihitung
// sekali, bukan per-item). Dipakai menu utama & quickLinks. active dihitung atas
// SELURUH pohon (flattenNav) — anak grup pun bisa jadi item aktif.
func navList(items []NavItem, currentPath string) g.Node {
	active := activeNavHref(flattenNav(items), currentPath)
	return g.Map(items, func(it NavItem) g.Node { return navNode(it, active) })
}

// navNode memilih bentuk render satu entri: grup bersarang, item mati (disabled),
// atau link biasa. Urutan cek penting — grup diperiksa lebih dulu karena Href-nya
// sengaja diabaikan.
func navNode(it NavItem, activeHref string) g.Node {
	switch {
	case len(it.Children) > 0:
		return navGroup(it, activeHref)
	case it.Disabled:
		return navDisabled(it)
	default:
		return navLink(it, activeHref)
	}
}

// flattenNav mengumpulkan semua item yang benar-benar bisa dituju (link ber-href,
// termasuk anak grup) untuk perhitungan active longest-match. Header grup
// (Href kosong) & item mati dilewati: keduanya bukan tujuan navigasi, dan header
// ber-Href kosong akan salah cocok ("/"+"/" jadi prefix segala path).
func flattenNav(items []NavItem) []NavItem {
	out := make([]NavItem, 0, len(items))
	for _, it := range items {
		if len(it.Children) > 0 {
			out = append(out, flattenNav(it.Children)...)
			continue
		}
		if it.Disabled || it.Href == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}

// quickLinks merender pintasan lintas-panel (mis. ke /dev, /admin) sesuai role.
// Kosong (nil) → tak render apa pun. Item aktif ditandai seperti navLink.
func quickLinks(d ShellData) g.Node {
	if len(d.QuickLinks) == 0 {
		return g.Text("")
	}
	return h.Div(
		h.Class("border-t border-base-300 px-3 py-2 flex flex-col gap-1"),
		navList(d.QuickLinks, d.CurrentPath),
	)
}

// notifBlock merender entri Notifikasi + badge jumlah. nil → tak render apa pun.
// Memakai navLink yang sama dengan menu lain agar active-state & hover identik;
// badge ditempel sebagai saudara, bukan cabang render terpisah.
func notifBlock(d ShellData) g.Node {
	if d.Notifications == nil {
		return g.Text("")
	}
	it := d.Notifications.Item
	active := ""
	if it.Href == d.CurrentPath || strings.HasPrefix(d.CurrentPath, it.Href+"/") {
		active = it.Href
	}
	badge := g.Node(g.Text(""))
	if d.Notifications.Count > 0 {
		// app-navlabel: ikut disembunyikan saat sidebar collapse jadi rail ikon
		// (CSS yang sama dengan label menu) — badge tak menggantung sendirian.
		badge = h.Span(
			h.Class("app-navlabel badge badge-primary badge-sm ml-auto"),
			g.Text(strconv.FormatInt(d.Notifications.Count, 10)),
		)
	}
	return h.Div(
		h.Class("border-t border-base-300 px-3 py-2 flex flex-col gap-1"),
		h.Div(h.Class("flex items-center"), navLinkWith(it, active, badge)),
	)
}

// activeNavHref mengembalikan href TUNGGAL yang aktif untuk currentPath dari
// daftar nav: LONGEST-MATCH menang. "/admin/workspace" mengaktifkan Workspace
// (href persis), BUKAN Dashboard (href "/admin") — dulu prefix telanjang bikin
// keduanya menyala. Match = exact ATAU sub-path pada batas segmen (href+"/").
// "" bila tak ada yang cocok.
func activeNavHref(nav []NavItem, currentPath string) string {
	best := ""
	for _, it := range nav {
		if it.Href == "/" {
			if currentPath == "/" && len(best) == 0 {
				best = "/"
			}
			continue
		}
		match := it.Href == currentPath || strings.HasPrefix(currentPath, it.Href+"/")
		if match && len(it.Href) > len(best) {
			best = it.Href
		}
	}
	return best
}

// navLink = satu item menu. active bila href-nya = activeHref (dihitung sekali di
// navList). title = tooltip (berguna saat rail collapsed, label tersembunyi).
func navLink(it NavItem, activeHref string) g.Node {
	return navLinkWith(it, activeHref, nil)
}

// navLinkWith = navLink + node tambahan di ujung kanan (mis. badge jumlah).
// Satu implementasi styling untuk semua item menu: varian ber-badge tak bisa
// menyimpang dari yang polos.
func navLinkWith(it NavItem, activeHref string, trailing g.Node) g.Node {
	active := it.Href == activeHref
	// Base + garis kiri transparan (border-l-2 border-transparent) SELALU ada agar
	// lebar/posisi item konsisten active vs non-active (tak bergeser 2px). Hover
	// ditambah per-state agar tak konflik dengan bg active.
	cls := "app-navlink flex items-center gap-3 rounded-md border-l-2 border-transparent px-3 py-2 text-sm"
	if active {
		// Active = SOFT: bg primary transparan tipis + garis kiri primary (aksen
		// warna brand LEMBUT, tak mencolok) — TAPI teks = base-content (bukan
		// text-primary). Alasan: sebagian tema punya primary sangat terang (cupcake
		// oklch 85%) → text-primary di latar terang kontras lemah/tak terbaca.
		// base-content dijamin kontras dgn base di SEMUA tema (token semantik daisyUI).
		// Warna active tetap terasa dari bg+border, keterbacaan tetap AA. Hover naik
		// tipis (masih transparan) — teks tak pernah hilang (beda dari bug lama:
		// hover:bg-base-200 menimpa bg solid → teks putih di atas abu terang).
		cls += " bg-primary/10 text-base-content font-medium border-primary hover:bg-primary/20"
	} else {
		// Non-active: hover netral (surface sedikit terangkat).
		cls += " hover:bg-base-200"
	}
	attrs := []g.Node{h.Href(it.Href), h.Class(cls), h.Title(it.Label)}
	if active {
		attrs = append(attrs, g.Attr("aria-current", "page"), g.Attr("data-active", "true"))
	}
	children := []g.Node{}
	if it.Icon != nil {
		children = append(children, it.Icon)
	}
	children = append(children, h.Span(h.Class("app-navlabel truncate"), g.Text(it.Label)))
	if trailing != nil {
		children = append(children, trailing)
	}
	return h.A(append(attrs, children...)...)
}

// navDisabled = modul yang wireframe-nya ada tapi backend-nya belum dibangun.
// BUKAN <a> (tak ada tujuan — link mati menyesatkan & bisa difokus keyboard):
// <span> teredam saja, tanpa label tambahan. Tak pernah aktif. Label pakai
// app-navlabel agar ikut tersembunyi saat sidebar collapse jadi rail ikon.
func navDisabled(it NavItem) g.Node {
	cls := "app-navlink flex items-center gap-3 rounded-md border-l-2 border-transparent " +
		"px-3 py-2 text-sm text-base-content/40 cursor-not-allowed"
	children := []g.Node{h.Class(cls), h.Title(it.Label), g.Attr("aria-disabled", "true")}
	if it.Icon != nil {
		children = append(children, it.Icon)
	}
	children = append(children, h.Span(h.Class("app-navlabel truncate"), g.Text(it.Label)))
	return h.Span(children...)
}

// navGroup = grup menu bersarang (mis. Settings). Header = TOMBOL toggle (bukan
// link — grup tak punya halaman sendiri), anak-anak indent di bawahnya.
// Collapsible via signal Datastar lokal; default TERBUKA bila salah satu anak
// sedang aktif, sehingga user tak perlu membuka manual untuk melihat posisinya.
// Signal diturunkan dari label (literal internal, bukan input user) → aman CSP.
func navGroup(it NavItem, activeHref string) g.Node {
	sig := navGroupSignal(it.Label)
	childActive := false
	for _, ch := range it.Children {
		if ch.Href != "" && ch.Href == activeHref {
			childActive = true
			break
		}
	}

	// Header: ikon + label + chevron. Grup aktif (anak menyala) → label tebal agar
	// terasa "di sini", tanpa meniru garis-kiri item link (grup bukan tujuan).
	headerCls := "app-navlink flex w-full items-center gap-3 rounded-md border-l-2 " +
		"border-transparent px-3 py-2 text-sm hover:bg-base-200"
	if childActive {
		headerCls += " font-medium"
	}
	headerKids := []g.Node{
		h.Type("button"), h.Class(headerCls), h.Title(it.Label),
		data.On("click", "$"+sig+" = !$"+sig),
	}
	if it.Icon != nil {
		headerKids = append(headerKids, it.Icon)
	}
	headerKids = append(headerKids,
		h.Span(h.Class("app-navlabel truncate"), g.Text(it.Label)),
		lucide.ChevronDown(
			h.Class("app-navlabel size-4 ml-auto transition-transform"),
			ClassOn("rotate-180", "$"+sig),
		),
	)

	kids := make([]g.Node, 0, len(it.Children))
	for _, ch := range it.Children {
		kids = append(kids, navNode(ch, activeHref))
	}
	// Anak-anak indent (pl-3 + garis pemisah tipis). display:none inline saat
	// default tertutup → tak ada kedip "terbuka sesaat" sebelum Datastar aktif
	// (pola sama dengan backdrop di AppShell). app-subnav (BL-71): kait CSS agar
	// saat sidebar collapse jadi rail 4rem, kontainer anak DIPAKSA tampak (ikon
	// submenu terlihat & terpusat) — di rail chevron/label tersembunyi jadi toggle
	// expand tak terjangkau; tanpa ini grup tertutup menyembunyikan ikon anaknya.
	kidsAttrs := []g.Node{h.Class("app-subnav flex flex-col gap-1 pl-3"), data.Show("$" + sig)}
	if !childActive {
		kidsAttrs = append(kidsAttrs, g.Attr("style", "display:none"))
	}

	return h.Div(
		h.Class("flex flex-col gap-1"),
		data.Signals(map[string]any{sig: childActive}),
		h.Button(headerKids...),
		h.Div(append(kidsAttrs, g.Group(kids))...),
	)
}

// navGroupSignal menurunkan nama signal Datastar dari label grup: huruf/angka
// kecil saja, diawali "nav_" agar selalu identifier JS valid. Label internal
// (bukan input user) → tak ada risiko injeksi ekspresi.
func navGroupSignal(label string) string {
	var b strings.Builder
	b.WriteString("nav_")
	for _, r := range strings.ToLower(label) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
