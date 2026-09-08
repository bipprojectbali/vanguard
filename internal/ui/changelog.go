package ui

// changelog.go — tombol "Pembaruan" (footer sidebar) + modal daftar perubahan
// per versi. Konten bersumber dari paket internal/changelog (dioper via ShellData,
// konvensi view murni-data). Modal memakai pola ConfirmModal (signal Datastar),
// badge "ada pembaruan" dikelola static/changelog.js (localStorage per-browser).

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"

	"go_starter/internal/changelog"
)

// changelogSignal = signal Datastar boolean pengendali modal Pembaruan.
const changelogSignal = "changelogOpen"

// ChangelogButton = tombol "Pembaruan" di footer sidebar. Klik → buka modal via
// signal. Atribut kait static/changelog.js:
//   - data-changelog-btn: penanda tombol;
//   - data-app-version: versi aktif, dibanding versi terakhir-dilihat (localStorage).
//
// Badge dot dirender dengan kelas `hidden` (tersembunyi default) — JS yang
// menyalakannya bila versi app belum pernah dilihat. Tanpa JS, atau versi sudah
// dilihat, tombol tetap bersih. version "" → tombol tak dirender (tak ada rilis).
func ChangelogButton(version string) g.Node {
	if version == "" {
		return g.Text("")
	}
	return h.Button(
		h.Type("button"),
		// Ikon di toolbar footer, sejajar tombol Tema (btn-ghost btn-sm gap-1).
		// relative → jangkar badge absolut; app-navlabel ikut tersembunyi di rail
		// collapse (konsisten Tema/BL-64), menyisakan ikon Sparkles saja.
		h.Class("btn btn-ghost btn-sm gap-1 relative"),
		g.Attr("data-changelog-btn", "true"),
		g.Attr("data-app-version", version),
		g.Attr("aria-label", "Pembaruan"),
		g.Attr("title", "Pembaruan aplikasi"),
		data.On("click", "$"+changelogSignal+" = true"),
		lucide.Sparkles(h.Class("size-4")),
		h.Span(h.Class("app-navlabel"), g.Text("Pembaruan")),
		// Badge "ada pembaruan" — hidden default; changelog.js buang `hidden`.
		h.Span(
			g.Attr("data-changelog-badge", "true"),
			g.Attr("aria-label", "Ada pembaruan"),
			h.Class("hidden absolute right-2 top-1 size-2 rounded-full bg-primary"),
		),
	)
}

// ChangelogModal = dialog read-only daftar perubahan per versi. Pola sama
// ConfirmModal (signal Datastar $changelogOpen; inline display:none anti-FOUC),
// dengan konten rilis yang bisa di-scroll. Sertakan SEKALI di shell. releases
// kosong → tak dirender.
func ChangelogModal(releases []changelog.Release) g.Node {
	if len(releases) == 0 {
		return g.Text("")
	}
	openExpr := "$" + changelogSignal
	return h.Div(
		h.Class("fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"),
		g.Attr("style", "display:none"),
		data.Show(openExpr),
		data.On("click", openExpr+" = false"), // klik backdrop → tutup
		h.Div(
			// flex-col → header tetap, body scroll. max-w-lg muat di 320px (p-4 luar).
			// max-height via inline style: kelas arbitrer max-h-[80vh] tak ada di
			// app.css prebuilt & binary tailwind tak tersedia untuk make css; pola
			// inline style sudah dipakai di codebase (mis. display:none ConfirmModal).
			h.Class("card bg-base-100 shadow-lg w-full max-w-lg flex flex-col"),
			g.Attr("style", "max-height:80vh"),
			data.On("click", "evt.stopPropagation()"),
			// Header tetap (tak ikut scroll).
			h.Div(
				h.Class("flex items-center justify-between gap-2 px-5 pt-5 pb-3 border-b border-base-300"),
				h.H2(
					h.Class("text-lg font-semibold flex items-center gap-2"),
					lucide.Sparkles(h.Class("size-5 text-primary")),
					g.Text("Pembaruan"),
				),
				h.Button(
					h.Type("button"),
					h.Class("btn btn-ghost btn-sm btn-circle"),
					g.Attr("aria-label", "Tutup"),
					data.On("click", openExpr+" = false"),
					lucide.X(h.Class("size-5")),
				),
			),
			// Body scrollable — daftar rilis (terbaru dulu).
			h.Div(
				h.Class("px-5 py-4 overflow-y-auto flex flex-col gap-6"),
				g.Map(releases, changelogReleaseNode),
			),
		),
	)
}

// changelogReleaseNode merender satu rilis: badge versi + tanggal, ringkasan
// opsional, lalu tiap section.
func changelogReleaseNode(r changelog.Release) g.Node {
	return h.Div(
		h.Class("flex flex-col gap-2"),
		h.Div(
			h.Class("flex items-center gap-2 flex-wrap"),
			h.Span(h.Class("badge badge-primary badge-sm"), g.Textf("v%s", r.Version)),
			h.Span(h.Class("text-xs text-base-content/60"), g.Text(r.Date)),
		),
		g.If(r.Summary != "",
			h.P(h.Class("text-sm text-base-content/70"), g.Text(r.Summary)),
		),
		g.Map(r.Sections, changelogSectionNode),
	)
}

// changelogSectionNode merender satu kelompok perubahan (judul opsional + bullet).
func changelogSectionNode(s changelog.Section) g.Node {
	return h.Div(
		h.Class("flex flex-col gap-1"),
		g.If(s.Title != "",
			h.H3(h.Class("text-sm font-semibold"), g.Text(s.Title)),
		),
		h.Ul(
			h.Class("list-disc pl-5 flex flex-col gap-1 text-sm text-base-content/80"),
			g.Map(s.Items, func(it string) g.Node { return h.Li(g.Text(it)) }),
		),
	)
}
