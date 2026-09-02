package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_quotes_detail.go — Quote Builder (detail satu quote). Murni-data: semua
// angka SUDAH diformat handler; unit_price = SNAPSHOT harga saat item dibuat
// (harga beku, acceptance M4). Semua aksi = NATIVE POST → 303 (gotcha #16), BUKAN
// Datastar (navigasi/redirect diblokir CSP). Reuse detailCard/detailField,
// field/selectField/labelFor dari accounts_*/deals_*; quoteStatusBadge dari
// sales_quotes.go. Kontrol tulis hanya muncul bila CanWrite (gate backend tetap
// penjaga sesungguhnya).

// QuoteItemRow = satu baris item siap render. Angka diformat; Quantity/Discount
// mentah dipertahankan untuk prefill form sunting per-item.
type QuoteItemRow struct {
	ID        int64
	PlanLabel string
	Quantity  string
	UnitPrice string
	Discount  string // persen mentah (mis. "10" / ""), untuk prefill
	Subtotal  string
}

// QuotePlanOption = satu opsi picker plan pada form tambah item.
type QuotePlanOption struct {
	ID    int64
	Label string
}

// QuoteDetailView = seluruh data builder siap render. Statuses = opsi kontrol
// status. Plans = katalog aktif untuk tambah item. Subtotal/Tax/GrandTotal SUDAH
// diformat handler.
type QuoteDetailView struct {
	Base       string
	DealID     int64
	ID         int64
	EntityCode string

	QuoteName string
	Status    string
	Statuses  []string

	AccountLabel string
	Expiration   string
	PreparedBy   string
	PaymentTerms string
	NotesTerms   string

	Subtotal   string
	Tax        string
	GrandTotal string

	// Pajak builder (BL-14). TaxMode = mode aktif ('percent'/'amount') → nilai awal
	// select & signal Datastar. TaxRateInput/TaxAmountInput = prefill input tiap mode
	// (desimal sudah dirapikan handler). TaxLabel = label baris ringkasan pajak
	// ("Pajak" atau "Pajak (11%)"). Semua di-precompute handler (view murni-data).
	TaxMode        string
	TaxRateInput   string
	TaxAmountInput string
	TaxLabel       string

	Items []QuoteItemRow
	Plans []QuotePlanOption

	CanWrite bool
	// Quotable (BL-13) = deal di jendela quoting (Qualification–Negotiation) →
	// mutasi diizinkan. Di luar itu builder READ-ONLY (arsip): kontrol tulis
	// disembunyikan, StageLockMsg jadi banner. Flag di-precompute handler.
	Quotable     bool
	StageLockMsg string
	// Expired (BL-17) = penanda kedaluwarsa computed-on-read (Draft dikecualikan),
	// di-precompute handler. Badge muncul di baris "Kedaluwarsa" kartu identitas —
	// terpisah dari Status (bisa kedaluwarsa walau status belum diubah manual).
	Expired bool
}

// CanMutate = boleh mengubah quote/item (punya izin tulis DAN deal di jendela
// quoting). Gerbang tunggal untuk semua kontrol tulis builder (BL-13); backend
// tetap penjaga sesungguhnya.
func (v QuoteDetailView) CanMutate() bool { return v.CanWrite && v.Quotable }

// quoteIdentityCard = kartu "Identitas Quote". Sebagian besar baris teks polos
// (reuse detailRow/cardRows), tetapi baris "Kedaluwarsa" bisa membawa badge
// penanda kedaluwarsa (BL-17) → tak lewat detailCard (khusus teks). Draft &
// tanggal sah tak menampilkan badge (v.Expired sudah di-precompute handler).
func quoteIdentityCard(v QuoteDetailView) g.Node {
	expVal := g.Node(g.Text(orDash(v.Expiration)))
	if v.Expired {
		expVal = h.Span(h.Class("flex flex-wrap items-center gap-2"),
			g.Text(orDash(v.Expiration)), quoteExpiredBadge())
	}
	return cardRows("Identitas Quote", "",
		detailRow("Desa", g.Text(orDash(v.AccountLabel))),
		detailRow("Kedaluwarsa", expVal),
		detailRow("Disusun oleh", g.Text(orDash(v.PreparedBy))),
		detailRow("Catatan Pembayaran", g.Text(orDash(v.PaymentTerms))),
		detailRow("Catatan / Syarat Lainnya", g.Text(orDash(v.NotesTerms))),
	)
}

// QuoteDetail merender builder: header (nama+kode+status+aksi), kartu identitas,
// tabel line items + total, lalu (bila boleh tulis) kelola item, tambah item, &
// kontrol status.
func QuoteDetail(v QuoteDetailView) g.Node {
	dealBase := v.Base + "/deals/" + strconv.FormatInt(v.DealID, 10)
	quoteBase := dealBase + "/quotes/" + strconv.FormatInt(v.ID, 10)

	title := v.QuoteName
	if title == "" {
		title = v.EntityCode
	}
	if title == "" {
		title = "Quote"
	}

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(title)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				ui.When(v.EntityCode != "", h.Span(
					h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
				quoteStatusBadge(v.Status),
			),
		),
		ui.When(v.CanMutate(), h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(quoteBase+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			deleteQuoteForm(quoteBase),
		)),
	)

	body := []g.Node{
		header,
		h.A(h.Href(dealBase+"/quotes"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar quote")),
	}
	// BL-13: pemegang tulis yang diblokir stage diberi alasan (read-only jujur).
	if v.CanWrite && !v.Quotable && v.StageLockMsg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "quote-lock", g.Text(v.StageLockMsg)))
	}
	body = append(body,
		quoteIdentityCard(v),
		quoteLineItems(v, quoteBase),
	)
	if v.CanMutate() {
		body = append(body,
			quoteAddItemForm(v, quoteBase),
			quoteTaxControl(v, quoteBase),
			quoteStatusControl(v, quoteBase),
		)
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// quoteAddItemForm = form tambah item: picker plan (harga di-SNAPSHOT saat submit)
// + qty + diskon. Native POST. Kosong bila katalog plan aktif kosong (tak ada yang
// bisa dijual) → keterangan jujur, bukan form mati.
func quoteAddItemForm(v QuoteDetailView, quoteBase string) g.Node {
	if len(v.Plans) == 0 {
		return h.Div(
			h.Class("card bg-base-100 border border-dashed border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0"),
				h.H2(h.Class("font-semibold mb-1"), g.Text("Tambah Item")),
				h.P(h.Class("text-sm text-base-content/60"),
					g.Text("Belum ada plan aktif di katalog untuk ditambahkan."))),
		)
	}
	placeholder := []g.Node{h.Value(""), h.Disabled(), h.Selected(), g.Text("— Pilih plan —")}
	opts := []g.Node{h.Option(placeholder...)}
	for _, p := range v.Plans {
		opts = append(opts, h.Option(h.Value(strconv.FormatInt(p.ID, 10)), g.Text(p.Label)))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Tambah Item")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Harga satuan dibekukan dari plan saat item ditambahkan.")),
			h.FormEl(
				h.Method("post"), h.Action(quoteBase+"/items"),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0 sm:col-span-2"),
					labelFor("Plan", "f-plan_id", true),
					h.Select(
						append([]g.Node{
							h.ID("f-plan_id"), h.Name("plan_id"), h.Required(),
							h.Class("select text-base w-full"),
						}, g.Group(opts))...,
					),
				),
				field("Kuantitas", "quantity", "1", true, "number"),
				field("Diskon (%)", "discount_pct", "", false, "number"),
				h.Div(
					h.Class("sm:col-span-2"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Tambah Item")),
				),
			),
		),
	)
}

// quoteStatusControl = kontrol ganti status manual (approval flow ditunda). Native
// POST; backend memvalidasi terhadap allowlist skema.
func quoteStatusControl(v QuoteDetailView, quoteBase string) g.Node {
	opts := selectedOptions(v.Statuses, v.Status)
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Ubah Status")),
			h.FormEl(
				h.Method("post"), h.Action(quoteBase+"/status"),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0"),
					labelFor("Status", "f-quote_status", true),
					h.Select(
						append([]g.Node{
							h.ID("f-quote_status"), h.Name("quote_status"), h.Required(),
							h.Class("select text-base w-full"),
						}, g.Group(opts))...,
					),
				),
				h.Div(
					h.Class("flex items-end"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Status")),
				),
			),
		),
	)
}

// quoteTaxControl = kontrol pajak builder (BL-14). Toggle mode Persentase/Nominal +
// satu input yang relevan. <select> di-bind signal $taxmode (data.Bind) → input Tarif
// (%) tampil saat 'percent', Nominal (Rp) saat 'amount' (showWhen = data.Show). KEDUA
// input selalu terkirim (pola dealStageControl BL-12); backend baca hanya yang cocok
// mode & bersihkan sisanya. Native POST → 303 (gotcha #16). Nilai mode = literal
// cermin taxModePercent/taxModeAmount di handler (paket berbeda, tak bisa impor const).
func quoteTaxControl(v QuoteDetailView, quoteBase string) g.Node {
	modeOpt := func(val, label string) g.Node {
		attrs := []g.Node{h.Value(val)}
		if val == v.TaxMode {
			attrs = append(attrs, h.Selected())
		}
		return h.Option(append(attrs, g.Text(label))...)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Pajak")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Persentase mengikuti subtotal otomatis (mis. PPN 11%). "+
					"Nominal = nilai rupiah tetap (mis. materai).")),
			h.FormEl(
				h.Method("post"), h.Action(quoteBase+"/tax"),
				data.Signals(map[string]any{"taxmode": v.TaxMode}),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0"),
					labelFor("Jenis Pajak", "f-tax_mode", true),
					h.Select(
						h.ID("f-tax_mode"), h.Name("tax_mode"), h.Required(),
						data.Bind("taxmode"),
						h.Class("select text-base w-full"),
						modeOpt("percent", "Persentase (%)"),
						modeOpt("amount", "Nominal (Rp)"),
					),
				),
				showWhen("$taxmode == 'percent'", "min-w-0",
					field("Tarif Pajak (%)", "tax_rate", v.TaxRateInput, false, "number")),
				showWhen("$taxmode == 'amount'", "min-w-0",
					field("Nominal Pajak (Rp)", "tax_amount", v.TaxAmountInput, false, "number")),
				h.Div(
					h.Class("sm:col-span-2"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Pajak")),
				),
			),
		),
	)
}

// deleteQuoteForm = tombol hapus quote (soft-delete). Native POST → 303 (gotcha #16).
func deleteQuoteForm(quoteBase string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(quoteBase+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text("Hapus")),
	)
}
