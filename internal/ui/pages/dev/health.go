package dev

import (
	"sort"
	"strconv"
	"strings"

	"go_starter/internal/health"
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// HealthPage merender ringkasan + toolbar filter + tabel + pagination. Filter,
// pagination, dan copy dikerjakan client-side oleh /static/health.js (dataset
// kecil, semua baris dikirim sekali). Baris membawa atribut data-* yang dibaca JS.
func HealthPage(res health.Result) g.Node {
	summary := "Semua " + strconv.Itoa(res.Total) + " file sehat ✓"
	summaryVariant := ""
	if res.Unhealthy > 0 {
		summary = strconv.Itoa(res.Unhealthy) + " dari " + strconv.Itoa(res.Total) + " file perlu perhatian"
		summaryVariant = "error"
	}

	return h.Div(
		h.ID("health-panel"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("File Health")),
		h.P(h.Class("text-sm text-base-content/70 mb-4"),
			g.Text("Ambang: per-tipe (handler 150, service 300, dst) + hard limit 500 baris / 20.000 karakter. Hanya dev.")),
		healthSummary(summary, summaryVariant),
		healthToolbar(res),
		h.Div(
			h.Class("card bg-base-100 border border-base-300 mt-4 min-w-0"),
			h.Div(
				h.Class("card-body min-w-0"),
				ui.TableScroll(h.Table(
					h.Class("w-full text-sm"),
					h.THead(
						h.Tr(
							h.Class("border-b border-base-300 text-left text-base-content/70"),
							h.Th(h.Class("py-2 pr-4 w-8"),
								h.Input(h.Type("checkbox"), h.ID("health-select-all"), h.Title("Pilih halaman ini")),
							),
							th("File"), th("Tipe"), th("Baris"), th("Karakter"), th("Status"),
						),
					),
					h.TBody(g.Map(res.Reports, healthRow)),
				)),
			),
		),
		healthPagination(),
		// Slot toast copy (kanan-bawah) — komponen bersama BL-156a. Kosong
		// saat render awal (ToastSlot, BUKAN Toast — cegah animasi fade
		// kosong terpicu tanpa isi); health.js mengisi elemen .alert saat
		// copy path terjadi (lihat flash() di static/health.js).
		ui.ToastSlot("health-toast"),
		// Logika filter/pagination/copy (same-origin, patuh CSP script-src 'self').
		h.Script(h.Src("/static/health.js"), h.Defer()),
	)
}

func healthSummary(msg, variant string) g.Node {
	cls := "alert"
	if variant == "error" {
		cls = "alert alert-error"
	}
	return h.Div(h.Class(cls), h.Role("status"), g.Text(msg))
}

// healthToolbar = kontrol filter (search + status + tipe) & tombol copy.
func healthToolbar(res health.Result) g.Node {
	return h.Div(
		h.Class("mt-4 flex flex-wrap items-center gap-2"),
		h.Input(
			h.Type("search"),
			h.ID("health-q"),
			h.Class("input input-sm max-w-xs"),
			h.Placeholder("Cari nama/path file…"),
		),
		h.Select(
			h.ID("health-status"),
			h.Class("select select-sm w-auto"),
			h.Option(h.Value(""), g.Text("Semua status")),
			h.Option(h.Value("unhealthy"), g.Text("Perlu perhatian")),
			h.Option(h.Value("healthy"), g.Text("Sehat")),
		),
		kindSelect(res),
		h.Div(h.Class("flex-1")), // spacer
		h.Button(
			h.Type("button"), h.ID("health-copy-selected"),
			h.Class("btn btn-outline btn-sm"),
			g.Text("Copy terpilih"),
		),
		h.Button(
			h.Type("button"), h.ID("health-copy-unhealthy"),
			h.Class("btn btn-outline btn-sm"),
			g.Text("Copy bermasalah"),
		),
	)
}

// kindSelect membangun dropdown tipe dari kind unik yang ada di hasil scan.
func kindSelect(res health.Result) g.Node {
	seen := map[string]bool{}
	var kinds []string
	for _, r := range res.Reports {
		if !seen[r.Kind] {
			seen[r.Kind] = true
			kinds = append(kinds, r.Kind)
		}
	}
	sort.Strings(kinds)
	opts := []g.Node{h.Option(h.Value(""), g.Text("Semua tipe"))}
	for _, k := range kinds {
		opts = append(opts, h.Option(h.Value(k), g.Text(k)))
	}
	return h.Select(append([]g.Node{h.ID("health-kind"), h.Class("select select-sm w-auto")}, opts...)...)
}

func healthPagination() g.Node {
	return h.Div(
		h.Class("mt-3 flex flex-wrap items-center justify-between gap-2 text-sm"),
		h.Span(h.ID("health-page-info"), h.Class("text-base-content/70")),
		// flex-wrap di BARIS TOMBOL, bukan hanya di wrapper luar: prev + angka
		// halaman + next tak muat satu baris di 375px dan akan mendorong lebar
		// halaman bila tak boleh membungkus (konvensi mobile-first).
		h.Div(
			h.Class("flex flex-wrap items-center gap-1"),
			h.Button(h.Type("button"), h.ID("health-prev"),
				h.Class("btn btn-outline btn-sm"),
				g.Text("Sebelumnya")),
			// Tombol angka halaman — diisi client-side (jumlah halaman berubah
			// saat filter aktif, jadi tak bisa di-render server).
			h.Div(h.ID("health-pages"), h.Class("flex flex-wrap items-center gap-1")),
			h.Button(h.Type("button"), h.ID("health-next"),
				h.Class("btn btn-outline btn-sm"),
				g.Text("Berikutnya")),
		),
	)
}

func healthRow(r health.Report) g.Node {
	status := "healthy"
	if !r.Healthy {
		status = "unhealthy"
	}
	attrs := []g.Node{
		h.Class("border-b border-base-300"),
		g.Attr("data-health-row", "true"),
		g.Attr("data-path", r.Path),
		g.Attr("data-status", status),
		g.Attr("data-kind", r.Kind),
	}
	if !r.Healthy {
		attrs = append(attrs, g.Attr("style", "box-shadow: inset 3px 0 0 var(--color-error, #ef4444)"))
	}
	lineCell := strconv.Itoa(r.Lines) + " / " + strconv.Itoa(r.Limit)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4"),
			h.Input(h.Type("checkbox"), g.Attr("data-health-check", "true"), h.Title("Pilih file ini")),
		),
		h.Td(h.Class("py-2 pr-4 font-mono text-xs"), g.Text(r.Path)),
		h.Td(h.Class("py-2 pr-4 text-base-content/70"), g.Text(r.Kind)),
		h.Td(h.Class("py-2 pr-4"), g.Text(lineCell)),
		h.Td(h.Class("py-2 pr-4"), g.Text(strconv.Itoa(r.Chars))),
		h.Td(h.Class("py-2"), healthStatus(r)),
	}
	return h.Tr(append(attrs, cells...)...)
}

func healthStatus(r health.Report) g.Node {
	if r.Healthy {
		return badge("sehat", "")
	}
	return h.Span(
		h.Class("badge badge-error"),
		h.Title(strings.Join(r.Reasons, "; ")),
		g.Text("perlu perhatian"),
	)
}
