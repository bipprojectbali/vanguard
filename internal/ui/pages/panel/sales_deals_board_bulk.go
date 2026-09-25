package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_deals_board_bulk.go — BL-75: form NATIVE (gotcha #16, native POST→303)
// + modal terminal utk drag-drop papan Kanban. static/dealboard.js merakit isi
// form (deal_id berulang, stage, mode, field reason) sebelum submit. Toggle
// modal/mode PLAIN JS (classlist/style) — spec BL-75 eksplisit MELARANG
// Datastar di fitur ini (beda dari dealStageModal/dealStageControl yang boleh
// Datastar krn ranah form single-deal biasa). Dirender HANYA di cabang Kanban
// non-table (lihat DealPipeline, sales_deals.go).
//
// Kontrak markup (id/name LITERAL, dikonsumsi static/dealboard.js — ubah
// keduanya BERSAMAAN, jangan sendiri-sendiri):
//   - #deal-stage-bulk-form             <form method=post action=.../deals/
//     stage-bulk> — target submit, baik langsung (non-terminal) maupun via
//     tombol Simpan modal (terminal).
//   - #deal-stage-bulk-ids              container kosong; JS append <input
//     type=hidden name=deal_id value=ID> satu per kartu terpilih sblm submit.
//   - #deal-stage-bulk-stage            <input type=hidden name=stage> — JS
//     set value = stage tujuan sblm submit.
//   - #deal-stage-bulk-mode             <input type=hidden name=mode> — JS
//     sinkron dari radio [name=ui-mode] aktif ("shared"|"individual").
//   - #deal-stage-bulk-modal            wrapper modal, style=display:none
//     default; JS toggle .style.display (tampil HANYA stage tujuan terminal).
//   - #deal-stage-bulk-summary          teks ringkas ("N deal → <stage>") — JS
//     isi saat modal dibuka.
//   - [name=ui-mode]                    radio shared/individual; klik → JS
//     toggle #deal-stage-bulk-shared-fields vs -individual-rows + isi #deal-
//     stage-bulk-mode.
//   - #deal-stage-bulk-shared-fields    field mode "shared" (satu nilai utk
//     seluruh seleksi) — name field APA ADANYA (win_loss_reason dst).
//   - #deal-stage-bulk-individual-rows  kosong default; JS append baris
//     ter-clone dari template, satu per deal terpilih (mode individual).
//   - #deal-stage-bulk-row-template     <template> satu baris field per-deal;
//     JS clone lalu ganti `name` field jadi `<name>__<dealId>` + isi label
//     nama deal (.deal-stage-bulk-row-label).
//   - [data-stage-section=won|lost]     field relevan HANYA salah satu target
//     terminal — JS toggle sesuai stage tujuan aktif saat modal dibuka.
//   - [data-bulk-cancel]                tombol/area penutup modal TANPA
//     submit (X, Batal, backdrop).
//   - #deal-stage-rules                 <script type=application/json> berisi
//     StageRulesJSON (map stage→[nextStage,...], dari nextDealStages —
//     ENUM INTERNAL, bukan input user, aman ditanam mentah, gotcha #15).
//     dealboard.js JSON.parse ini sbg satu-satunya sumber aturan transisi.

// dealBulkBoard = form bulk + modal terminal, dirender SEKALI per halaman
// (bukan per-kartu) — dealboard.js mengisi & submit form ini dari kartu mana
// pun yang di-drop.
func dealBulkBoard(v DealPipelineView) g.Node {
	return h.FormEl(
		h.ID("deal-stage-bulk-form"),
		h.Method("post"), h.Action(v.Base+"/deals/stage-bulk"),
		h.Div(h.ID("deal-stage-bulk-ids")),
		h.Input(h.Type("hidden"), h.ID("deal-stage-bulk-stage"), h.Name("stage")),
		h.Input(h.Type("hidden"), h.ID("deal-stage-bulk-mode"), h.Name("mode"), h.Value("shared")),
		h.Script(h.Type("application/json"), h.ID("deal-stage-rules"), g.Raw(v.StageRulesJSON)),
		dealBulkModal(v),
	)
}

// dealBulkModal = shell modal terminal (tersembunyi default, plain-JS toggle —
// BUKAN data.Show). Pola visual mirror dealStageModal, tapi interaksi murni JS.
func dealBulkModal(v DealPipelineView) g.Node {
	return h.Div(
		h.ID("deal-stage-bulk-modal"),
		h.Class("fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"),
		g.Attr("style", "display:none"),
		// Klik backdrop = tutup: JS cek e.target === elemen ini (bukan
		// data-bulk-cancel + stopPropagation — atribut onclick inline DIBLOKIR
		// CSP, script-src tanpa unsafe-inline, lihat internal/mw/security.go).
		h.Div(
			h.Class("card bg-base-100 shadow-lg w-full max-w-lg flex flex-col min-w-0"),
			g.Attr("style", "max-height:80vh"),
			h.Div(
				h.Class("flex items-center justify-between gap-2 px-5 pt-5 pb-3 border-b border-base-300"),
				h.H2(h.Class("text-lg font-semibold"), g.Text("Pindahkan Tahap")),
				h.Button(
					h.Type("button"), h.Class("btn btn-ghost btn-sm btn-circle"),
					g.Attr("aria-label", "Tutup"), g.Attr("data-bulk-cancel", ""),
					g.Text("×"),
				),
			),
			h.Div(
				h.Class("px-5 py-4 overflow-y-auto grid gap-3 min-w-0"),
				h.P(h.ID("deal-stage-bulk-summary"), h.Class("text-sm text-base-content/70")),
				dealBulkModeToggle(),
				h.Div(h.ID("deal-stage-bulk-shared-fields"), h.Class("grid gap-3 min-w-0"),
					dealBulkReasonFields(v)),
				h.Div(h.ID("deal-stage-bulk-individual-rows"), h.Class("grid gap-3 min-w-0"),
					g.Attr("style", "display:none")),
				h.Template(h.ID("deal-stage-bulk-row-template"),
					h.Div(
						h.Class("border border-base-300 rounded-box p-3 grid gap-2 min-w-0"),
						h.P(h.Class("text-sm font-medium deal-stage-bulk-row-label")),
						dealBulkReasonFields(v),
					),
				),
			),
			h.Div(
				h.Class("flex flex-wrap justify-end gap-2 px-5 pb-5"),
				h.Button(h.Type("button"), h.Class("btn btn-ghost min-h-11"),
					g.Attr("data-bulk-cancel", ""), g.Text("Batal")),
				h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
					g.Text("Simpan")),
			),
		),
	)
}

// dealBulkModeToggle = radio shared/individual — JS murni (tanpa data.Bind)
// menyinkronkan hidden #deal-stage-bulk-mode & memilih blok field yang tampil.
func dealBulkModeToggle() g.Node {
	radio := func(val, label string, checked bool) g.Node {
		attrs := []g.Node{
			h.Type("radio"), h.Name("ui-mode"), h.Value(val),
			h.Class("radio radio-sm"),
		}
		if checked {
			attrs = append(attrs, h.Checked())
		}
		return h.Label(
			h.Class("flex items-center gap-2 text-sm"),
			h.Input(attrs...), g.Text(label),
		)
	}
	return h.Div(
		h.Class("flex flex-wrap gap-4"),
		radio("shared", "Satu alasan untuk semua", true),
		radio("individual", "Alasan per-deal", false),
	)
}

// dealBulkReasonFields = field reason dipakai KEDUA target terminal — win_loss_reason
// (Won & Lost), subscription_status (Won saja), loss_reason_code+loss_notes
// (Lost saja). data-stage-section menandai relevansi utk toggle JS saat modal
// dibuka; keduanya dirender SEKALIGUS (server tak tahu target sebelum drop),
// JS yang menyembunyikan bagian tak relevan. Dipakai DUA tempat: blok "shared"
// (name asli) & template baris "individual" (JS mengganti name jadi
// `<name>__<id>` saat clone) — makanya fungsi ini TANPA parameter id/sufiks.
func dealBulkReasonFields(v DealPipelineView) g.Node {
	return h.Div(
		h.Class("grid gap-3 min-w-0"),
		field("Alasan Menang/Kalah", "win_loss_reason", "", false, "text"),
		g.If(len(v.WonSubStatuses) > 0, h.Div(
			g.Attr("data-stage-section", "won"),
			selectField("Status Langganan Awal", "subscription_status",
				v.WonSubStatuses[0], v.WonSubStatuses, false,
				"Deal menang membuat langganan otomatis untuk desa & paket deal ini."),
		)),
		h.Div(
			g.Attr("data-stage-section", "lost"),
			selectField("Alasan Kalah (Kode)", "loss_reason_code", "", v.LossReasonCodes, false,
				"Wajib dipilih saat deal Closed Lost — dipakai laporan Win/Loss."),
		),
		h.Div(
			g.Attr("data-stage-section", "lost"),
			textareaField("Catatan Kekalahan", "loss_notes", ""),
		),
	)
}
