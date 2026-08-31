package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts_form.go — form buat/sunting desa + penugasan CSM. Form NATIVE POST →
// 303 (gotcha #16: redirect lewat SSE diblokir CSP). Validasi sesungguhnya di
// backend (parseAccountForm); atribut di sini hanya jaring klien.
//
// Nilai enum untuk dropdown dioper handler (AccountEnumOptions) — view tak
// memutuskan mode/aturan. Semua nilai prefill sudah string di handler.

// AccountFormFields = nilai prefill form (edit) atau kosong (buat). Semua string
// agar view netral terhadap tipe DB. Population/HamletsCount/Budget sebagai
// string apa adanya untuk input number/text.
type AccountFormFields struct {
	VillageName           string
	AccountType           string
	Website               string
	Description           string
	DistrictID            string
	VillageAddress        string
	PostalCode            string
	Territory             string
	VillageStatus         string
	VillageClassification string
	Population            string
	HamletsCount          string
	VillageBudget         string
	ContactPhone          string
	OfficePhone           string
	OfficeEmail           string
}

// AccountMemberOption = satu kandidat penerima tugas (assigned/backup CSM).
type AccountMemberOption struct {
	ID    int64
	Label string
}

// AccountFormView = data halaman form. Action = URL POST tujuan. IsEdit
// mengubah judul/label & memunculkan kartu penugasan. PhoneEditable=false (bukan
// Sales) → field HP dikunci: nilainya tersamar, dan membiarkannya editable akan
// menimpa nomor asli dengan mask saat submit (handler juga mempertahankannya).
type AccountFormView struct {
	Base   string
	Action string
	IsEdit bool
	Err    string
	Fields AccountFormFields

	// RegionsJSON = dataset penuh master wilayah (h.regionsJSON), diembed sekali
	// utk cascading dropdown Provinsi/Kabupaten-Kota/Kecamatan (ADR 0009).
	RegionsJSON string

	PhoneEditable bool

	Types           []string
	Statuses        []string
	Classifications []string

	// Penugasan CSM (hanya edit). AssignAction = URL POST /assign. Members =
	// kandidat. AssignedCSM/BackupCSM = pilihan saat ini (id sebagai string, ""
	// = belum ditugaskan).
	AssignAction string
	Members      []AccountMemberOption
	AssignedCSM  string
	BackupCSM    string
}

// AccountForm merender halaman form lengkap.
func AccountForm(v AccountFormView) g.Node {
	title := "Tambah Desa"
	submit := "Simpan Desa"
	if v.IsEdit {
		title = "Sunting Desa"
		submit = "Simpan Perubahan"
	}

	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.P(h.Class("text-sm text-base-content/60 mb-1"), g.Text(accountFormHint(v.IsEdit))),
			h.A(h.Href(v.Base+"/accounts"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar desa")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "account-form-err", g.Text(v.Err)))
	}

	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),

		formCard("Identitas",
			field("Nama Desa", "village_name", v.Fields.VillageName, true, "text"),
			selectField("Tipe Akun", "account_type", v.Fields.AccountType, v.Types, true,
				"Prospect = calon pelanggan (belum berlangganan) · Customer = pelanggan aktif · "+
					"Former Customer = pernah berlangganan, sudah berhenti."),
			entityCodeField(v.IsEdit),
			field("Website", "website", v.Fields.Website, false, "url"),
			textareaField("Deskripsi", "description", v.Fields.Description),
		),
		formCard("Wilayah",
			regionSelect("account", v.RegionsJSON, v.Fields.DistrictID, !v.IsEdit),
			field("Alamat", "village_address", v.Fields.VillageAddress, false, "text"),
			field("Kode Pos", "postal_code", v.Fields.PostalCode, false, "text"),
			field("Teritori", "territory", v.Fields.Territory, false, "text",
				"Pembagian wilayah kerja Sales/CS internal (bebas isi) — beda dari Kabupaten/Kota "+
					"administratif di atas."),
		),
		formCard("Profil Desa",
			selectField("Status", "village_status", v.Fields.VillageStatus, v.Statuses, false,
				"Sebutan resmi wilayah administratif setingkat desa — beda istilah per daerah "+
					"(Kelurahan, Nagari di Sumbar, Gampong di Aceh, dst)."),
			selectField("Klasifikasi (IDM)", "village_classification", v.Fields.VillageClassification, v.Classifications, false,
				"Indeks Desa Membangun, dari yang paling maju: Mandiri > Maju > Berkembang > "+
					"Tertinggal > Sangat Tertinggal."),
			field("Jumlah Penduduk", "population", v.Fields.Population, false, "number"),
			field("Jumlah Dusun", "hamlets_count", v.Fields.HamletsCount, false, "number"),
			field("Anggaran (APBDes)", "village_budget", v.Fields.VillageBudget, false, "text"),
		),
		formCard("Kontak",
			phoneField(v.PhoneEditable, v.Fields.ContactPhone),
			field("Telepon Kantor", "office_phone", v.Fields.OfficePhone, false, "tel"),
			field("Email Kantor", "office_email", v.Fields.OfficeEmail, false, "email"),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/accounts"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))

	if v.IsEdit {
		body = append(body, assignCard(v))
	}
	// Cascading dropdown wilayah (regionSelect di atas cuma menanam data + markup;
	// interaksi berjenjangnya di sini, same-origin CSP-safe, gotcha #12).
	body = append(body, h.Script(h.Src("/static/regions.js"), h.Defer()))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// accountFormHint = penjelasan singkat di bawah judul form, beda utk tambah vs
// sunting — sama pola dgn accountsTabDesc (penjelasan mengikuti konteks yang
// sedang dilihat, bukan satu kalimat generik). Menyebut penugasan CSM eksplisit
// karena assignCard baru muncul setelah desa TERSIMPAN (IsEdit) — tanpa hint ini
// pengguna baru bisa bingung kenapa opsi itu tak ada di form tambah.
func accountFormHint(isEdit bool) string {
	if isEdit {
		return "Ubah data desa ini. Penugasan CS (utama/cadangan) diatur terpisah lewat kartu \"Penugasan CS\" di bawah."
	}
	return "Lengkapi data desa baru untuk workspace ini. Penugasan CS (utama/cadangan) bisa dilakukan setelah desa tersimpan, dari halaman sunting."
}

// assignCard = penugasan CSM (assigned + backup). FORM TERPISAH posting ke
// /assign — aksi berbeda dari simpan-field, jadi tak boleh berbagi tombol submit.
func assignCard(v AccountFormView) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-1"), g.Text("Penugasan CS")),
			h.P(h.Class("text-sm text-base-content/60 mb-2"),
				g.Text("Menentukan siapa yang melihat desa ini di daftar mereka.")),
			h.FormEl(
				h.Method("post"), h.Action(v.AssignAction),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				memberSelect("CS Utama", "assigned_csm", v.AssignedCSM, v.Members),
				memberSelect("CS Cadangan", "backup_csm", v.BackupCSM, v.Members),
				h.Div(
					h.Class("sm:col-span-2"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Penugasan")),
				),
			),
		),
	)
}

// memberSelect = dropdown anggota (opsional; "" = tak ditugaskan).
func memberSelect(label, name, current string, members []AccountMemberOption) g.Node {
	nodes := []g.Node{h.Option(h.Value(""), g.Text("— Tidak ditugaskan —"))}
	for _, m := range members {
		id := strconv.FormatInt(m.ID, 10)
		attrs := []g.Node{h.Value(id)}
		if id == current {
			attrs = append(attrs, h.Selected())
		}
		nodes = append(nodes, h.Option(append(attrs, g.Text(m.Label))...))
	}
	sel := []g.Node{h.ID("f-" + name), h.Name(name), h.Class("select text-base w-full")}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name, false),
		h.Select(append(sel, g.Group(nodes))...),
	)
}

// labelFor merender label + penanda wajib (*). Diberi `for` agar tap label
// memfokus input (tap target).
func labelFor(text, forID string, required bool) g.Node {
	children := []g.Node{g.Text(text)}
	if required {
		children = append(children, h.Span(h.Class("text-error ml-0.5"), g.Text("*")))
	}
	return h.Label(h.For(forID), h.Class("text-sm text-base-content/70"), g.Group(children))
}
