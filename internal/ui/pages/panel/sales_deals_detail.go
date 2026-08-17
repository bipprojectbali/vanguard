package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_deals_detail.go — halaman detail satu deal. Murni-data: Amount SUDAH
// disamarkan handler (F4). Kolom kosong → "—". Reuse detailCard/detailField dari
// accounts_detail.go & dealStageBadge dari sales_deals.go. Ganti stage = kontrol
// aksi NATIVE POST (gotcha #16), BUKAN drag-drop/FormPostSelect: win/loss reason
// butuh field teks yang select-onchange tak bisa kumpulkan (keputusan terkunci #1).

// DealDetailView = seluruh data satu deal siap render. Stages = urutan pipeline
// (untuk stepper + pilihan ganti stage). AccountLabel/PrimaryContact sudah
// diresolusi handler (best-effort, label cadangan bila di luar tenant/terhapus).
type DealDetailView struct {
	Base       string
	ID         int64
	EntityCode string

	DealName     string
	AccountID    int64
	AccountLabel string

	PrimaryContact   string
	Stage            string
	Stages           []string
	DealType         string
	Amount           string
	Probability      string
	ExpectedClose    string
	ForecastCategory string
	NextStep         string
	SubscriptionTerm string

	Competitor    string
	WinLossReason string
	ClosedDate    string
	LossNotes     string

	Owner    string
	CanWrite bool

	// Quotes = pratinjau quote deal ini (Modul 4). Diisi handler via
	// ListQuotesForDeal (dibatasi); daftar penuh di /deals/{id}/quotes.
	Quotes []QuoteRow

	// Activities = timeline aktivitas deal ini (M7-A). Diisi handler via
	// activitiesTimelineFor (dibatasi activityTimelineLimit baris terbaru).
	Activities ActivityTimelineView
}

// DealDetail merender hub detail: header (nama + kode + stage + aksi), stepper
// pipeline, kontrol ganti stage (bila boleh tulis), kartu inti & analisis, lalu
// placeholder lintas-modul (Activities & Modul 5).
func DealDetail(v DealDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	base := v.Base + "/deals/" + idStr

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.DealName)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				ui.When(v.EntityCode != "", h.Span(
					h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
				dealStageBadge(v.Stage),
			),
		),
		ui.When(v.CanWrite, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(base+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			deleteDealForm(base),
		)),
	)

	prob := v.Probability
	if prob != "" {
		prob += "%"
	}
	accountLink := h.A(
		h.Href(v.Base+"/accounts/"+strconv.FormatInt(v.AccountID, 10)),
		h.Class("link link-hover"), g.Text(orDash(v.AccountLabel)))

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.A(h.Href(v.Base+"/deals"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke pipeline")),
		dealStepper(v.Stages, v.Stage),
		ui.When(v.CanWrite, dealStageControl(v, base)),
		dealIdentityCard(v, accountLink),
		detailCard("Nilai & Peluang", []detailField{
			{"Nilai", v.Amount},
			{"Probabilitas", prob},
			{"Perkiraan Tutup", v.ExpectedClose},
			{"Kategori Forecast", v.ForecastCategory},
			{"Termin Langganan", v.SubscriptionTerm},
		}),
		detailCard("Hasil & Analisis", []detailField{
			{"Alasan Menang/Kalah", v.WinLossReason},
			{"Kompetitor", v.Competitor},
			{"Tanggal Tutup", v.ClosedDate},
			{"Catatan Kekalahan", v.LossNotes},
		}),
		dealQuotesCard(v),
		ActivityTimeline(v.Activities),
	)
}

// dealQuotesCard = pratinjau Quote deal ini (Modul 4). Daftar ringkas + tautan
// ke builder/daftar penuh. Tombol "Buat Quote" hanya bila boleh tulis. Quote
// hidup DI BAWAH deal (nested) — semua tautan lewat base deal.
func dealQuotesCard(v DealDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	quotesBase := v.Base + "/deals/" + idStr + "/quotes"

	head := h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
		h.H2(h.Class("font-semibold"), g.Text("Quote")),
		ui.When(v.CanWrite, h.A(
			h.Href(quotesBase+"/new"), h.Class("btn btn-sm btn-primary min-h-11"),
			g.Text("Buat Quote"))),
	)

	var content g.Node
	if len(v.Quotes) == 0 {
		content = h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Belum ada quote untuk deal ini."))
	} else {
		rows := make([]g.Node, 0, len(v.Quotes))
		for _, q := range v.Quotes {
			href := quotesBase + "/" + strconv.FormatInt(q.ID, 10)
			rows = append(rows, h.A(
				h.Href(href),
				h.Class("flex flex-wrap items-center justify-between gap-2 py-2 "+
					"border-b border-base-300/50 last:border-0 hover:bg-base-200/50"),
				h.Span(h.Class("min-w-0 truncate font-medium"),
					g.Text(quoteRowLabel(q))),
				h.Span(h.Class("flex items-center gap-2 shrink-0"),
					quoteStatusBadge(q.Status),
					h.Span(h.Class("text-sm text-base-content/70"), g.Text(orDash(q.GrandTotal)))),
			))
		}
		content = h.Div(h.Class("min-w-0"),
			h.Div(h.Class("grid"), g.Group(rows)),
			h.A(h.Href(quotesBase), h.Class("text-sm text-base-content/60 mt-2 inline-block"),
				g.Text("Lihat semua quote »")),
		)
	}

	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body min-w-0"), head, content),
	)
}

// quoteRowLabel = label ringkas satu quote di kartu deal: nama bila ada, jatuh ke
// kode, lalu id.
func quoteRowLabel(q QuoteRow) string {
	if q.QuoteName != "" {
		return q.QuoteName
	}
	if q.EntityCode != "" {
		return q.EntityCode
	}
	return "Quote #" + strconv.FormatInt(q.ID, 10)
}

// dealIdentityCard = kartu inti; nilai Desa dirender sebagai TAUTAN (bukan teks
// biasa) karena membuka detail desa, sisanya field biasa via detailCard.
func dealIdentityCard(v DealDetailView, accountLink g.Node) g.Node {
	row := func(label string, value g.Node) g.Node {
		return h.Div(
			h.Class("grid gap-1 sm:grid-cols-3 sm:gap-2 py-2 border-b border-base-300/50 last:border-0"),
			h.Dt(h.Class("text-sm text-base-content/60"), g.Text(label)),
			h.Dd(h.Class("sm:col-span-2 break-words"), value),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Identitas Deal")),
			h.Dl(
				h.Class("min-w-0"),
				row("Desa", accountLink),
				row("Kontak Utama", g.Text(orDash(v.PrimaryContact))),
				row("Tipe Deal", g.Text(orDash(v.DealType))),
				row("Langkah Berikutnya", g.Text(orDash(v.NextStep))),
				row("Pemilik", g.Text(orDash(v.Owner))),
			),
		),
	)
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

// dealStageControl = kontrol ganti stage: NATIVE POST (gotcha #16). Menyediakan
// field win/loss reason + catatan kekalahan yang WAJIB diisi backend saat stage
// terminal (Closed Won/Lost) — select-onchange tak bisa mengumpulkannya, maka
// bukan FormPostSelect. Backend tetap penjaga sesungguhnya (validDealStages + guard).
func dealStageControl(v DealDetailView, base string) g.Node {
	opts := make([]g.Node, 0, len(v.Stages))
	for _, s := range v.Stages {
		attrs := []g.Node{h.Value(s)}
		if s == v.Stage {
			attrs = append(attrs, h.Selected())
		}
		opts = append(opts, h.Option(append(attrs, g.Text(s))...))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Ubah Tahap")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Tahap Closed Won/Closed Lost wajib menyertakan alasan menang/kalah.")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/stage"),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0"),
					labelFor("Tahap", "f-stage", true),
					h.Select(
						append([]g.Node{
							h.ID("f-stage"), h.Name("stage"), h.Required(),
							h.Class("select text-base w-full"),
						}, g.Group(opts))...,
					),
				),
				field("Alasan Menang/Kalah", "win_loss_reason", v.WinLossReason, false, "text"),
				textareaField("Catatan Kekalahan", "loss_notes", v.LossNotes),
				h.Div(
					h.Class("sm:col-span-2"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Tahap")),
				),
			),
		),
	)
}


// deleteDealForm = tombol hapus (soft-delete). Form NATIVE POST → 303 (gotcha #16).
func deleteDealForm(base string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text("Hapus")),
	)
}
