package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts_detail.go — halaman detail satu desa. Murni-data: ContactPhone SUDAH
// disamarkan handler (F4) bila penglihatnya bukan Sales; view merendernya apa
// adanya. Kolom kosong ditampilkan sebagai "—", bukan disembunyikan, agar
// pembaca tahu bedanya "belum diisi" dari "tak ada field-nya".

// AccountDetailView = seluruh data satu desa siap render. Semua string sudah
// diformat/disamarkan di handler. CanWrite = tombol Sunting/Tugaskan/Hapus
// tampil (F2 write & tak read-only).
type AccountDetailView struct {
	Base        string
	ID          int64
	VillageName string
	// VillageCode = Kode Desa (Kemendagri) yang ditampilkan; entity_code (kode
	// sistem) sengaja TAK di-view lagi (BL-61) — tetap auto-generated di backend.
	VillageCode string
	AccountType string
	Website     string
	Description string

	// AccountOwnerName/ParentAccountLabel = kolom identitas tambahan (M2-7).
	// ParentAccountHref non-kosong → ParentAccountLabel jadi tautan; kosong
	// (termasuk saat resolve induk gagal best-effort) → teks polos/"—".
	AccountOwnerName   string
	ParentAccountLabel string
	ParentAccountHref  string

	Province       string
	Regency        string
	District       string
	VillageAddress string
	PostalCode     string
	Territory      string
	// Latitude/Longitude = koordinat desa (M2-7), kolom sudah ada di skema
	// sejak awal, sebelumnya tak pernah disurfacekan ke view.
	Latitude  string
	Longitude string

	VillageStatus         string
	VillageClassification string
	Population            string
	HamletsCount          string
	VillageBudget         string

	ContactPhone string
	OfficePhone  string
	OfficeEmail  string

	CanWrite bool

	// Subscription/CustomerSuccess/Audit = kartu ringkasan lintas-modul (M2-8),
	// satu layout utk semua role — F2/F3/F4 di handler yang memutuskan isinya,
	// bukan tampilan terpisah per POV. Diisi handler via accounts_rollups.go.
	Subscription    SubscriptionSummaryView
	CustomerSuccess CustomerSuccessSummaryView
	Audit           AuditView

	// Related = baris "Terkait" (M2-9): ringkasan Contacts/Deals/Subscriptions/
	// Tickets satu desa.
	Related RelatedRecordsView

	// Activities = timeline aktivitas desa ini (M7-A). Diisi handler via
	// activitiesTimelineFor (dibatasi activityTimelineLimit baris terbaru).
	Activities ActivityTimelineView
}

// AccountDetail merender hub detail: header (nama + kode + aksi), lalu kartu
// identitas, wilayah, profil desa, dan kontak.
func AccountDetail(v AccountDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	base := v.Base + "/accounts/" + idStr

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.VillageName)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				// BL-61: badge Kode Sistem (entity_code) dihapus dari detail —
				// operator cukup lihat "Kode Desa (Kemendagri)" (village_code) di
				// kartu Identitas. entity_code tetap dibuat & tersimpan di backend.
				// Tipe Akun kini badge terisi (badge-neutral) — mewarisi gaya
				// background badge kode sistem yang dilepas, bukan ghost.
				h.Span(h.Class("badge badge-neutral"), g.Text(v.AccountType)),
			),
		),
		ui.When(v.CanWrite, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(base+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			deleteAccountForm(base),
		)),
	)

	parentAccountValue := g.Node(g.Text(orDash(v.ParentAccountLabel)))
	if v.ParentAccountHref != "" {
		parentAccountValue = h.A(h.Href(v.ParentAccountHref), h.Class("link link-hover"),
			g.Text(v.ParentAccountLabel))
	}

	identitas := cardRows("Identitas", "",
		detailRow("Nama Desa", g.Text(orDash(v.VillageName))),
		detailRow("Kode Desa (Kemendagri)", g.Text(orDash(v.VillageCode))),
		detailRow("Tipe Akun", g.Text(orDash(v.AccountType))),
		detailRow("Pemilik Akun", g.Text(orDash(v.AccountOwnerName))),
		detailRow("Induk Akun", parentAccountValue),
		detailRow("Website", g.Text(orDash(v.Website))),
		detailRow("Deskripsi", g.Text(orDash(v.Description))),
	)

	wilayah := detailCard("Wilayah", []detailField{
		{"Provinsi", v.Province},
		{"Kabupaten/Kota", v.Regency},
		{"Kecamatan", v.District},
		{"Alamat", v.VillageAddress},
		{"Kode Pos", v.PostalCode},
		{"Teritori", v.Territory},
		{"Lintang", v.Latitude},
		{"Bujur", v.Longitude},
	})

	profilDesa := detailCard("Profil Desa", []detailField{
		{"Status", v.VillageStatus},
		{"Klasifikasi (IDM)", v.VillageClassification},
		{"Jumlah Penduduk", v.Population},
		{"Jumlah Dusun", v.HamletsCount},
		{"Anggaran (APBDes)", v.VillageBudget},
	})

	kontak := detailCard("Kontak", []detailField{
		{"HP Kontak", v.ContactPhone},
		{"Telepon Kantor", v.OfficePhone},
		{"Email Kantor", v.OfficeEmail},
	})

	sistem := detailCard("Sistem", []detailField{
		{"Dibuat oleh", v.Audit.CreatedByName},
		{"Dibuat pada", v.Audit.CreatedAt},
		{"Diperbarui oleh", v.Audit.UpdatedByName},
		{"Diperbarui pada", v.Audit.UpdatedAt},
	})

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2"),
			h.A(h.Href(v.Base+"/accounts"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar desa")),
			h.A(h.Href(base+"/contacts"), h.Class("btn btn-sm btn-ghost min-h-11"),
				g.Text("Kontak »")),
			h.A(h.Href(base+"/customer-success"), h.Class("btn btn-sm btn-ghost min-h-11"),
				g.Text("Customer Success »")),
		),
		h.Div(
			h.Class("grid grid-cols-1 md:grid-cols-2 gap-4 min-w-0"),
			h.Div(h.Class("grid gap-4 min-w-0"), identitas, wilayah, profilDesa, kontak),
			h.Div(h.Class("grid gap-4 min-w-0"),
				SubscriptionSummaryCard(v.Subscription),
				CustomerSuccessSummaryCard(v.CustomerSuccess),
				sistem,
			),
		),
		RelatedRecords(v.Related),
		ActivityTimeline(v.Activities),
	)
}

type detailField struct {
	label string
	value string
}

// detailCard = kartu satu kelompok field label:nilai teks polos — wrapper tipis
// di atas cardRows/detailRow (accounts_detail_rollup.go), yang menerima g.Node
// utk baris non-teks (badge/tautan, dipakai kartu rollup & Identitas).
func detailCard(title string, fields []detailField) g.Node {
	rows := make([]g.Node, 0, len(fields))
	for _, f := range fields {
		rows = append(rows, detailRow(f.label, g.Text(orDash(f.value))))
	}
	return cardRows(title, "", rows...)
}

// deleteAccountForm = tombol hapus (soft-delete). Form NATIVE POST → 303
// (gotcha #16). Konfirmasi via native confirm dialog tak tersedia lewat CSP
// inline; penghapusan reversibel (soft-delete) jadi cukup tombol destruktif jelas.
func deleteAccountForm(base string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text("Hapus")),
	)
}
