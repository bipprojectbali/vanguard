package handler

import (
	"net/http"

	"go_starter/internal/ui/pages/panel"
)

// sales_leads_import.go — BL-133: GET form unggah + unduh template CSV impor
// massal Lead. Aksi unggah (dry-run) & konfirmasi (tulis) ada di
// sales_leads_import_preview.go & sales_leads_import_confirm.go — dipisah
// agar tiap file di bawah ambang Route/Handler=150 (pola sama BL-63,
// accounts_import.go).
//
// Gerbang SAMA dgn LeadNew/LeadCreate (requireLeadWrite) utk SEMUA rute di
// sini — termasuk GET form & unduh template: keduanya prasyarat aksi tulis
// (impor), bukan tampilan data lead yang berdiri sendiri.
//
// TIDAK TERKAIT BL-63/BL-134 (keputusan final 14 Sep tasks.md): Lead hanya
// butuh district_id, bukan Account — konversi Lead→Account tetap fitur
// manual existing (LeadConvert), di luar cakupan impor massal ini.

// maxLeadImportRows/maxLeadImportFileBytes — batas terpisah dari
// maxImportRows/maxImportFileBytes milik Account (accounts_import.go): nilai
// sama (1000 baris, 2 MiB) tapi named const sendiri per modul agar
// perubahan batas salah satu modul tak diam-diam ikut mengubah yang lain.
const maxLeadImportRows = 1000
const maxLeadImportFileBytes = 2 << 20 // 2 MiB

// leadImportCSVHeader = urutan kolom template unduhan & referensi kolom yang
// dikenali parseLeadImportCSV. "lead_name"/"kode_kecamatan" wajib; sisanya
// opsional. Penyesuaian dari draf awal tasks.md (diminta user setelah rilis
// pertama BL-133): "lead_source" DIHAPUS (tak ada padanan di form manual
// Tambah Lead, lihat sales_leads_form.go — kolom itu tak pernah diisi lewat
// form sejak BL-82 mempersempitnya ke dropdown terkurasi); "mobile_phone" &
// "whatsapp" DIGABUNG jadi SATU kolom "mobile_phone" — form manual
// (phoneNumField "HP / WhatsApp") sudah menggabungnya, CSV kini mengikuti.
var leadImportCSVHeader = []string{
	"lead_name", "kode_kecamatan", "contact_person", "job_title",
	"lead_status", "rating", "estimated_value", "mobile_phone", "email",
}

// leadImportCSVSampleRow = satu baris contoh di template unduhan — nilai sah
// nyata (enum lead_status/rating, kode Kecamatan format 3-segmen Kemendagri)
// agar operator langsung melihat format yang diterima. rating HARUS salah
// satu dari "Hot"/"Warm"/"Cold" (validLeadRatings, sales_leads_form.go) —
// nilai lain ditolak sbg baris invalid.
var leadImportCSVSampleRow = []string{
	"Desa Sukamaju", "32.01.01", "Budi Santoso", "Kepala Desa",
	"New", "Warm", "50000000", "081234567890", "budi@example.id",
}

// leadImportDistrictNameCol = kolom "Kecamatan" di tabel pratinjau — SATU-
// SATUNYA kolom BUKAN kolom CSV asli (tak ada di leadImportCSVHeader/
// leadImportColumnMap, jadi TAK muncul di template unduhan). Diisi dari hasil
// resolusi kode_kecamatan → master data (resolveLeadImportRows), disisipkan
// tepat setelah "kode_kecamatan" (sales_leads_import_preview.go) — pola sama
// importVillageNameCol milik BL-63.
var leadImportDistrictNameCol = panel.LeadImportColumn{Key: "nama_kecamatan", Label: "Kecamatan"}

// leadImportColumnLabels = label kolom manusiawi utk header tabel pratinjau —
// KUNCI & URUTAN pemakaian SAMA dgn leadImportCSVHeader.
var leadImportColumnLabels = map[string]string{
	"lead_name":       "Nama Lead",
	"kode_kecamatan":  "Kode Kecamatan",
	"contact_person":  "Kontak",
	"job_title":       "Jabatan",
	"lead_status":     "Status",
	"rating":          "Rating",
	"estimated_value": "Estimasi Nilai",
	"mobile_phone":    "HP / WhatsApp", // label SAMA persis dgn form manual (phoneNumField, sales_leads_form.go)
	"email":           "Email",
}

// LeadImportForm — GET /leads/import. Form unggah CSV kosong (native
// multipart — gotcha #16 + belum ada presedan Datastar utk file upload).
func (h *Handler) LeadImportForm(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.LeadImportFormView{
		Base:         base,
		Action:       base + "/leads/import",
		TemplateURL:  base + "/leads/import/template",
		Err:          wsErrMsg(r.URL.Query().Get("err")),
		MaxRows:      maxLeadImportRows,
		MaxFileBytes: maxLeadImportFileBytes,
	}
	h.renderWorkspaceShell(w, r, "Impor Lead", "/leads", panel.LeadImportForm(v))
}

// LeadImportTemplate — GET /leads/import/template. Unduh CSV contoh (header +
// 1 baris) agar operator tak menebak nama/urutan kolom.
func (h *Handler) LeadImportTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	if err := writeCSV(w, "template-impor-lead", leadImportCSVHeader, [][]string{leadImportCSVSampleRow}); err != nil {
		h.Log.Error("leads import: write template", "err", err)
	}
}
