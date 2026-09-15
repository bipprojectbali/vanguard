package handler

import (
	"net/http"

	"go_starter/internal/ui/pages/panel"
)

// accounts_import.go — BL-63: GET form unggah + unduh template CSV. Aksi
// unggah (dry-run) & konfirmasi (tulis) ada di accounts_import_preview.go &
// accounts_import_confirm.go — dipisah agar tiap file di bawah ambang
// Route/Handler=150.
//
// Gerbang SAMA dgn AccountNew (requireAccountWrite) utk SEMUA rute di sini —
// termasuk GET form & unduh template: keduanya prasyarat aksi tulis (impor),
// bukan tampilan data desa yang berdiri sendiri, jadi tak ada alasan
// melonggarkannya utk role read-only (support).

// maxImportRows membatasi jumlah BARIS DATA per file (di luar header) — selaras
// batas yang diusulkan BL-133 utk operasi massal. Melampauinya = file ditolak
// SEBELUM baris mana pun diproses (import_too_many), bukan dipotong senyap.
const maxImportRows = 1000

// maxImportFileBytes membatasi ukuran file unggahan. 2 MiB longgar utk ~1000
// baris teks CSV (jauh di atas kebutuhan realistis) tapi mencegah unggahan
// besar yang menahan memori request (Rule 13 — I/O berat harus dibatasi).
const maxImportFileBytes = 2 << 20 // 2 MiB

// importCSVHeader = urutan kolom template unduhan & referensi kolom yang
// dikenali parseImportCSV. "kode_desa"/"tipe_akun" wajib; sisanya opsional.
var importCSVHeader = []string{
	"kode_desa", "tipe_akun", "email_pemilik", "alamat", "kode_pos", "website",
	"deskripsi", "status_desa", "klasifikasi_idm", "jumlah_penduduk",
	"jumlah_dusun", "anggaran_apbdes", "telepon_kantor", "email_kantor", "hp_kontak",
}

// importCSVSampleRow = satu baris contoh di template unduhan — nilai valid
// nyata (bukan placeholder "xxx") agar operator langsung melihat format yang
// diterima (mis. tipe_akun harus salah satu enum, bukan teks bebas).
var importCSVSampleRow = []string{
	"32.01.01.2001", "prospect", "", "Jl. Merdeka No. 1", "40311", "",
	"", "Desa", "Berkembang", "3000", "8", "500000000", "", "", "",
}

// importVillageNameCol = kolom "Desa" di tabel pratinjau — SATU-SATUNYA kolom
// yang BUKAN kolom CSV asli (bukan bagian importCSVHeader/importColumnMap,
// jadi TAK muncul di template unduhan). Diisi dari hasil resolusi
// village_code → master data (resolveImportRows), disisipkan tepat setelah
// "kode_desa" (accounts_import_preview.go) agar operator langsung lihat nama
// desa yang dimaksud tanpa buka data master terpisah.
var importVillageNameCol = panel.AccountImportColumn{Key: "nama_desa", Label: "Desa"}

// importColumnLabels = label kolom manusiawi utk header tabel pratinjau —
// KUNCI & URUTAN pemakaian SAMA dgn importCSVHeader, mencegah drift antara
// kolom yang diterima parser dan kolom yang ditampilkan ke operator.
var importColumnLabels = map[string]string{
	"kode_desa":       "Kode Desa",
	"tipe_akun":       "Tipe Akun",
	"email_pemilik":   "Email Pemilik",
	"alamat":          "Alamat",
	"kode_pos":        "Kode Pos",
	"website":         "Website",
	"deskripsi":       "Deskripsi",
	"status_desa":     "Status Desa",
	"klasifikasi_idm": "Klasifikasi IDM",
	"jumlah_penduduk": "Jumlah Penduduk",
	"jumlah_dusun":    "Jumlah Dusun",
	"anggaran_apbdes": "Anggaran APBDes",
	"telepon_kantor":  "Telepon Kantor",
	"email_kantor":    "Email Kantor",
	"hp_kontak":       "HP Kontak",
}

// AccountImportForm — GET /accounts/import. Form unggah CSV kosong (native
// multipart — gotcha #16 + belum ada presedan Datastar utk file upload,
// jangan dicampur SSE).
func (h *Handler) AccountImportForm(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.AccountImportFormView{
		Base:         base,
		Action:       base + "/accounts/import",
		TemplateURL:  base + "/accounts/import/template",
		Err:          wsErrMsg(r.URL.Query().Get("err")),
		MaxRows:      maxImportRows,
		MaxFileBytes: maxImportFileBytes,
	}
	h.renderWorkspaceShell(w, r, "Impor Desa", "/accounts", panel.AccountImportForm(v))
}

// AccountImportTemplate — GET /accounts/import/template. Unduh CSV contoh
// (header + 1 baris) agar operator tak menebak nama/urutan kolom.
func (h *Handler) AccountImportTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	if err := writeCSV(w, "template-impor-desa", importCSVHeader, [][]string{importCSVSampleRow}); err != nil {
		h.Log.Error("accounts import: write template", "err", err)
	}
}
