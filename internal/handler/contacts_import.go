package handler

import (
	"net/http"

	"go_starter/internal/ui/pages/panel"
)

// contacts_import.go — BL-134: GET form unggah + unduh template CSV. Aksi
// unggah (dry-run) & konfirmasi (tulis) ada di contacts_import_preview.go &
// contacts_import_confirm.go — dipisah agar tiap file di bawah ambang
// Route/Handler=150. Pola & alasan pemisahan SAMA accounts_import.go (BL-63).
//
// Gerbang SAMA dgn ContactCreate/ContactCreateGlobal (requireContactWrite)
// utk SEMUA rute di sini — termasuk GET form & unduh template: keduanya
// prasyarat aksi tulis (impor), bukan tampilan data kontak berdiri sendiri.
// maxImportRows/maxImportFileBytes DIPAKAI ULANG dari accounts_import.go
// (package sama, threshold bisnis satu tempat utk SEMUA impor CSV).

// contactImportCSVHeader = urutan kolom template unduhan & referensi kolom
// yang dikenali parseContactImportCSV. "kode_desa"/"first_name" wajib;
// sisanya opsional. Kolom SAMA persis nama field parseContactForm
// (contacts_form.go) — beda dgn importCSVHeader (BL-63) yang berbahasa
// Indonesia lalu dipetakan importColumnMap.
//
// Diperkecil dari rencana awal 20 kolom (keputusan 15 Sep — lihat catatan di
// docs/crm/tasks.md BL-134): kolom yang di-drop (salutation, position_category,
// contact_role, preferred_channel, is_technical_contact, term_period,
// office_phone, email, mailing_address, city, postal_code, email_opt_out,
// do_not_contact) TETAP bisa diisi lewat form tambah/ubah kontak manual — hanya
// tak lagi ditawarkan lewat jalur impor massal, demi mengecilkan risiko satu
// baris tertolak (all-or-nothing) akibat enum salah ketik.
var contactImportCSVHeader = []string{
	"kode_desa", "first_name", "last_name", "job_title",
	"mobile_phone", "whatsapp_number", "is_primary_contact",
}

// contactImportCSVSampleRow = satu baris contoh di template unduhan — nilai
// valid nyata, urutan SAMA dgn contactImportCSVHeader. is_primary_contact
// HANYA menerima "true"/"false"/kosong (lihat normalizeContactPrimaryFlag,
// contacts_import_parse.go) — beda dari optBool checkbox form manual.
var contactImportCSVSampleRow = []string{
	"32.01.01.2001", "Budi", "Santoso", "Kepala Desa",
	"081234567890", "081234567890", "true",
}

// contactImportVillageNameCol = kolom "Desa" di tabel pratinjau — SATU-
// SATUNYA kolom yang BUKAN kolom CSV asli, diisi dari hasil resolusi
// village_code → account_id (resolveContactImportRows), disisipkan tepat
// setelah "kode_desa" (contacts_import_preview.go).
var contactImportVillageNameCol = panel.AccountImportColumn{Key: "nama_desa", Label: "Desa"}

// contactImportColumnLabels = label kolom manusiawi utk header tabel
// pratinjau — KUNCI & URUTAN pemakaian SAMA dgn contactImportCSVHeader.
var contactImportColumnLabels = map[string]string{
	"kode_desa":          "Kode Desa",
	"first_name":         "Nama Depan",
	"last_name":          "Nama Belakang",
	"job_title":          "Jabatan",
	"mobile_phone":       "HP",
	"whatsapp_number":    "WhatsApp",
	"is_primary_contact": "Kontak Utama (true/false)",
}

// ContactImportForm — GET /contacts/import. Form unggah CSV kosong (native
// multipart — gotcha #16, jangan dicampur SSE).
func (h *Handler) ContactImportForm(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.ContactImportFormView{
		Base:         base,
		Action:       base + "/contacts/import",
		TemplateURL:  base + "/contacts/import/template",
		Err:          wsErrMsg(r.URL.Query().Get("err")),
		MaxRows:      maxImportRows,
		MaxFileBytes: maxImportFileBytes,
	}
	h.renderWorkspaceShell(w, r, "Impor Kontak", "/contacts", panel.ContactImportForm(v))
}

// ContactImportTemplate — GET /contacts/import/template. Unduh CSV contoh
// (header + 1 baris) agar operator tak menebak nama/urutan/enum kolom.
func (h *Handler) ContactImportTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	if err := writeCSV(w, "template-impor-kontak", contactImportCSVHeader, [][]string{contactImportCSVSampleRow}); err != nil {
		h.Log.Error("contacts import: write template", "err", err)
	}
}
