package panel

import (
	"strconv"
	"strings"

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
// aksi nested). VillageName = nama desa induk (kartu Identitas + breadcrumb).
// ReportsToID>0 → tautan ke kontak atasan di desa yang sama. Owner/CreatedBy/
// UpdatedBy sudah berupa NAMA (bukan id) dari handler; "" → "—". CreatedAt/UpdatedAt
// sudah terformat waktu lokal. LastContacted/LastActivity ditunda modul Activities
// (handler mengisi "" → "—"). CanWrite = tombol Sunting/Jadikan Utama/Hapus tampil.
type ContactDetailView struct {
	Base        string
	AccountBase string
	ID          int64
	AccountID   int64
	Name        string
	FirstName   string
	LastName    string
	Salutation  string
	JobTitle    string
	VillageName string

	ReportsToID   int64
	ReportsToName string
	OwnerName     string

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

	LastContacted string
	LastActivity  string
	EmailOptOut   bool
	DoNotContact  bool

	CreatedByName string
	CreatedAt     string
	UpdatedByName string
	UpdatedAt     string

	CanWrite bool
}

// ContactDetail merender hub detail kontak: header (breadcrumb + nama + penanda +
// subjudul + aksi), lalu ENAM kartu di grid dua kolom (mobile: satu kolom):
// Identitas, Peran & Otoritas, Komunikasi, Alamat, Ringkasan Keterlibatan (kolom
// aktivitas ditunda modul Activities), dan Sistem & Audit (read-only).
func ContactDetail(v ContactDetailView) g.Node {
	base := v.AccountBase + "/contacts/" + strconv.FormatInt(v.ID, 10)

	// Breadcrumb: Desa → Kontak → nama kini. Tautan <a> biasa (bookmarkable, lolos
	// gotcha #16). Desa & daftar kontak tertaut; nama kini teks biasa.
	crumb := h.Div(
		h.Class("flex flex-wrap items-center gap-1 text-sm text-base-content/60 min-w-0"),
		h.A(h.Href(v.AccountBase), h.Class("link link-hover truncate"),
			g.Text(orDash(v.VillageName))),
		g.Text("/"),
		h.A(h.Href(v.AccountBase+"/contacts"), h.Class("link link-hover"), g.Text("Kontak")),
		g.Text("/"),
		h.Span(h.Class("truncate text-base-content/80"), g.Text(v.Name)),
	)

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.Name)),
			ui.When(contactSubtitle(v) != "", h.P(
				h.Class("text-base-content/70 truncate"), g.Text(contactSubtitle(v)))),
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
		crumb,
		header,
		contactDetailCards(v),
	)
}

// contactSubtitle merangkai subjudul "Jabatan · Desa" dari bagian yang terisi saja
// (kosong tak menyisakan pemisah menggantung). Kosong total → "" (subjudul
// disembunyikan lewat ui.When di header).
func contactSubtitle(v ContactDetailView) string {
	parts := make([]string, 0, 2)
	if v.JobTitle != "" {
		parts = append(parts, v.JobTitle)
	}
	if v.VillageName != "" {
		parts = append(parts, v.VillageName)
	}
	return strings.Join(parts, " · ")
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
