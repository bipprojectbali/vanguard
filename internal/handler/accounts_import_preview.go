package handler

import (
	"io"
	"net/http"
	"strings"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// accounts_import_preview.go — BL-63: AccountImportPreview (POST
// /accounts/import), dry-run: parse+resolve SEMUA baris TANPA menulis DB, lalu
// render pratinjau. CSV mentah dibawa ke konfirmasi lewat hidden field
// (raw_csv) — divalidasi ULANG penuh di AccountImportConfirm (accounts_import_confirm.go),
// bukan dipercaya dari sini (menutup celah TOCTOU by construction).

// AccountImportPreview — POST /accounts/import. Native multipart upload
// (gotcha #16 — bukan Datastar). Galat file/parse → redirect ke form unggah
// dgn ?err=; validasi per-baris (village_code tak ketemu, dst) TIDAK
// menggagalkan request ini — semua ditampilkan di tabel pratinjau agar
// operator tahu PERSIS baris mana yang harus diperbaiki di filenya.
func (h *Handler) AccountImportPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()

	// Batasi ukuran BODY total (bukan cuma file) — mencegah unggahan besar
	// menahan memori request (Rule 13). Melampaui batas → ParseMultipartForm
	// gagal dgn "http: request body too large".
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxImportFileBytes))
	if err := r.ParseMultipartForm(int64(maxImportFileBytes)); err != nil {
		wsRedirect(w, r, "/accounts/import", "import_invalid")
		return
	}

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		wsRedirect(w, r, "/accounts/import", "import_invalid")
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		wsRedirect(w, r, "/accounts/import", "import_invalid")
		return
	}
	rawCSV := string(raw)

	rows, code := parseImportCSV(strings.NewReader(rawCSV))
	if code != "" {
		wsRedirect(w, r, "/accounts/import", code)
		return
	}

	tenantID := session.TenantID(ctx)
	resolved, anyFailed := resolveImportRows(ctx, h, tenantID, rows)

	// Kolom "Desa" BUKAN kolom CSV (importCSVHeader tak berubah, template
	// unduhan tetap sama) — ini nama hasil RESOLUSI village_code ke master
	// data (rr.villageName, resolveImportRows), disisipkan tepat setelah
	// "Kode Desa" agar operator langsung lihat desa mana yang dimaksud kode
	// itu tanpa buka tab lain. importVillageNameCol dipisah dari
	// importColumnLabels krn key-nya sengaja tak ada di importColumnMap
	// (bukan field yang di-parse dari CSV).
	columns := make([]panel.AccountImportColumn, 0, len(importCSVHeader)+1)
	for _, key := range importCSVHeader {
		columns = append(columns, panel.AccountImportColumn{Key: key, Label: importColumnLabels[key]})
		if key == "kode_desa" {
			columns = append(columns, importVillageNameCol)
		}
	}

	base := wsPath(slugFromRequest(r), "")
	v := panel.AccountImportPreviewView{
		Base: base,
		Upload: panel.AccountImportFormView{
			Base:         base,
			Action:       base + "/accounts/import",
			TemplateURL:  base + "/accounts/import/template",
			MaxRows:      maxImportRows,
			MaxFileBytes: maxImportFileBytes,
		},
		ConfirmURL: base + "/accounts/import/confirm",
		RawCSV:     rawCSV,
		AnyFailed:  anyFailed,
		Columns:    columns,
		Rows:       make([]panel.AccountImportPreviewRow, 0, len(resolved)),
	}
	// rows[i] & resolved[i] selalu berpasangan (resolveImportRows menjaga
	// urutan & jumlah sama dgn input) — nilai MENTAH dari rows dipakai agar
	// tabel tetap menampilkan data baris walau gagal ter-resolve (rr.form
	// kosong pada baris error). ValidCount/InvalidCount dihitung di sini
	// (bukan di view) — view murni-data, tak boleh menghitung ulang dari Rows.
	for i, rr := range resolved {
		row := panel.AccountImportPreviewRow{Values: rawImportRowValues(rows[i])}
		// villageName kosong pada baris gagal (belum sempat ter-resolve) ATAU
		// bila master data ternyata punya nama kosong — tampilkan "-" alih-
		// alih sel kosong yang gampang terlihat seperti data hilang/rusak.
		name := rr.villageName
		if name == "" {
			name = "-"
		}
		row.Values[importVillageNameCol.Key] = name
		if rr.errCode != "" {
			row.ErrMsg = wsErrMsg(rr.errCode)
			v.InvalidCount++
		} else {
			v.ValidCount++
		}
		v.Rows = append(v.Rows, row)
	}

	h.renderWorkspaceShell(w, r, "Impor Desa", "/accounts", panel.AccountImportPreview(v))
}

// rawImportRowValues memetakan balik satu importRow ke map[header CSV]nilai
// APA ADANYA (belum diresolusi) — dipakai pratinjau agar kolom tabel persis
// mengikuti importCSVHeader, termasuk utk baris yang gagal validasi.
func rawImportRowValues(row importRow) map[string]string {
	values := make(map[string]string, len(importCSVHeader))
	values["kode_desa"] = row.villageCode
	values["email_pemilik"] = row.ownerEmail
	for header, field := range importColumnMap {
		values[header] = row.fields[field]
	}
	return values
}
