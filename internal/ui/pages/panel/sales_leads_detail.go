package panel

import (
	"strconv"

	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_leads_detail.go — halaman detail satu lead. Murni-data: EstValue &
// telepon SUDAH disamarkan handler (F4). Kolom kosong → "—".
//
// BL-119 (wireframe 4.1): tata letak = header (nama + badge status/rating +
// baris Pemilik·Sumber + grup aksi) lalu grid DUA-kolom kartu (Identitas ·
// Kualifikasi&Status | Lokasi&Kontak · Sistem&Audit), ditutup keterangan
// konversi. Aksi di header: Sunting, Ubah Status (→ modal), Konversi Lead /
// Lihat Deal, Hapus (→ modal konfirmasi). Konversi TAK lagi banner terpisah;
// kontrol status & hapus TAK lagi inline — keduanya modal CSP-safe (signal
// Datastar; form NATIVE POST → 303, gotcha #16). Kartu di sales_leads_detail_cards.go.

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
	Email       string

	Owner string

	// BL-119: kartu "Sistem & Audit" (wireframe 4.1). Nilai sudah diformat handler.
	CreatedByName string
	CreatedAt     string
	UpdatedByName string
	UpdatedAt     string

	Converted       bool
	ConvertedDealID string

	CanWrite   bool
	CanConvert bool
}

// LeadDetail merender hub detail: header (nama + badge + baris meta + aksi), grid
// dua-kolom kartu, keterangan konversi, lalu modal (status & hapus) tersembunyi.
func LeadDetail(v LeadDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	base := v.Base + "/leads/" + idStr

	body := []g.Node{
		leadDetailHeader(v, base),
		h.A(h.Href(v.Base+"/leads"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar lead")),
		// BL-83: galat PRG kontrol status (?err=CODE) disurfacing di sini.
		ui.When(v.Err != "", ui.Alert(ui.VariantDestructive, "lead-status-err", g.Text(v.Err))),
		// Grid dua-kolom (mobile: satu kolom). Urutan mengisi baris: Identitas |
		// Kualifikasi, lalu Lokasi | Sistem&Audit (wireframe 4.1).
		h.Div(
			h.Class("grid gap-4 md:grid-cols-2 min-w-0"),
			leadIdentityCard(v),
			leadQualificationCard(v),
			leadLocationCard(v),
			leadSystemAuditCard(v),
		),
		leadConvertNote(v),
	}
	// Modal kontrol status: hanya untuk aktor boleh-tulis atas lead BELUM
	// dikonversi (converted = status terminal; guard query AND NOT converted).
	if v.CanWrite && !v.Converted {
		body = append(body, leadStatusModal(v, base))
	}
	// Modal konfirmasi hapus (BL-120): aktor boleh-tulis atas lead BELUM
	// dikonversi. Lead terkonversi = terminal (tak boleh disunting/dihapus) →
	// tombol & modal hapus tak dirender (sejajar guard backend LeadDelete).
	if v.CanWrite && !v.Converted {
		body = append(body, leadDeleteModal(base))
	}

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// leadDetailHeader = judul + badge status/rating + baris Pemilik·Sumber (kiri) &
// grup aksi (kanan). Grup aksi hanya untuk aktor boleh-tulis; lead terkonversi
// menukar tombol Konversi jadi tautan "Lihat Deal».
func leadDetailHeader(v LeadDetailView, base string) g.Node {
	badges := []g.Node{
		ui.When(v.EntityCode != "", h.Span(
			h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
		leadStatusBadge(v.Status),
		ui.When(v.Rating != "", h.Span(h.Class("badge badge-ghost"), g.Text(v.Rating))),
	}
	meta := leadMetaLine(v)
	return h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2"),
				h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.LeadName)),
				g.Group(badges),
			),
			ui.When(meta != "", h.P(h.Class("text-sm text-base-content/60 mt-1"), g.Text(meta))),
		),
		leadDetailActions(v, base),
	)
}

// leadMetaLine = "Pemilik: X · Sumber: Y" (bagian yang terisi saja). "" bila
// keduanya kosong (baris tak dirender).
func leadMetaLine(v LeadDetailView) string {
	parts := make([]string, 0, 2)
	if v.Owner != "" {
		parts = append(parts, "Pemilik: "+v.Owner)
	}
	if v.LeadSource != "" {
		parts = append(parts, "Sumber: "+v.LeadSource)
	}
	line := ""
	for i, p := range parts {
		if i > 0 {
			line += " · "
		}
		line += p
	}
	return line
}

// leadDetailActions = grup tombol header. Aktor read-only: hanya tautan Deal bila
// terkonversi. Aktor boleh-tulis atas lead BELUM dikonversi: Sunting, Ubah Status,
// Konversi. Lead TERKONVERSI = terminal: Sunting/Ubah Status/Hapus disembunyikan
// (backend LeadEdit/LeadUpdate/LeadDelete menolak) — hanya "Lihat Deal" tersisa.
func leadDetailActions(v LeadDetailView, base string) g.Node {
	if v.Converted {
		if v.ConvertedDealID != "" {
			return h.Div(h.Class("flex flex-wrap items-center gap-2"),
				leadDealLink(v))
		}
		return nil
	}
	if !v.CanWrite {
		return nil
	}
	actions := []g.Node{
		h.A(h.Href(base+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
		leadStatusTrigger(),
	}
	if v.CanConvert {
		actions = append(actions, h.A(
			h.Href(base+"/convert"),
			h.Class("btn btn-sm btn-primary min-h-11 gap-1"),
			lucide.Check(h.Class("size-4")),
			g.Text("Konversi Lead"),
		))
	}
	actions = append(actions, leadDeleteTrigger())
	return h.Div(h.Class("flex flex-wrap items-center gap-2"), g.Group(actions))
}

// leadDealLink = tautan ke Deal hasil konversi (dipakai header saat terkonversi).
func leadDealLink(v LeadDetailView) g.Node {
	return h.A(
		h.Href(v.Base+"/deals/"+v.ConvertedDealID),
		h.Class("btn btn-sm btn-primary min-h-11"), g.Text("Lihat Deal »"),
	)
}

// leadConvertNote = keterangan konversi di bawah grid (wireframe 4.1: menggantikan
// banner). Tiga keadaan jujur: sudah dikonversi, Qualified (ajakan), belum layak.
func leadConvertNote(v LeadDetailView) g.Node {
	if v.Converted {
		return h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Lead ini sudah dikonversi menjadi Desa, Kontak, dan Deal."))
	}
	if v.CanConvert {
		return h.Div(
			h.Class("min-w-0"),
			h.P(h.Class("text-sm font-medium text-primary"),
				g.Text("Konversi membuat Deal + Desa + Kontak.")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Lead Qualified — satu klik menaikkan ke pipeline penjualan.")),
		)
	}
	return h.P(h.Class("text-sm text-base-content/60"),
		g.Text("Lead bisa dikonversi setelah berstatus Qualified."))
}

// leadDeleteSignal = signal Datastar boolean pengendali modal konfirmasi hapus.
const leadDeleteSignal = "leadDeleteOpen"

// leadDeleteTrigger = tombol pembuka modal hapus (klik → set signal true). BL-120:
// hapus lead kini lewat konfirmasi modal, bukan submit langsung.
func leadDeleteTrigger() g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("btn btn-sm btn-error btn-outline min-h-11"),
		data.On("click", "$"+leadDeleteSignal+" = true"),
		g.Text("Hapus"),
	)
}

// leadDeleteModal = modal konfirmasi hapus lead (BL-120). Pola sama assignCSModal
// (signal $leadDeleteOpen; inline display:none anti-FOUC; backdrop/Batal menutup
// via signal). Form NATIVE POST ke /delete → 303 (gotcha #16). Sertakan SEKALI.
func leadDeleteModal(base string) g.Node {
	openExpr := "$" + leadDeleteSignal
	return h.Div(
		h.Class("fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"),
		g.Attr("style", "display:none"),
		data.Show(openExpr),
		data.On("click", openExpr+" = false"),
		h.Div(
			h.Class("card bg-base-100 shadow-lg w-full max-w-md"),
			data.On("click", "evt.stopPropagation()"),
			h.Div(
				h.Class("flex items-center justify-between gap-2 px-5 pt-5 pb-3 border-b border-base-300"),
				h.H2(h.Class("text-lg font-semibold"), g.Text("Hapus Lead")),
				h.Button(
					h.Type("button"),
					h.Class("btn btn-ghost btn-sm btn-circle"),
					g.Attr("aria-label", "Tutup"),
					data.On("click", openExpr+" = false"),
					lucide.X(h.Class("size-5")),
				),
			),
			h.Div(
				h.Class("px-5 py-4"),
				h.P(h.Class("text-sm text-base-content/70 mb-4"),
					g.Text("Lead ini akan dihapus dari daftar aktif. Lanjutkan?")),
				h.Div(
					h.Class("flex flex-wrap justify-end gap-2"),
					h.Button(h.Type("button"), h.Class("btn btn-ghost min-h-11"),
						data.On("click", openExpr+" = false"), g.Text("Batal")),
					h.FormEl(
						h.Method("post"), h.Action(base+"/delete"),
						h.Button(h.Type("submit"), h.Class("btn btn-error min-h-11"),
							g.Text("Hapus")),
					),
				),
			),
		),
	)
}
