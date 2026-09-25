package pages

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Landing merender halaman depan publik (route "/"). Dapat diakses SEMUA
// (tak redirect). CTA menyesuaikan status login: user login → tombol ke home
// per-role (homePath), anonim → "Masuk". homePath diabaikan bila anonim.
//
// Disamakan dengan kartu /login (auth_login.go) — brandRow, internalBadge,
// headline+subtext (grup gap-1 yang sama), tombol — termasuk teksnya, supaya
// kedua halaman publik satu bahasa desain & satu pesan produk. Beda dari
// /login: (1) tombol Google diganti CTA sesuai status login (landingCTA),
// tanpa alert error (tak relevan di sini); (2) TANPA disclaimer "Dengan
// melanjutkan..." di bawah kartu — disclaimer itu spesifik konteks login
// (persetujuan saat otentikasi), tak relevan di halaman depan yang bisa
// dilihat siapa saja tanpa aksi. Tak ada mockup Penpot khusus utk halaman
// ini (beda dari /login yang eksplisit ikut board "Auth Flow — 2. Landing").
func Landing(loggedIn bool, homePath, brand string) g.Node {
	return h.Div(
		h.Class("min-h-[70vh] flex flex-col items-center justify-center gap-4 py-8 text-center"),
		h.Div(
			h.Class("w-full max-w-xl"),
			ui.Card(
				brandRow(brand),
				internalBadge(),
				h.Div(
					h.Class("flex flex-col gap-1"),
					h.H1(
						h.Class("text-lg font-bold leading-snug"),
						g.Text("Kelola relasi pelanggan desa dengan satu platform"),
					),
					h.P(
						h.Class("text-base-content/70"),
						g.Text("Sales, Customer Success, Support & Subscription — semua dalam satu tempat."),
					),
				),
				landingCTA(loggedIn, homePath),
			),
		),
	)
}

// landingCTA memilih tombol aksi sesuai status login.
func landingCTA(loggedIn bool, homePath string) g.Node {
	if loggedIn {
		return h.Div(
			h.Class("flex items-center justify-center gap-3"),
			h.A(h.Href(homePath), h.Class("btn btn-primary"), g.Text("Buka aplikasi")),
		)
	}
	// Anonim: arahkan ke /login (Google-only, lihat auth_login.go).
	return h.Div(
		h.Class("flex items-center justify-center gap-3"),
		h.A(h.Href("/login"), h.Class("btn btn-primary"), g.Text("Masuk")),
	)
}
