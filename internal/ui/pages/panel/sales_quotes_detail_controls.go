package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_quotes_detail_controls.go — kontrol tulis Quote Builder (dipisah dari
// sales_quotes_detail.go demi batas File Health View/Component). Semua aksi =
// NATIVE POST → 303 (gotcha #16); hanya dirender bila CanWrite (gate backend
// tetap penjaga sesungguhnya).

// quoteStatusControl = kontrol ganti status manual (approval flow ditunda). Native
// POST; backend memvalidasi terhadap allowlist skema. BL-148: quote tanpa line item
// hanya boleh Draft → opsi dibatasi ke Draft saja + catatan (backend QuoteStatus =
// penjaga sesungguhnya; "Draft" literal = cermin quoteInitialStatus di handler,
// paket berbeda tak bisa impor const).
func quoteStatusControl(v QuoteDetailView, quoteBase string) g.Node {
	statuses := v.Statuses
	if !v.HasItems {
		statuses = []string{"Draft"}
	}
	opts := selectedOptions(statuses, v.Status)
	fields := []g.Node{
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
	}
	body := []g.Node{
		h.H2(h.Class("font-semibold"), g.Text("Ubah Status")),
		h.FormEl(
			append([]g.Node{
				h.Method("post"), h.Action(quoteBase + "/status"),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
			}, fields...)...,
		),
	}
	if !v.HasItems {
		body = append(body, h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Tambahkan minimal satu item sebelum memajukan status dari Draft.")))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(append([]g.Node{h.Class("card-body min-w-0 gap-3")}, body...)...),
	)
}

// quoteTaxBody = isi modal Pajak (BL-70; kontrol pajak builder BL-14). Toggle mode
// Persentase/Nominal + satu input yang relevan. <select> di-bind signal $taxmode
// (data.Bind) → input Tarif (%) tampil saat 'percent', Nominal (Rp) saat 'amount'
// (showWhen = data.Show). KEDUA input selalu terkirim (pola dealStageControl BL-12);
// backend baca hanya yang cocok mode & bersihkan sisanya. Native POST → 303
// (gotcha #16); modal hanya WADAH — form tak berubah. Nilai mode = literal cermin
// taxModePercent/taxModeAmount di handler (paket berbeda, tak bisa impor const).
// $taxmode = toggle FIELD (bukan mekanisme modal): modal buka/tutup via
// checkbox-toggle CSS (modalDialog) — dua concern beda, tak dicampur.
func quoteTaxBody(v QuoteDetailView, quoteBase string) g.Node {
	modeOpt := func(val, label string) g.Node {
		attrs := []g.Node{h.Value(val)}
		if val == v.TaxMode {
			attrs = append(attrs, h.Selected())
		}
		return h.Option(append(attrs, g.Text(label))...)
	}
	return h.Div(
		h.Class("grid gap-3 min-w-0"),
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
			// BL-147: Nominal Pajak = uang rupiah bulat → moneyFieldRp (afiks "Rp" +
			// data-numgroup) selaras Nilai Deal/Estimasi Lead, bukan type="number"
			// telanjang. numgroup.js mengelompokkan ribuan & menormalkan jadi digit
			// polos saat submit; backend cleanThousands penjaga bila JS mati.
			showWhen("$taxmode == 'amount'", "min-w-0",
				moneyFieldRp("Nominal Pajak", "tax_amount", v.TaxAmountInput)),
			h.Div(
				h.Class("sm:col-span-2"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
					g.Text("Simpan Pajak")),
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
