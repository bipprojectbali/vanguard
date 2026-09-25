package pages

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// auth_login.go — halaman Masuk terpadu (GET /login, satu-satunya jalur auth
// produksi). Desain mengikuti mockup Penpot "Auth Flow — 2. Landing (Sign In /
// Sign Up)": logo+brand, badge internal, headline & subtext produk, tombol
// Google, disclaimer. Tanpa form password — satu tombol Google cukup.
//
// Satu tombol Google cukup untuk sign-in DAN sign-up: findOrLinkGoogleUser
// (handler/oauth_google.go) auto-membuat user baru bila email belum terdaftar,
// jadi tak perlu form pendaftaran terpisah di produksi — /register (dev-only,
// password auth) tetap ada sebagai halaman terpisah, tak ditautkan dari sini
// (routes.go, gate devMode).

// Login merender halaman masuk terpadu. brand (nama aplikasi) dioper dari
// handler (APP_NAME) — view tetap murni-data, tak membaca config sendiri.
func Login(errMsg, brand string) g.Node {
	card := []g.Node{
		brandRow(brand),
		internalBadge(),
		// Headline+subtext dikelompokkan dgn gap sendiri (gap-1, lebih rapat dari
		// gap-6 bawaan card-body) — dua ini satu kesatuan pesan produk, beda dari
		// jarak ke brandRow/badge di atas & tombol/disclaimer di bawah.
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
	}
	// errMsg dirender tanpa syarat: kode ?err=inactive juga bisa datang dari
	// jalur Google (produksi, lihat oauth_google.go), bukan cuma form password.
	if errMsg != "" {
		card = append(card, ui.Alert(ui.VariantDestructive, "auth-error", g.Text(errMsg)))
	}
	card = append(card, googleButton("Lanjutkan dengan Google"))

	return h.Div(
		h.Class("min-h-[70vh] flex flex-col items-center justify-center gap-4 py-8 text-center"),
		h.Div(h.Class("w-full max-w-xl"), ui.Card(card...)),
		h.P(
			h.Class("text-xs text-base-content/60 max-w-xs"),
			g.Text("Dengan melanjutkan, Anda menyetujui penggunaan data sesuai kebijakan internal."),
		),
	)
}

// brandRow — logo bulat (huruf "V") + nama aplikasi, berjajar tengah (baris
// atas kartu). Huruf V dekoratif (bukan singkatan brand yang dioper via
// parameter) — aria-hidden tetap dipasang, nama aplikasi sudah terbaca via
// span brand di sebelahnya.
func brandRow(brand string) g.Node {
	return h.Div(
		h.Class("flex items-center justify-center gap-3"),
		h.Div(
			h.Class("size-10 rounded-full bg-primary shrink-0 flex items-center justify-center"),
			g.Attr("aria-hidden", "true"),
			h.Span(h.Class("text-primary-content font-bold"), g.Text("V")),
		),
		h.Span(h.Class("text-lg font-bold"), g.Text(brand)),
	)
}

// internalBadge — pil peringatan "hanya untuk internal". Token warning (bukan
// warna absolut) agar tetap adaptif di ke-6 tema (gotcha #11). badge-sm agar
// tak bersaing visual dengan brand/headline di atas & bawahnya.
func internalBadge() g.Node {
	return h.Div(
		h.Class("flex justify-center"),
		h.Span(h.Class("badge badge-warning badge-sm font-semibold"), g.Text("INTERNAL USE ONLY")),
	)
}
