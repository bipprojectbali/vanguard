package pages

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// auth_login.go — halaman Masuk terpadu (GET /login, satu-satunya jalur auth
// produksi). Desain mengikuti mockup Penpot "Auth Flow — 2. Landing (Sign In /
// Sign Up)": logo+brand, badge internal, headline & subtext produk, tombol
// Google, footnote domain, disclaimer. Form email/password (dev-only) tetap
// ditambahkan di bawah tombol Google bila showPassword.
//
// Satu tombol Google cukup untuk sign-in DAN sign-up: findOrLinkGoogleUser
// (handler/oauth_google.go) auto-membuat user baru bila email belum terdaftar,
// jadi tak perlu form pendaftaran terpisah di produksi — /register hanya
// terdaftar saat devMode (routes.go).

// Login merender halaman masuk terpadu. brand (nama aplikasi) dioper dari
// handler (APP_NAME) — view tetap murni-data, tak membaca config sendiri.
func Login(showPassword bool, errMsg, brand string) g.Node {
	card := []g.Node{
		brandRow(brand),
		internalBadge(),
		h.H1(
			h.Class("text-2xl font-bold leading-snug"),
			g.Text("Kelola relasi pelanggan desa dengan satu platform"),
		),
		h.P(
			h.Class("text-base-content/70"),
			g.Text("Sales, Customer Success, Support & Subscription — semua dalam satu tempat."),
		),
	}
	// errMsg dirender TANPA syarat showPassword: kode ?err=inactive juga bisa
	// datang dari jalur Google (produksi, lihat oauth_google.go) — sebelumnya
	// alert ini tersembunyi di balik gate showPassword, jadi user produksi
	// diarahkan ulang ke /login tanpa penjelasan apa pun (hilang senyap).
	if errMsg != "" {
		card = append(card, ui.Alert(ui.VariantDestructive, "auth-error", g.Text(errMsg)))
	}
	card = append(card,
		googleButton("Lanjutkan dengan Google"),
		h.P(
			h.Class("text-xs text-base-content/60"),
			g.Text("Khusus akun @vanguard-desa.id atau yang diundang workspace"),
		),
	)
	if showPassword {
		card = append(card,
			passwordDivider(),
			passwordFields("/login", "Masuk", false),
			h.P(
				h.Class("text-sm"),
				g.Text("Belum punya akun? "),
				h.A(h.Href("/register"), g.Text("Daftar")),
			),
		)
	}

	return h.Div(
		h.Class("min-h-[70vh] flex flex-col items-center justify-center gap-4 py-8 text-center"),
		h.Div(h.Class("w-full max-w-md"), ui.Card(card...)),
		h.P(
			h.Class("text-xs text-base-content/60 max-w-xs"),
			g.Text("Dengan melanjutkan, Anda menyetujui penggunaan data sesuai kebijakan internal."),
		),
	)
}

// brandRow — logo bulat + nama aplikasi, berjajar tengah (baris atas kartu).
func brandRow(brand string) g.Node {
	return h.Div(
		h.Class("flex items-center justify-center gap-3"),
		h.Div(h.Class("size-10 rounded-full bg-primary shrink-0"), g.Attr("aria-hidden", "true")),
		h.Span(h.Class("text-lg font-bold"), g.Text(brand)),
	)
}

// internalBadge — pil peringatan "hanya untuk internal". Token warning (bukan
// warna absolut) agar tetap adaptif di ke-6 tema (gotcha #11).
func internalBadge() g.Node {
	return h.Div(
		h.Class("flex justify-center"),
		h.Span(h.Class("badge badge-warning font-semibold"), g.Text("INTERNAL USE ONLY")),
	)
}
