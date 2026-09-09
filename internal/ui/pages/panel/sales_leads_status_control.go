package panel

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_leads_status_control.go — kontrol "Ubah Status" di HALAMAN DETAIL lead
// (BL-83). Transisi status = aksi tersendiri, dipisah dari sunting profil
// (LeadForm). Backend (LeadStatus, sales_leads_status.go) tetap penjaga
// sesungguhnya (validLeadStatuses + guard converted).
//
// BL-119 (wireframe 4.1): kontrol TAK lagi kartu inline yang selalu tampil —
// kini tombol "Ubah Status" di header (leadStatusTrigger) yang membuka MODAL
// (leadStatusModal, pola assignCSModal: signal $leadStatusOpen, inline
// display:none anti-FOUC). Form di dalam modal TETAP NATIVE POST → 303 (gotcha
// #16); select ter-bind $leadstatus agar "Alasan Unqualified" muncul kondisional
// tanpa round-trip.

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

// leadStatusSignal = signal Datastar boolean pengendali modal Ubah Status.
const leadStatusSignal = "leadStatusOpen"

// leadStatusTrigger = tombol pembuka modal Ubah Status (klik → set signal true).
// Dipasang di grup aksi header (leadDetailActions), hanya untuk aktor boleh-tulis
// atas lead belum dikonversi — sepasang dengan leadStatusModal (signal sama).
func leadStatusTrigger() g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("btn btn-sm min-h-11"),
		data.On("click", "$"+leadStatusSignal+" = true"),
		g.Text("Ubah Status"),
	)
}

// leadStatusModal = modal ganti status: form NATIVE POST (gotcha #16). <select>
// di-bind ke signal $leadstatus (data.Bind) → "Alasan Unqualified" (BL-80) tampil
// hanya saat Unqualified tanpa round-trip. Tak dipakai FormPostSelect karena
// alasan Unqualified perlu ikut terkumpul saat submit. Backend tetap penjaga:
// validasi status + buang alasan basi saat status ≠ Unqualified.
//
// Hanya dirender saat aktor boleh tulis & lead belum dikonversi (LeadDetail).
// Lead terkonversi = status terminal; modal + tombol disembunyikan + query menolak.
func leadStatusModal(v LeadDetailView, base string) g.Node {
	openExpr := "$" + leadStatusSignal
	sel := []g.Node{
		h.ID("f-lead_status"), h.Name("lead_status"),
		data.Bind("leadstatus"),
		h.Class("select text-base w-full"),
		h.Required(),
	}
	return h.Div(
		h.Class("fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"),
		g.Attr("style", "display:none"),
		data.Show(openExpr),
		data.On("click", openExpr+" = false"), // klik backdrop → tutup
		h.Div(
			h.Class("card bg-base-100 shadow-lg w-full max-w-lg flex flex-col"),
			g.Attr("style", "max-height:80vh"),
			data.On("click", "evt.stopPropagation()"),
			h.Div(
				h.Class("flex items-center justify-between gap-2 px-5 pt-5 pb-3 border-b border-base-300"),
				h.H2(h.Class("text-lg font-semibold"), g.Text("Ubah Status")),
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
					// required KONDISIONAL (data.Attr) — HANYA saat Unqualified; field
					// tersembunyi (display:none) tak boleh required (submit status lain
					// gagal "invalid form control not focusable"). Ekspresi sama data-show.
					showWhen(unqualifiedReasonShowExpr, "min-w-0",
						textareaField("Alasan Unqualified", "unqualified_reason", v.UnqualifiedReason,
							data.Attr("required", unqualifiedReasonShowExpr))),
					h.Div(
						h.Class("flex flex-wrap justify-end gap-2"),
						h.Button(h.Type("button"), h.Class("btn btn-ghost min-h-11"),
							data.On("click", openExpr+" = false"), g.Text("Batal")),
						h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
							g.Text("Simpan Status")),
					),
				),
			),
		),
	)
}
