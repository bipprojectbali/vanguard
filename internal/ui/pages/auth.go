package pages

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// auth.go — halaman Daftar (dev-only; route-nya tak ada di prod) + helper form
// password/tombol Google dipakai bersama auth_login.go. Login (jalur sign-in/
// sign-up terpadu produksi) desainnya berbeda jauh (kartu brand + headline
// mockup Penpot "Auth Flow — 2. Landing") sehingga dipisah ke auth_login.go.

// Register merender halaman pendaftaran (dev-only; route-nya tak ada di prod).
//
// askWorkspace: apakah pendaftar diminta menamai workspace-nya. HANYA di mode
// multi — di mode single ia bergabung ke aplikasi yang SUDAH ada dan sudah
// bernama, jadi menanyakannya berarti meminta nama yang lalu dibuang. Handler
// pun menolak nama kosong, sehingga field ini bukan cuma mubazir di sana:
// ia membuat pendaftaran MUSTAHIL diselesaikan.
//
// View tetap murni-data (tak memanggil appmode sendiri) — handler yang
// menurunkannya, sesuai konvensi.
func Register(errMsg string, askWorkspace bool) g.Node {
	card := []g.Node{googleButton("Daftar dengan Google")}
	if errMsg != "" {
		card = append(card, ui.Alert(ui.VariantDestructive, "auth-error", g.Text(errMsg)))
	}
	card = append(card, passwordDivider(), passwordFields("/register", "Daftar", askWorkspace))

	return h.Div(
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text("Daftar")),
		ui.Card(card...),
		h.P(
			h.Class("mt-4"),
			g.Text("Sudah punya akun? "),
			h.A(h.Href("/login"), g.Text("Masuk")),
		),
	)
}

// googleButton — tautan penuh (bukan @post) ke flow OAuth. Navigasi biasa 302.
// Logo "super G" 4-warna resmi + teks berbeda per konteks (label diteruskan
// pemanggil; lokalisasi teks diizinkan pedoman Google — hanya logonya yang
// wajib resmi). daisyUI .btn sudah flex + gap, jadi logo & teks otomatis
// berjajar rapi.
func googleButton(label string) g.Node {
	return h.A(
		h.Href("/api/auth/google"),
		h.Class("btn btn-outline w-full"),
		ui.GoogleG(h.Class("size-[18px]")), // 18px = spesifikasi Google
		g.Text(label),
	)
}

// passwordDivider memberi pemisah visual "atau" antara Google dan form password.
func passwordDivider() g.Node {
	return h.P(h.Class("text-center text-sm text-base-content/70"), g.Text("atau"))
}

// passwordFields adalah form email/password NATIVE (dev-only). Submit = POST biasa
// ke action → server balas HTTP 303 (redirect). Input pakai name= (bukan Datastar
// bind); tombol type=submit. showWorkspace menambah field "Nama Workspace" di atas.
func passwordFields(action, submitLabel string, showWorkspace bool) g.Node {
	fields := []g.Node{}
	if showWorkspace {
		fields = append(fields, h.Div(
			h.Class("grid gap-2"),
			ui.Label("Nama Workspace", h.For("workspace")),
			ui.Input(
				h.ID("workspace"), h.Name("workspace"), h.Type("text"),
				h.Placeholder("mis. Acme Corp"),
			),
		))
	}
	fields = append(fields,
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Email", h.For("email")),
			ui.Input(
				h.ID("email"), h.Name("email"), h.Type("email"),
				h.Placeholder("nama@contoh.com"),
			),
		),
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Password", h.For("password")),
			ui.Input(
				h.ID("password"), h.Name("password"), h.Type("password"),
				h.Placeholder("••••••••"),
			),
		),
		h.Button(h.Type("submit"), h.Class("btn btn-primary w-full"), g.Text(submitLabel)),
	)
	// Form native: method POST + action → navigasi 303 (bukan SSE). w-full agar
	// tombol & field mengisi kartu (mobile-first).
	return h.FormEl(
		h.Method("post"), h.Action(action), h.Class("grid gap-4"),
		g.Group(fields),
	)
}
