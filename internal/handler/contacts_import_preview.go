package handler

import (
	"io"
	"net/http"
	"strings"

	"go_starter/internal/ui/pages/panel"
)

// contacts_import_preview.go — BL-134: ContactImportPreview (POST
// /contacts/import), dry-run: parse+resolve SEMUA baris TANPA menulis DB,
// lalu render pratinjau. CSV mentah dibawa ke konfirmasi lewat hidden field
// (raw_csv) — divalidasi ULANG penuh di ContactImportConfirm
// (contacts_import_confirm.go), bukan dipercaya dari sini (menutup celah
// TOCTOU by construction). Pola identik accounts_import_preview.go (BL-63).

// ContactImportPreview — POST /contacts/import. Native multipart upload
// (gotcha #16 — bukan Datastar). Galat file/parse → redirect ke form unggah
// dgn ?err=; validasi per-baris (desa tak ketemu, primary ganda, dst) TIDAK
// menggagalkan request ini — semua ditampilkan di tabel pratinjau.
func (h *Handler) ContactImportPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}

	// Batasi ukuran BODY total (bukan cuma file) — mencegah unggahan besar
	// menahan memori request (Rule 13). Melampaui batas → ParseMultipartForm
	// gagal dgn "http: request body too large".
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxImportFileBytes))
	if err := r.ParseMultipartForm(int64(maxImportFileBytes)); err != nil {
		wsRedirect(w, r, "/contacts/import", "import_invalid")
		return
	}

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		wsRedirect(w, r, "/contacts/import", "import_invalid")
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		wsRedirect(w, r, "/contacts/import", "import_invalid")
		return
	}
	rawCSV := string(raw)

	rows, code := parseContactImportCSV(strings.NewReader(rawCSV))
	if code != "" {
		wsRedirect(w, r, "/contacts/import", code)
		return
	}

	ctx := r.Context()
	resolved, anyFailed := resolveContactImportRows(ctx, h, rows)

	// Kolom "Desa" BUKAN kolom CSV asli — hasil RESOLUSI village_code →
	// account_id (resolveContactImportRows), disisipkan tepat setelah
	// "kode_desa" agar operator langsung lihat desa yang dimaksud kode itu.
	columns := make([]panel.AccountImportColumn, 0, len(contactImportCSVHeader)+1)
	for _, key := range contactImportCSVHeader {
		columns = append(columns, panel.AccountImportColumn{Key: key, Label: contactImportColumnLabels[key]})
		if key == "kode_desa" {
			columns = append(columns, contactImportVillageNameCol)
		}
	}

	base := wsPath(slugFromRequest(r), "")
	v := panel.ContactImportPreviewView{
		Base: base,
		Upload: panel.ContactImportFormView{
			Base:         base,
			Action:       base + "/contacts/import",
			TemplateURL:  base + "/contacts/import/template",
			MaxRows:      maxImportRows,
			MaxFileBytes: maxImportFileBytes,
		},
		ConfirmURL: base + "/contacts/import/confirm",
		RawCSV:     rawCSV,
		AnyFailed:  anyFailed,
		Columns:    columns,
		Rows:       make([]panel.ContactImportPreviewRow, 0, len(resolved)),
	}
	// rows[i] & resolved[i] selalu berpasangan (resolveContactImportRows
	// menjaga urutan & jumlah sama dgn input) — nilai MENTAH dari rows
	// dipakai agar tabel tetap menampilkan data baris walau gagal ter-resolve.
	for i, rr := range resolved {
		row := panel.ContactImportPreviewRow{Values: rawContactImportRowValues(rows[i])}
		name := rr.villageName
		if name == "" {
			name = "-"
		}
		row.Values[contactImportVillageNameCol.Key] = name
		if rr.errCode != "" {
			row.ErrMsg = wsErrMsg(rr.errCode)
			v.InvalidCount++
		} else {
			v.ValidCount++
		}
		v.Rows = append(v.Rows, row)
	}

	h.renderWorkspaceShell(w, r, "Impor Kontak", "/contacts", panel.ContactImportPreview(v))
}

// rawContactImportRowValues memetakan balik satu contactImportRow ke
// map[header CSV]nilai APA ADANYA (belum diresolusi) — dipakai pratinjau
// agar kolom tabel persis mengikuti contactImportCSVHeader, termasuk utk
// baris yang gagal validasi.
func rawContactImportRowValues(row contactImportRow) map[string]string {
	values := make(map[string]string, len(contactImportCSVHeader))
	values["kode_desa"] = row.villageCode
	for header, field := range contactImportColumnMap {
		values[header] = row.fields[field]
	}
	return values
}
