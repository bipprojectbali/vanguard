package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_convert.go — halaman review konversi Lead → Desa + Kontak + Deal. Satu
// form NATIVE POST → 303 (gotcha #16) yang membuat TIGA entitas sekaligus. Semua
// nilai pra-isi dari lead (handler), semua bisa disunting sebelum submit. Reuse
// helper formCard/field/selectField dari accounts_form.go (satu paket panel).
//
// Nomor telepon: bila PhoneEditable=false (bukan Sales), field dikunci &
// menampilkan mask TANPA name → tak ikut ter-submit; handler menyalin nomor asli
// lead server-side (F4). Sama pola dengan phoneField accounts.

// ConvertFormFields = nilai pra-isi review (dari lead). Semua string agar view
// netral terhadap tipe DB.
type ConvertFormFields struct {
	VillageName string
	AccountType string
	DistrictID  string

	FirstName   string
	LastName    string
	JobTitle    string
	MobilePhone string
	Whatsapp    string
	Email       string

	DealName string
	Amount   string
}

// DuplicateCandidate = satu desa lain di tenant yang sama dgn nama yang mirip
// (case-insensitive) dgn desa yang akan dibuat oleh konversi ini. Ditautkan ke
// account yang sudah ada agar pengguna bisa memeriksa sebelum lanjut.
type DuplicateCandidate struct {
	AccountID   int64
	EntityCode  string
	VillageName string
	RegionLabel string
}

// LeadConvertView = data halaman review. Action = URL POST konversi. BackURL =
// kembali ke detail lead. LeadName/LeadCode untuk header konteks. Duplicates =
// kandidat desa duplikat (soft-warning, non-blocking — lihat sales_convert.go
// handler & docs/crm/tasks.md M4-6).
type LeadConvertView struct {
	Base    string
	Action  string
	BackURL string
	Err     string

	LeadName string
	LeadCode string

	PhoneEditable bool
	AccountTypes  []string
	Fields        ConvertFormFields
	Duplicates    []DuplicateCandidate

	// RegionsJSON = dataset penuh master wilayah (h.regionsJSON), diembed sekali
	// utk cascading dropdown Provinsi/Kabupaten-Kota/Kecamatan (ADR 0009).
	RegionsJSON string
}

// LeadConvert merender halaman review lengkap: header konteks, banner penjelasan,
// lalu satu form 3-kartu (Desa, Kontak Utama, Deal) + tombol konversi.
func LeadConvert(v LeadConvertView) g.Node {
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Konversi Lead")),
			h.A(h.Href(v.BackURL), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke detail lead")),
		),
		h.Div(
			h.Class("card bg-base-100 border border-success/40 min-w-0"),
			h.Div(h.Class("card-body"),
				h.P(h.Class("text-sm"),
					g.Text("Lead "),
					h.Span(h.Class("font-semibold"), g.Text(v.LeadName)),
					ui.When(v.LeadCode != "", g.Group([]g.Node{
						g.Text(" ("),
						h.Span(h.Class("font-mono"), g.Text(v.LeadCode)),
						g.Text(")"),
					})),
					g.Text(" akan menjadi Desa + Kontak utama + Deal baru. "+
						"Periksa & sunting nilai di bawah sebelum mengonversi.")),
			),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "convert-err", g.Text(v.Err)))
	}
	if len(v.Duplicates) > 0 {
		body = append(body, duplicateWarning(v.Base, v.Duplicates))
	}

	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),

		formCard("Desa (Account)",
			field("Nama Desa", "village_name", v.Fields.VillageName, true, "text"),
			selectField("Tipe Akun", "account_type", v.Fields.AccountType, v.AccountTypes, true),
			regionSelect("convert", v.RegionsJSON, v.Fields.DistrictID),
		),
		formCard("Kontak Utama",
			field("Nama Depan", "first_name", v.Fields.FirstName, true, "text"),
			field("Nama Belakang", "last_name", v.Fields.LastName, false, "text"),
			field("Jabatan", "job_title", v.Fields.JobTitle, false, "text"),
			convertPhoneField("HP", "mobile_phone", v.Fields.MobilePhone, v.PhoneEditable),
			convertPhoneField("WhatsApp", "whatsapp_number", v.Fields.Whatsapp, v.PhoneEditable),
			field("Email", "email", v.Fields.Email, false, "email"),
		),
		formCard("Deal",
			field("Nama Deal", "deal_name", v.Fields.DealName, true, "text"),
			field("Nilai (Rp)", "amount", v.Fields.Amount, false, "text"),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-success min-h-11"),
				g.Text("Konversi Sekarang")),
			h.A(h.Href(v.BackURL), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	// Cascading dropdown wilayah (regionSelect di atas cuma menanam data + markup;
	// interaksi berjenjangnya di sini, same-origin CSP-safe, gotcha #12).
	body = append(body, h.Script(h.Src("/static/regions.js"), h.Defer()))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// duplicateWarning merender banner SOFT-WARNING (bukan hard block — nama desa
// sama bisa valid beda dusun/kabupaten): daftar desa lain dgn nama serupa,
// masing-masing ditautkan ke detail account agar bisa diperiksa sebelum lanjut.
func duplicateWarning(base string, dupes []DuplicateCandidate) g.Node {
	items := make([]g.Node, 0, len(dupes))
	for _, d := range dupes {
		items = append(items, h.Li(
			h.A(
				h.Href(base+"/accounts/"+strconv.FormatInt(d.AccountID, 10)),
				h.Class("link link-hover font-medium"),
				g.Text(d.VillageName),
			),
			ui.When(d.EntityCode != "", g.Group([]g.Node{
				g.Text(" ("),
				h.Span(h.Class("font-mono"), g.Text(d.EntityCode)),
				g.Text(")"),
			})),
			ui.When(d.RegionLabel != "", g.Group([]g.Node{
				g.Text(" — " + d.RegionLabel),
			})),
		))
	}
	return ui.Alert(ui.VariantWarning, "convert-dupe-warn",
		h.Div(
			h.P(h.Class("font-semibold"),
				g.Text("Ada desa dgn nama serupa di workspace ini. Periksa dulu — "+
					"mungkin desa yang sama, atau memang beda dusun/kabupaten:")),
			h.Ul(h.Class("list-disc list-inside text-sm"), g.Group(items)),
		),
	)
}

// convertPhoneField = input telepon yang menghormati F4. Editable (Sales) →
// input biasa. Terkunci (bukan Sales) → input disabled menampilkan mask, TANPA
// name agar tak ter-submit; handler menyalin nomor asli lead server-side.
func convertPhoneField(label, name, val string, editable bool) g.Node {
	if editable {
		return field(label, name, val, false, "tel")
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name+"_ro", false),
		ui.Input(
			h.ID("f-"+name+"_ro"), h.Type("tel"), h.Value(val),
			h.Disabled(), h.Class("input text-base w-full"),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Nomor disamarkan; disalin dari lead saat konversi.")),
	)
}
