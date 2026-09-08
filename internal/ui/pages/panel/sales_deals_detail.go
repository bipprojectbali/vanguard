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

	PrimaryContact string
	Stage          string
	Stages         []string
	// WonSubStatuses (BL-21) = pilihan status langganan awal (Active/Trial) untuk
	// dropdown yang muncul saat memilih Closed Won; diisi handler dari enum
	// autoritatif (validInitialSubStatuses). Kosong → dropdown tak dirender.
	WonSubStatuses []string
	DealType       string
	Amount         string
	// AmountLabel (BL-88) = label baris Nilai, diturunkan handler: "Nilai perkiraan"
	// (pra-quote, manual) atau "Nilai diakui (dari quote)" bila deal punya quote
	// Accepted (Amount = grand_total quote, otoritatif). View murni-data.
	AmountLabel      string
	Probability      string
	ExpectedClose    string
	ForecastCategory string
	NextStep         string
	SubscriptionTerm string

	Competitor    string
	WinLossReason string
	// LossReasonCode (BL-44 3a) = kode alasan kalah terstruktur (picklist), hanya
	// terisi saat Closed Lost. LossReasonCodes = opsi dropdown untuk kontrol stage.
	LossReasonCode  string
	LossReasonCodes []string
	ClosedDate      string
	LossNotes       string

	Owner    string
	CanWrite bool

	// CanCreateQuote (BL-86) = boleh MEMBUAT quote baru: CanWrite DAN stage dalam
	// jendela quotable (Qualification–Negotiation). Digate terpisah dari CanWrite
	// agar tombol "Buat Quote" tak tampil saat aksinya pasti ditolak backend
	// (BL-13). QuoteStageLockMsg = alasan singkat saat di luar jendela (kosong bila
	// boleh), dari handler via stageLockMsg — jangan hitung logika stage di view.
	CanCreateQuote    bool
	QuoteStageLockMsg string

	// Quotes = pratinjau quote deal ini (Modul 4). Diisi handler via
	// ListQuotesForDeal (dibatasi); daftar penuh di /deals/{id}/quotes.
	Quotes []QuoteRow

	// QuotesSummary (BL-18) = ringkasan agregat quote deal ini, di-precompute
	// handler dari SEMUA quote hidup (bukan hanya pratinjau termuat).
	QuotesSummary QuotesSummary

	// Activities = timeline aktivitas deal ini (M7-A). Diisi handler via
	// activitiesTimelineFor (dibatasi activityTimelineLimit baris terbaru).
	Activities ActivityTimelineView

	// Err/Msg (BL-99) = umpan balik PRG untuk aksi di halaman ini (ubah tahap,
	// sunting, hapus). Diisi handler dari ?err/?ok via wsErrMsg/dealsMsg. Kritis
	// khusus tahap terminal: gerbang Closed Won/Lost yang gagal me-redirect ke
	// DETAIL ini dgn ?err — tanpa banner, penolakan tampak seperti "tak tersimpan".
	Err string
	Msg string
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
	amountLabel := v.AmountLabel
	if amountLabel == "" {
		amountLabel = "Nilai"
	}
	accountLink := h.A(
		h.Href(v.Base+"/accounts/"+strconv.FormatInt(v.AccountID, 10)),
		h.Class("link link-hover"), g.Text(orDash(v.AccountLabel)))

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.A(h.Href(v.Base+"/deals"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke pipeline")),
		// BL-99: banner umpan balik PRG (pola sama halaman pipeline). Ditaruh dekat
		// atas agar alasan penolakan tahap terminal langsung terlihat di dekat form.
		ui.When(v.Err != "", ui.Alert(ui.VariantDestructive, "deal-err", g.Text(v.Err))),
		ui.When(v.Msg != "", ui.Alert(ui.VariantDefault, "deal-ok", g.Text(v.Msg))),
		dealStepper(displayStages(v.Stages, v.Stage), v.Stage),
		ui.When(v.CanWrite, dealStageControl(v, base)),
		dealIdentityCard(v, accountLink),
		detailCard("Nilai & Peluang", []detailField{
			{amountLabel, v.Amount},
			{"Probabilitas", prob},
			{"Perkiraan Tutup", v.ExpectedClose},
			{"Kategori Forecast", v.ForecastCategory},
			{"Termin Langganan", v.SubscriptionTerm},
		}),
		detailCard("Hasil & Analisis", []detailField{
			{"Alasan Menang/Kalah", v.WinLossReason},
			{"Kode Alasan Kalah", v.LossReasonCode},
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
		h.Div(h.Class("flex flex-wrap items-center gap-2 min-w-0"),
			h.H2(h.Class("font-semibold"), g.Text("Quote")),
			ui.When(v.QuotesSummary.Total > 0, quotesSummaryBadge(v.QuotesSummary)),
		),
		ui.When(v.CanCreateQuote, h.A(
			h.Href(quotesBase+"/new"), h.Class("btn btn-sm btn-primary min-h-11"),
			g.Text("Buat Quote"))),
		// BL-86: saat boleh tulis tapi stage di luar jendela quotable, tombol
		// disembunyikan; tampilkan hint singkat agar user paham kapan bisa.
		ui.When(v.CanWrite && !v.CanCreateQuote && v.QuoteStageLockMsg != "",
			h.Span(h.Class("text-xs text-base-content/60"),
				g.Text(v.QuoteStageLockMsg))),
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
				h.Span(h.Class("flex flex-wrap items-center gap-2 shrink-0"),
					quoteStatusBadge(q.Status),
					ui.When(q.Expired, quoteExpiredBadge()),
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
// biasa) karena membuka detail desa, sisanya field teks biasa. Lewat cardRows/
// detailRow (accounts_detail_rollup.go) — bukan detailCard yang cuma menerima
// nilai string — karena baris Desa membawa g.Node tautan.
func dealIdentityCard(v DealDetailView, accountLink g.Node) g.Node {
	return cardRows("Identitas Deal", "",
		detailRow("Desa", accountLink),
		detailRow("Kontak Utama", g.Text(orDash(v.PrimaryContact))),
		detailRow("Tipe Deal", g.Text(orDash(v.DealType))),
		detailRow("Langkah Berikutnya", g.Text(orDash(v.NextStep))),
		detailRow("Pemilik", g.Text(orDash(v.Owner))),
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
