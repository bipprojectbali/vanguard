package ui

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ThemeOption = satu tema daisyUI yang bisa dipilih user. Label = teks tampil,
// Value = nama tema (harus terdaftar di @plugin daisyui.js di input.css).
type ThemeOption struct {
	Label string
	Value string
	Dark  bool // untuk ikon indikator; tak memengaruhi CSS
}

// themeList = daftar tema kurasi (SATU sumber kebenaran). Harus SELARAS dengan
// blok `themes:` di static/input.css — tema yang tak terdaftar di sana tak
// ter-generate CSS-nya. Campuran terang & gelap agar user punya pilihan nyata.
var themeList = []ThemeOption{
	{Label: "Terang", Value: "light"},
	{Label: "Gelap", Value: "dark", Dark: true},
	{Label: "Cupcake", Value: "cupcake"},
	{Label: "Nord", Value: "nord"},
	{Label: "Dim", Value: "dim", Dark: true},
	{Label: "Sunset", Value: "sunset", Dark: true},
}

// ThemeToggle merender pemilih tema yang buka ke BAWAH (untuk nav atas).
func ThemeToggle() g.Node { return themeDropdown(false) }

// ThemeToggleUp merender pemilih tema yang buka ke ATAS (untuk footer sidebar,
// yang berada di tepi bawah — buka ke bawah akan terpotong).
func ThemeToggleUp() g.Node { return themeDropdown(true) }

// themeDropdown = pemilih tema (dropdown CSS-only via <details>, CSP-safe —
// tanpa inline JS). Tiap opsi = radio ber-class `theme-controller`; daisyUI
// menerapkan tema dari radio yang tercentang lewat CSS murni (`:has()`).
// Persistensi & no-FOUC ditangani theme.js (set data-theme + centang radio yang
// cocok saat load). name grup unik agar radio saling eksklusif.
func themeDropdown(openUp bool) g.Node {
	items := make([]g.Node, 0, len(themeList))
	for _, t := range themeList {
		items = append(items, themeRadioItem(t))
	}
	// Menu pakai base-100 + border → kontras jelas di atas latar base-200
	// maupun sidebar base-100 (shadow + border memberi batas), di semua tema.
	cls := "dropdown dropdown-end"
	menuCls := "dropdown-content menu bg-base-100 border border-base-300 rounded-box z-50 mt-2 w-40 p-2 shadow-lg"
	if openUp {
		cls = "dropdown dropdown-top dropdown-end"
		menuCls = "dropdown-content menu bg-base-100 border border-base-300 rounded-box z-50 mb-2 w-40 p-2 shadow-lg"
	}
	return h.Details(
		h.Class(cls),
		g.Attr("data-theme-dropdown", "true"), // hook theme.js: tutup setelah pilih
		h.Summary(
			// Ikon-saja (btn-square) selaras tombol Pembaruan: baris ikon footer
			// tanpa teks. Label via aria-label/title. (BL-104 revisi)
			h.Class("btn btn-ghost btn-sm btn-square"),
			g.Attr("aria-label", "Ganti tema"),
			g.Attr("title", "Ganti tema"),
			lucide.Palette(h.Class("size-4")),
		),
		h.Ul(
			h.Class(menuCls),
			g.Group(items),
		),
	)
}

// themeRadioItem = satu baris tema dalam dropdown. Radio theme-controller +
// ikon indikator terang/gelap. Value dikirim ke daisyUI (CSS) & disimpan
// theme.js (via listener change → localStorage).
func themeRadioItem(t ThemeOption) g.Node {
	icon := lucide.Sun(h.Class("size-4 opacity-70"))
	if t.Dark {
		icon = lucide.Moon(h.Class("size-4 opacity-70"))
	}
	return h.Li(
		h.Label(
			h.Class("flex items-center gap-2 cursor-pointer"),
			h.Input(
				h.Type("radio"),
				h.Name("theme-choice"),
				h.Class("radio radio-sm theme-controller"),
				h.Value(t.Value),
				g.Attr("aria-label", t.Label),
			),
			icon,
			h.Span(g.Text(t.Label)),
		),
	)
}
