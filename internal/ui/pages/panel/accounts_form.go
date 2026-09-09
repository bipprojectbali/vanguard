package panel

import (
	"strconv"

	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
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
	VillageName string
	// VillageID = id master regions level 4 (Desa/Kelurahan) terpilih, utk
	// preselect dropdown Desa saat edit (BL-66). Kosong = legacy/village_code tak
	// cocok master → dropdown Desa dibiarkan kosong (nama tersimpan tetap tampil
	// sbg catatan). VillageName kini catatan read-only, bukan input.
	VillageID             string
	AccountType           string
	Website               string
	Description           string
	DistrictID            string
	VillageAddress        string
	PostalCode            string
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
// mengubah judul/label & memunculkan kartu penugasan. BL-106: HP Kontak account
// tak lagi ber-FLS — field HP selalu biasa (tak ada flag kunci per-peran).
type AccountFormView struct {
	Base   string
	Action string
	IsEdit bool
	Err    string
	Fields AccountFormFields

	// RegionsJSON = dataset penuh master wilayah (h.regionsJSON), diembed sekali
	// utk cascading dropdown Provinsi/Kabupaten-Kota/Kecamatan (ADR 0009).
	RegionsJSON string

	// VillagesURL = endpoint lazy-fetch daftar Desa per Kecamatan (BL-66),
	// mis. "/w/{slug}/accounts/villages". Dataset Desa (~83.762) terlalu besar
	// utk diembed di RegionsJSON → dropdown level 4 memuatnya on-demand.
	VillagesURL string

	Types           []string
	Statuses        []string
	Classifications []string
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
			// BL-66: input "Nama Desa" dilepas — nama diturunkan dari Desa yang
			// dipilih di kartu Wilayah (master Kemendagri), bukan diketik bebas.
			selectField("Tipe Akun", "account_type", v.Fields.AccountType, v.Types, true,
				"Prospect = calon pelanggan (belum berlangganan) · Customer = pelanggan aktif · "+
					"Former Customer = pernah berlangganan, sudah berhenti."),
			// BL-60: input "Kode Sistem" (entity_code) dilepas dari UI — kode desa
			// yang dipakai manusia adalah village_code (Kemendagri). entity_code
			// TETAP dibuat otomatis & disimpan (accounts_codes.go); hanya pintu
			// override manual di form yang dihapus.
			field("Website", "website", v.Fields.Website, false, "url"),
			textareaField("Deskripsi", "description", v.Fields.Description),
		),
		formCard("Wilayah",
			// BL-66: cascading 4 level (Provinsi→Kab/Kota→Kecamatan→Desa). Desa
			// (level 4) menentukan village_code Kemendagri + district_id; nama desa
			// diturunkan darinya. Saat edit desa lama tak cocok master, dropdown
			// Desa kosong → tampilkan nama tersimpan sbg catatan agar tak hilang.
			g.If(v.IsEdit && v.Fields.VillageName != "",
				h.P(h.Class("text-sm text-base-content/60 sm:col-span-2"),
					g.Text("Desa saat ini: "+v.Fields.VillageName+". Pilih ulang di bawah untuk mengubah."))),
			regionSelectWithVillage("account", v.RegionsJSON, v.Fields.DistrictID, v.Fields.VillageID, v.VillagesURL, !v.IsEdit),
			field("Alamat", "village_address", v.Fields.VillageAddress, false, "text"),
			field("Kode Pos", "postal_code", v.Fields.PostalCode, false, "text"),
			// BL-60: field "Teritori" dilepas dari UI untuk sementara. Kolomnya &
			// data lama DIPERTAHANKAN (tetap tampil di detail; AccountUpdate tak
			// menimpanya) — cukup input form yang disembunyikan.
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
			// BL-60: APBDes = input UANG (prefix "Rp", keypad angka, pengelompokan
			// ribuan) — bukan teks bebas. Nilai dinormalkan digit polos sebelum
			// submit (numgroup.js); backend cleanThousands penjaga tanpa JS.
			moneyFieldRp("Anggaran (APBDes)", "village_budget", v.Fields.VillageBudget),
		),
		formCard("Kontak",
			phoneField(v.Fields.ContactPhone),
			field("Telepon Kantor", "office_phone", v.Fields.OfficePhone, false, "tel"),
			field("Email Kantor", "office_email", v.Fields.OfficeEmail, false, "email"),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/accounts"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))

	// BL-108: kartu "Penugasan CS" TIDAK lagi di form edit desa — dipindah ke
	// halaman detail Customer Success (assignCSCard, customer_success_view.go).
	// Satu pintu penugasan, tak menumpang form profil.
	// Cascading dropdown wilayah (regionSelect di atas cuma menanam data + markup;
	// interaksi berjenjangnya di sini, same-origin CSP-safe, gotcha #12).
	body = append(body, h.Script(h.Src("/static/regions.js"), h.Defer()))
	// BL-60: pengelompokan ribuan utk Anggaran (APBDes) (data-numgroup di
	// moneyFieldRp) — format tampilan + normalisasi digit polos saat submit.
	body = append(body, h.Script(h.Src("/static/numgroup.js"), h.Defer()))
	// BL-106: HP Kontak = input angka (data-phonenum di phoneField) — phonenum.js
	// membuang huruf/simbol sejak diketik. Tanpa JS pun aman (backend optPhone).
	body = append(body, h.Script(h.Src("/static/phonenum.js"), h.Defer()))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// accountFormHint = penjelasan singkat di bawah judul form, beda utk tambah vs
// sunting — sama pola dgn accountsTabDesc (penjelasan mengikuti konteks yang
// sedang dilihat, bukan satu kalimat generik). BL-108: penugasan CS dipindah ke
// halaman detail Customer Success desa, jadi hint mengarahkan ke sana (bukan
// lagi "kartu di bawah") — form ini murni profil desa.
func accountFormHint(isEdit bool) string {
	if isEdit {
		return "Ubah data desa ini. Penugasan CS (utama/cadangan) diatur di halaman Customer Success desa."
	}
	return "Lengkapi data desa baru untuk workspace ini. Penugasan CS (utama/cadangan) diatur di halaman Customer Success desa setelah tersimpan."
}

// assignCSSignal = signal Datastar boolean pengendali modal Penugasan CS.
const assignCSSignal = "assignOpen"

// assignCSTrigger = tombol pembuka modal Penugasan CS (klik → set signal true).
// BL-108 (revisi 9 Sep): penugasan CSM dibuka lewat modal, BUKAN kartu inline di
// halaman detail — sepasang dengan assignCSModal (signal sama).
func assignCSTrigger() g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("btn btn-sm min-h-11"),
		data.On("click", "$"+assignCSSignal+" = true"),
		g.Text("Penugasan CS"),
	)
}

// assignCSModal = modal berisi form penugasan CSM (utama + cadangan). Pola sama
// ConfirmModal/ChangelogModal (signal Datastar $assignOpen; inline display:none
// anti-FOUC; backdrop/tombol Batal menutup via signal). Form NATIVE POST ke
// action /assign → 303 balik ke CS detail (gotcha #16: navigasi tak lewat SSE).
// Sertakan SEKALI di halaman. Aksi berbeda dari simpan-field profil, jadi form
// & tombol submit terpisah. Parameter telanjang agar tak tergantung view manapun.
func assignCSModal(action, assignedCSM, backupCSM string, members []AccountMemberOption) g.Node {
	openExpr := "$" + assignCSSignal
	return h.Div(
		h.Class("fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"),
		g.Attr("style", "display:none"),
		data.Show(openExpr),
		data.On("click", openExpr+" = false"), // klik backdrop → tutup
		h.Div(
			// max-height via inline style: kelas arbitrer max-h-[80vh] tak ada di
			// app.css prebuilt & binary tailwind tak tersedia utk make css (pola sama
			// ChangelogModal). max-w-lg muat di 320px (p-4 luar).
			h.Class("card bg-base-100 shadow-lg w-full max-w-lg flex flex-col"),
			g.Attr("style", "max-height:80vh"),
			data.On("click", "evt.stopPropagation()"),
			h.Div(
				h.Class("flex items-center justify-between gap-2 px-5 pt-5 pb-3 border-b border-base-300"),
				h.H2(h.Class("text-lg font-semibold"), g.Text("Penugasan CS")),
				h.Button(
					h.Type("button"),
					h.Class("btn btn-ghost btn-sm btn-circle"),
					g.Attr("aria-label", "Tutup"),
					data.On("click", openExpr+" = false"),
					lucide.X(h.Class("size-5")),
				),
			),
			h.Div(
				h.Class("px-5 py-4 overflow-y-auto"),
				h.P(h.Class("text-sm text-base-content/60 mb-3"),
					g.Text("Menentukan siapa yang melihat desa ini di daftar mereka.")),
				h.FormEl(
					h.Method("post"), h.Action(action),
					h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
					memberSelect("CS Utama", "assigned_csm", assignedCSM, members),
					memberSelect("CS Cadangan", "backup_csm", backupCSM, members),
					h.Div(
						h.Class("sm:col-span-2 flex flex-wrap justify-end gap-2"),
						h.Button(h.Type("button"), h.Class("btn btn-ghost min-h-11"),
							data.On("click", openExpr+" = false"), g.Text("Batal")),
						h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
							g.Text("Simpan Penugasan")),
					),
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
