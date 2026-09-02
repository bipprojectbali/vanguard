package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_deals_detail_stage.go — stepper & kontrol ganti tahap deal (dipisah dari
// sales_deals_detail.go demi batas File Health View/Component). Representasi VISUAL
// pipeline (6 langkah) + kontrol NATIVE POST (gotcha #16); field win/loss & status
// langganan kondisional lewat data-show. Backend tetap penjaga sesungguhnya.

// Label tahap terminal — sumber kebenaran enum tetap dealStageOptions (handler).
// Di sini HANYA untuk representasi VISUAL (BL-12): Closed Won/Closed Lost adalah
// hasil terminal saling-eksklusif (satu deal berakhir di salah satu, bukan lewat
// keduanya), jadi ditampilkan sebagai SATU langkah ke-6 di stepper.
const (
	stageClosedWon  = "Closed Won"
	stageClosedLost = "Closed Lost"
)

// displayStages menurunkan 6 langkah TAMPILAN dari daftar pipeline penuh (7):
// semua tahap aktif (kecuali dua terminal) + satu node terminal. Label node-6
// mengikuti hasil aktual — Closed Won / Closed Lost bila deal sudah tertutup,
// "Ditutup" bila masih terbuka. Sengaja TERPISAH dari dealStageOptions (validasi
// + dropdown + enum DB tetap 7); yang berubah hanya yang dilihat mata.
func displayStages(stages []string, current string) []string {
	out := make([]string, 0, 6)
	for _, s := range stages {
		if s == stageClosedWon || s == stageClosedLost {
			continue
		}
		out = append(out, s)
	}
	terminal := "Ditutup"
	if current == stageClosedWon || current == stageClosedLost {
		terminal = current
	}
	return append(out, terminal)
}

// dealStepper = penanda visual posisi stage di sepanjang pipeline. Discroll dalam
// kontainer sendiri (overflow-x-auto) agar tak meluberkan halaman di mobile.
func dealStepper(stages []string, current string) g.Node {
	items := make([]g.Node, 0, len(stages))
	passed := true
	for _, s := range stages {
		cls := "step"
		if s == current {
			cls += " step-primary"
			passed = false
		} else if passed {
			cls += " step-primary"
		}
		items = append(items, h.Li(h.Class(cls), g.Text(s)))
	}
	return h.Div(
		h.Class("overflow-x-auto min-w-0 pb-1"),
		h.Ul(h.Class("steps steps-horizontal text-xs"), g.Group(items)),
	)
}

// showWhen membungkus node dengan data-show Datastar (BL-12): field kondisional
// muncul/lenyap mengikuti nilai <select> tahap. State form efemeral = ranah sah
// Datastar (unsafe-eval sudah aktif; ekspresi hanya literal enum internal, bukan
// input user). class opsional mempertahankan span grid pembungkus. Backend TETAP
// penjaga sesungguhnya (guard terminal, sales_deals_stage.go) — toggle ini murni
// UX; field tersembunyi tetap terkirim tapi dibersihkan backend saat non-terminal.
func showWhen(expr, class string, node g.Node) g.Node {
	attrs := []g.Node{data.Show(expr)}
	if class != "" {
		attrs = append(attrs, h.Class(class))
	}
	return h.Div(append(attrs, node)...)
}

// selectedOptions membangun opsi <option value=…> untuk sebuah <select> manual:
// tiap nilai jadi satu opsi, dengan `selected` pada yang cocok. Dipakai kontrol
// ganti tahap deal & status quote (select manual yang tak lewat selectField karena
// perlu atribut ekstra seperti data.Bind/ID kustom).
func selectedOptions(values []string, selected string) []g.Node {
	opts := make([]g.Node, 0, len(values))
	for _, s := range values {
		attrs := []g.Node{h.Value(s)}
		if s == selected {
			attrs = append(attrs, h.Selected())
		}
		opts = append(opts, h.Option(append(attrs, g.Text(s))...))
	}
	return opts
}

// dealStageControl = kontrol ganti stage: NATIVE POST (gotcha #16). Menyediakan
// field win/loss reason + catatan kekalahan yang WAJIB diisi backend saat stage
// terminal (Closed Won/Lost) — select-onchange tak bisa mengumpulkannya, maka
// bukan FormPostSelect. Backend tetap penjaga sesungguhnya (validDealStages + guard).
//
// Field kondisional (BL-12): <select> di-bind ke signal $stage (data.Bind) →
// "Alasan Menang/Kalah" tampil saat Closed Won ATAU Closed Lost; "Catatan
// Kekalahan" HANYA saat Closed Lost (selaras backend: loss_notes cuma relevan
// saat kalah). Toggle klien murni UX — validasi & pembersihan tetap di handler.
func dealStageControl(v DealDetailView, base string) g.Node {
	opts := selectedOptions(v.Stages, v.Stage)
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Ubah Tahap")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Tahap Closed Won/Closed Lost wajib menyertakan alasan menang/kalah.")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/stage"),
				data.Signals(map[string]any{"stage": v.Stage}),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0"),
					labelFor("Tahap", "f-stage", true),
					h.Select(
						append([]g.Node{
							h.ID("f-stage"), h.Name("stage"), h.Required(),
							data.Bind("stage"),
							h.Class("select text-base w-full"),
						}, g.Group(opts))...,
					),
				),
				showWhen(
					"$stage == '"+stageClosedWon+"' || $stage == '"+stageClosedLost+"'",
					"min-w-0", field("Alasan Menang/Kalah", "win_loss_reason", v.WinLossReason, false, "text"),
				),
				// BL-21: Closed Won membuat langganan otomatis — user memilih status awalnya
				// (Active/Trial). Default (opsi pertama = Active) selalu terisi → select tetap
				// valid meski tersembunyi di stage lain (tak memblok submit). g.Iff (bukan
				// g.If): argumen g.If dievaluasi eager, jadi WonSubStatuses[0] panic saat slice
				// kosong — bungkus dalam closure agar hanya diakses bila ada opsi.
				g.Iff(len(v.WonSubStatuses) > 0, func() g.Node {
					return showWhen(
						"$stage == '"+stageClosedWon+"'",
						"min-w-0", selectField("Status Langganan Awal", "subscription_status",
							v.WonSubStatuses[0], v.WonSubStatuses, true,
							"Deal menang membuat langganan otomatis untuk desa & paket deal ini."),
					)
				}),
				showWhen(
					"$stage == '"+stageClosedLost+"'",
					"sm:col-span-2 min-w-0", textareaField("Catatan Kekalahan", "loss_notes", v.LossNotes),
				),
				h.Div(
					h.Class("sm:col-span-2"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Tahap")),
				),
			),
		),
	)
}
