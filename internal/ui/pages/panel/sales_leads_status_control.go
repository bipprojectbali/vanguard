package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_leads_status_control.go — kartu "Ubah Status" di HALAMAN DETAIL lead
// (BL-83). Transisi status = aksi tersendiri, dipisah dari sunting profil
// (LeadForm), cermin dealStageControl (sales_deals_detail_stage.go) namun TANPA
// stepper: status lead bercabang (Unqualified/Converted bukan langkah linear),
// jadi kontrol = kartu select + submit, bukan garis tahap. Kontrol NATIVE POST →
// 303 (gotcha #16). Backend (LeadStatus, sales_leads_status.go) tetap penjaga
// sesungguhnya (validLeadStatuses + guard converted).

// leadUnqualified = nilai status yang mengaktifkan field "Alasan Unqualified"
// (BL-80). Dirujuk ekspresi data-show; disatukan sebagai const agar markup & backend
// tak menyimpang literalnya.
const leadUnqualified = "Unqualified"

// unqualifiedReasonShowExpr = ekspresi data-show field "Alasan Unqualified":
// tampil hanya saat $leadstatus == "Unqualified". Dirakit dari const (bukan
// literal tersebar) agar sinkron dgn nilai enum.
const unqualifiedReasonShowExpr = "$leadstatus == '" + leadUnqualified + "'"

// leadStatusLegend = makna tiap status lead (BL-3). Urut = alur kualifikasi
// (New → Contacted → Qualified, atau bercabang ke Unqualified). 'Converted' tak
// di sini: itu status sistem hasil konversi, tak bisa dipilih manual (lihat
// validLeadStatuses). HARUS himpunan yang sama dengan leadStatusOptions.
var leadStatusLegend = [][2]string{
	{"New", "Baru masuk, belum dihubungi."},
	{"Contacted", "Sudah dihubungi, belum dikualifikasi."},
	{"Qualified", "Cocok dan siap dikonversi jadi Deal."},
	{"Unqualified", "Tak cocok atau tak berminat (isi alasannya)."},
}

// leadStatusControl = kartu ganti status: NATIVE POST (gotcha #16). <select>
// di-bind ke signal $leadstatus (data.Bind) → "Alasan Unqualified" (BL-80) tampil
// hanya saat Unqualified tanpa round-trip. Tak dipakai FormPostSelect karena
// alasan Unqualified perlu ikut terkumpul saat submit. Backend tetap penjaga:
// validasi status + buang alasan basi saat status ≠ Unqualified.
//
// Hanya dirender saat aktor boleh tulis & lead belum dikonversi (leadDetailView).
// Lead terkonversi = status terminal; kontrol disembunyikan + query menolak.
func leadStatusControl(v LeadDetailView, base string) g.Node {
	sel := []g.Node{
		h.ID("f-lead_status"), h.Name("lead_status"),
		data.Bind("leadstatus"),
		h.Class("select text-base w-full"),
		h.Required(),
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Ubah Status")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Ubah status kualifikasi lead. Status Unqualified minta alasannya.")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/status"),
				// Signal $leadstatus diinisialisasi dari status TERSIMPAN → no-FOUC:
				// lead Unqualified langsung menampilkan field alasannya. Efemeral
				// (state form), bukan data dikirim ke server.
				data.Signals(map[string]any{"leadstatus": v.Status}),
				h.Class("grid gap-3 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0"),
					labelWithLegend("Status", "f-lead_status", true, leadStatusLegend),
					h.Select(append(sel, g.Group(enumOptions(v.Status, v.Statuses, false)))...),
				),
				// BL-80: alasan Unqualified HANYA bermakna saat status Unqualified —
				// disembunyikan (data-show) untuk status lain. data-show = jaring UX
				// klien (display:none, field TETAP terkirim); backend (LeadStatus)
				// penegak: nilai basi dibuang saat status ≠ Unqualified.
				showWhen(unqualifiedReasonShowExpr, "min-w-0",
					textareaField("Alasan Unqualified", "unqualified_reason", v.UnqualifiedReason)),
				h.Div(
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Status")),
				),
			),
		),
	)
}
