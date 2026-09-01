package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"go_starter/internal/ui"
)

// sales_quotes_form.go — form buat/sunting HEADER quote (item disunting di
// builder). Native POST → 303 (gotcha #16); validasi sesungguhnya di backend
// (parseQuoteForm). Reuse formCard/field/textareaField/memberSelect dari
// accounts_form.go. account_id & deal_id TAK di form — diwarisi dari deal induk.
// Pajak TIDAK lagi di header (BL-14) — dikelola di builder (kontrol Pajak) tempat
// subtotal sudah hidup; header hanya identitas quote.

// QuoteFormFields = nilai prefill (edit) atau kosong (buat). Semua string agar
// view netral terhadap tipe DB. PreparedBy = id anggota terpilih (string).
type QuoteFormFields struct {
	QuoteName      string
	ExpirationDate string
	PaymentTerms   string
	NotesTerms     string
	PreparedBy     string
}

// QuoteFormView = data halaman form. Action = URL POST tujuan. IsEdit mengubah
// judul/label. Members = kandidat penyusun (prepared_by), F3-scoped di handler.
type QuoteFormView struct {
	Base   string
	DealID int64
	Action string
	IsEdit bool
	Err    string
	// ExpMin (BL-17) = batas bawah `min` input tanggal kedaluwarsa (hari ini,
	// YYYY-MM-DD, zona aplikasi) — jaring klien agar picker tak menawarkan tanggal
	// lampau. Backend (parseQuoteForm) tetap penjaga sesungguhnya.
	ExpMin  string
	Fields  QuoteFormFields
	Members []AccountMemberOption
}

// QuoteForm merender halaman form lengkap.
func QuoteForm(v QuoteFormView) g.Node {
	dealBase := v.Base + "/deals/" + strconv.FormatInt(v.DealID, 10)
	title := "Buat Quote"
	submit := "Simpan Quote"
	if v.IsEdit {
		title = "Sunting Quote"
		submit = "Simpan Perubahan"
	}

	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(dealBase+"/quotes"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar quote")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "quote-form-err", g.Text(v.Err)))
	}

	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),

		formCard("Identitas Quote",
			field("Nama Quote", "quote_name", v.Fields.QuoteName, false, "text"),
			dateFieldMin("Tanggal Kedaluwarsa", "expiration_date", v.Fields.ExpirationDate, v.ExpMin),
			memberSelect("Disusun oleh", "prepared_by", v.Fields.PreparedBy, v.Members),
		),
		formCard("Syarat & Catatan",
			textareaField("Catatan Pembayaran", "payment_terms", v.Fields.PaymentTerms),
			textareaField("Catatan / Syarat Lainnya", "notes_terms", v.Fields.NotesTerms),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(dealBase+"/quotes"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}
