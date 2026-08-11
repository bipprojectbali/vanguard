package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts_detail.go — halaman detail satu kontak. Murni-data: MobilePhone &
// WhatsappNumber SUDAH disamarkan handler (F4) bila penglihatnya bukan Sales;
// OfficePhone tidak (nomor kelembagaan). Kolom kosong → "—" (detailCard), bukan
// disembunyikan: pembaca harus bisa membedakan "belum diisi" dari "tak ada".

// ContactDetailView = seluruh data satu kontak siap render. Semua string sudah
// diformat/disamarkan di handler. AccountBase = URL desa induk (tautan kembali &
// aksi nested). CanWrite = tombol Sunting/Jadikan Utama/Hapus tampil.
type ContactDetailView struct {
	Base        string
	AccountBase string
	ID          int64
	AccountID   int64
	Name        string
	Salutation  string
	JobTitle    string

	PositionCategory string
	ContactRole      string
	TermPeriod       string
	IsPrimary        bool
	IsTechnical      bool

	MobilePhone    string
	WhatsappNumber string
	OfficePhone    string
	Email          string

	PreferredChannel string
	MailingAddress   string
	City             string
	PostalCode       string

	EmailOptOut  bool
	DoNotContact bool

	CanWrite bool
}

// ContactDetail merender hub detail kontak: header (nama + penanda + aksi), lalu
// kartu identitas, kontak (nomor/email), dan preferensi/kepatuhan.
func ContactDetail(v ContactDetailView) g.Node {
	base := v.AccountBase + "/contacts/" + strconv.FormatInt(v.ID, 10)

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.Name)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				ui.When(v.IsPrimary, h.Span(
					h.Class("badge badge-primary"), g.Text("Kontak Utama"))),
				ui.When(v.IsTechnical, h.Span(
					h.Class("badge badge-ghost"), g.Text("Kontak Teknis"))),
				ui.When(v.EmailOptOut, h.Span(
					h.Class("badge badge-warning"), g.Text("Opt-out email"))),
				ui.When(v.DoNotContact, h.Span(
					h.Class("badge badge-error"), g.Text("Jangan hubungi"))),
			),
		),
		ui.When(v.CanWrite, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(base+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			ui.When(!v.IsPrimary, setPrimaryContactForm(base)),
			deleteContactForm(base),
		)),
	)

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.A(h.Href(v.AccountBase+"/contacts"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar kontak")),
		detailCard("Identitas", []detailField{
			{"Sapaan", v.Salutation},
			{"Nama", v.Name},
			{"Jabatan", v.JobTitle},
			{"Jabatan (Kategori)", v.PositionCategory},
			{"Peran", v.ContactRole},
			{"Periode Menjabat", v.TermPeriod},
		}),
		detailCard("Kontak", []detailField{
			{"HP (Pribadi)", v.MobilePhone},
			{"WhatsApp", v.WhatsappNumber},
			{"Telepon Kantor", v.OfficePhone},
			{"Email", v.Email},
			{"Kanal Pilihan", v.PreferredChannel},
		}),
		detailCard("Alamat & Kepatuhan", []detailField{
			{"Alamat Surat", v.MailingAddress},
			{"Kota", v.City},
			{"Kode Pos", v.PostalCode},
			{"Opt-out Email", boolLabel(v.EmailOptOut)},
			{"Jangan Hubungi", boolLabel(v.DoNotContact)},
		}),
	)
}

// boolLabel = teks manusiawi untuk kolom boolean kepatuhan (bukan checkbox di
// detail read-only). "Ya"/"Tidak" agar tak rancu dengan "—" (belum diisi).
func boolLabel(b bool) string {
	if b {
		return "Ya"
	}
	return "Tidak"
}

// setPrimaryContactForm = tombol "Jadikan Utama". Form NATIVE POST → 303
// (gotcha #16). Hanya muncul bila kontak ini BELUM utama (ui.When di header).
func setPrimaryContactForm(base string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/primary"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-primary btn-outline min-h-11"),
			g.Text("Jadikan Utama")),
	)
}

// deleteContactForm = tombol hapus (soft-delete). Form NATIVE POST → 303.
// Menghapus kontak utama membebaskan slot utama desa (index partial).
func deleteContactForm(base string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text("Hapus")),
	)
}
