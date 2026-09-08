package ui

// shelluser.go — blok identitas user di dasar sidebar (avatar, email, tema,
// logout). Berdiri sendiri: satu-satunya bagian sidebar yang menyentuh sesi user
// dan pemicu modal logout.

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sidebarUser = blok identitas di bagian bawah sidebar (avatar, email, logout).
func sidebarUser(d ShellData) g.Node {
	return h.Div(
		h.Class("border-t border-base-300 p-3 flex flex-col gap-2"),
		// app-shelluser = kait collapse (BL-64): saat rail 4rem, email
		// (.app-navlabel) tersembunyi sehingga hanya avatar + tombol tema tersisa.
		// justify-between bawaan mendorong keduanya ke tepi → keluar rail sempit;
		// di collapse input.css mengubahnya jadi kolom ter-tengah column-reverse
		// (tombol tema DI ATAS avatar) + mengarahkan dropdown tema membuka ke kanan
		// rail agar pilihan tema tak terpotong keluar layar.
		h.Div(
			h.Class("app-shelluser flex items-center justify-between gap-2 min-w-0"),
			h.Div(
				h.Class("flex items-center gap-2 min-w-0"),
				Avatar(d.AvatarURL, "", d.UserEmail, 32),
				h.Span(h.Class("app-navlabel text-sm truncate"), g.Text(d.UserEmail)),
			),
			// Toolbar ikon footer: Pembaruan (Sparkles) di samping Tema. Pembaruan
			// tak dirender bila belum ada rilis (version ""); badge "ada pembaruan"
			// dikelola changelog.js.
			h.Div(
				h.Class("app-usertools flex items-center gap-1"),
				ChangelogButton(d.ChangelogVersion),
				ThemeToggleUp(),
			),
		),
		h.Button(
			h.Class("btn btn-outline btn-sm w-full"),
			g.Attr("title", "Keluar"),
			ConfirmTrigger("logoutConfirm"), // buka modal, bukan langsung logout
			lucide.LogOut(h.Class("size-4")),
			h.Span(h.Class("app-navlabel"), g.Text("Keluar")),
		),
	)
}
