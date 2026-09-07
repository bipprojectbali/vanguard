package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_leads_detail.go — halaman detail satu lead. Murni-data: EstValue &
// telepon SUDAH disamarkan handler (F4). Kolom kosong → "—". Reuse detailCard/
// detailField dari accounts_detail.go. Tombol Konversi hanya bila handler
// memutuskan CanConvert (Qualified & belum dikonversi & boleh tulis).

// LeadDetailView = seluruh data satu lead siap render. Converted + ConvertedDealID
// menentukan apakah menampilkan tautan ke Deal hasil konversi vs tombol Konversi.
type LeadDetailView struct {
	Base       string
	ID         int64
	EntityCode string

	LeadName      string
	ContactPerson string
	JobTitle      string
	LeadSource    string

	Status            string
	Rating            string
	UnqualifiedReason string
	EstValue          string

	// Statuses = opsi dropdown kontrol "Ubah Status" (BL-83, dari handler).
	// Enum status manual (Converted dikecualikan — status sistem).
	Statuses []string
	// Err = pesan galat PRG (?err=CODE) utk kontrol status; kosong = tak ada.
	Err string

	Province    string
	Regency     string
	District    string
	MobilePhone string
	Whatsapp    string
	Email       string

	Owner string

	Converted       bool
	ConvertedDealID string

	CanWrite   bool
	CanConvert bool
}

// LeadDetail merender hub detail: header (nama + kode + status + aksi), lalu
// kartu identitas, kualifikasi, lokasi/kontak, dan banner konversi.
func LeadDetail(v LeadDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	base := v.Base + "/leads/" + idStr

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.LeadName)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				ui.When(v.EntityCode != "", h.Span(
					h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
				leadStatusBadge(v.Status),
				ui.When(v.Rating != "", h.Span(h.Class("badge badge-ghost"), g.Text(v.Rating))),
			),
		),
		ui.When(v.CanWrite, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(base+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			deleteLeadForm(base),
		)),
	)

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.A(h.Href(v.Base+"/leads"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar lead")),
		// BL-83: galat PRG kontrol status (?err=CODE) disurfacing di sini — beda
		// dari detail Deal yang tak menampilkannya. Kosong → tak dirender.
		ui.When(v.Err != "", ui.Alert(ui.VariantDestructive, "lead-status-err", g.Text(v.Err))),
		leadConvertBanner(v, base),
		// BL-83: kontrol "Ubah Status" hanya untuk aktor boleh-tulis atas lead yang
		// BELUM dikonversi (converted = status terminal, tak boleh diputar balik).
		ui.When(v.CanWrite && !v.Converted, leadStatusControl(v, base)),
		detailCard("Identitas Lead", []detailField{
			{"Nama Lead", v.LeadName},
			{"Kontak", v.ContactPerson},
			{"Jabatan", v.JobTitle},
			{"Sumber", v.LeadSource},
			{"Pemilik", v.Owner},
		}),
		detailCard("Kualifikasi & Status", []detailField{
			{"Status", v.Status},
			{"Rating", v.Rating},
			{"Nilai Estimasi", v.EstValue},
			{"Alasan Unqualified", v.UnqualifiedReason},
		}),
		detailCard("Lokasi & Kontak", []detailField{
			{"Provinsi", v.Province},
			{"Kabupaten/Kota", v.Regency},
			{"Kecamatan", v.District},
			{"HP", v.MobilePhone},
			{"WhatsApp", v.Whatsapp},
			{"Email", v.Email},
		}),
	)
}

// leadConvertBanner = ajakan/keterangan konversi. Tiga keadaan jujur: sudah
// dikonversi (tautan ke Deal), bisa dikonversi (tombol ke halaman review), atau
// belum layak (keterangan syarat, tanpa tombol yang menyesatkan).
func leadConvertBanner(v LeadDetailView, base string) g.Node {
	if v.Converted {
		msg := "Lead ini sudah dikonversi menjadi Desa, Kontak, dan Deal."
		body := []g.Node{h.Span(g.Text(msg))}
		if v.ConvertedDealID != "" {
			body = append(body, h.A(
				h.Href(v.Base+"/deals/"+v.ConvertedDealID),
				h.Class("btn btn-sm btn-primary min-h-11"), g.Text("Lihat Deal »")))
		}
		return h.Div(
			h.Class("card bg-base-100 border border-primary/40 min-w-0"),
			h.Div(h.Class("card-body flex-row flex-wrap items-center justify-between gap-2"),
				g.Group(body)),
		)
	}
	if v.CanConvert {
		return h.Div(
			h.Class("card bg-base-100 border border-success/40 min-w-0"),
			h.Div(h.Class("card-body flex-row flex-wrap items-center justify-between gap-2"),
				h.Span(g.Text("Lead ini Qualified — konversikan jadi Desa + Kontak + Deal.")),
				h.A(h.Href(base+"/convert"), h.Class("btn btn-sm btn-success min-h-11"),
					g.Text("Konversi Lead »")),
			),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Lead bisa dikonversi setelah berstatus Qualified."))),
	)
}

// deleteLeadForm = tombol hapus (soft-delete). Form NATIVE POST → 303 (gotcha #16).
func deleteLeadForm(base string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text("Hapus")),
	)
}
