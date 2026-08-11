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
	EntityCode  string
	VillageName string
	VillageCode string
	AccountType string
	Website     string
	Description string

	Province       string
	Regency        string
	District       string
	VillageAddress string
	PostalCode     string
	Territory      string

	VillageStatus         string
	VillageClassification string
	Population            string
	HamletsCount          string
	VillageBudget         string

	ContactPhone string
	OfficePhone  string
	OfficeEmail  string

	CanWrite bool
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
				ui.When(v.EntityCode != "", h.Span(
					h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
				h.Span(h.Class("badge badge-ghost"), g.Text(v.AccountType)),
			),
		),
		ui.When(v.CanWrite, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(base+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			deleteAccountForm(base),
		)),
	)

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2"),
			h.A(h.Href(v.Base+"/accounts"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar desa")),
			h.A(h.Href(base+"/contacts"), h.Class("btn btn-sm btn-ghost min-h-11"),
				g.Text("Kontak »")),
		),
		detailCard("Identitas", []detailField{
			{"Nama Desa", v.VillageName},
			{"Kode Desa (Kemendagri)", v.VillageCode},
			{"Tipe Akun", v.AccountType},
			{"Website", v.Website},
			{"Deskripsi", v.Description},
		}),
		detailCard("Wilayah", []detailField{
			{"Provinsi", v.Province},
			{"Kabupaten/Kota", v.Regency},
			{"Kecamatan", v.District},
			{"Alamat", v.VillageAddress},
			{"Kode Pos", v.PostalCode},
			{"Teritori", v.Territory},
		}),
		detailCard("Profil Desa", []detailField{
			{"Status", v.VillageStatus},
			{"Klasifikasi (IDM)", v.VillageClassification},
			{"Jumlah Penduduk", v.Population},
			{"Jumlah Dusun", v.HamletsCount},
			{"Anggaran (APBDes)", v.VillageBudget},
		}),
		detailCard("Kontak", []detailField{
			{"HP Kontak", v.ContactPhone},
			{"Telepon Kantor", v.OfficePhone},
			{"Email Kantor", v.OfficeEmail},
		}),
	)
}

type detailField struct {
	label string
	value string
}

// detailCard = kartu satu kelompok field, sebagai daftar deskripsi. Grid
// 1-kolom di mobile → 2-kolom label:nilai di sm ke atas (mobile-first).
func detailCard(title string, fields []detailField) g.Node {
	rows := make([]g.Node, 0, len(fields))
	for _, f := range fields {
		rows = append(rows, h.Div(
			h.Class("grid gap-1 sm:grid-cols-3 sm:gap-2 py-2 border-b border-base-300/50 last:border-0"),
			h.Dt(h.Class("text-sm text-base-content/60"), g.Text(f.label)),
			h.Dd(h.Class("sm:col-span-2 break-words"), g.Text(orDash(f.value))),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text(title)),
			h.Dl(h.Class("min-w-0"), g.Group(rows)),
		),
	)
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
