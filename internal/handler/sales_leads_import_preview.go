package handler

import (
	"io"
	"net/http"
	"strings"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_leads_import_preview.go — BL-133: LeadImportPreview (POST
// /leads/import), dry-run: parse+resolve SEMUA baris TANPA menulis DB, lalu
// render pratinjau. CSV mentah dibawa ke konfirmasi lewat hidden field
// (raw_csv) — divalidasi ULANG penuh di LeadImportConfirm
// (sales_leads_import_confirm.go), bukan dipercaya dari sini (menutup celah
// TOCTOU by construction). Mirror accounts_import_preview.go (BL-63).

// LeadImportPreview — POST /leads/import. Native multipart upload (gotcha
// #16 — bukan Datastar). Galat file/parse → redirect ke form unggah dgn
// ?err=; validasi per-baris (kode_kecamatan tak ketemu, dst) TIDAK
// menggagalkan request ini — semua ditampilkan di tabel pratinjau agar
// operator tahu PERSIS baris mana yang harus diperbaiki di filenya.
func (h *Handler) LeadImportPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()

	// Batasi ukuran BODY total (bukan cuma file) — mencegah unggahan besar
	// menahan memori request (Rule 13). Melampaui batas → ParseMultipartForm
	// gagal dgn "http: request body too large".
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxLeadImportFileBytes))
	if err := r.ParseMultipartForm(int64(maxLeadImportFileBytes)); err != nil {
		wsRedirect(w, r, "/leads/import", "import_invalid")
		return
	}

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		wsRedirect(w, r, "/leads/import", "import_invalid")
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		wsRedirect(w, r, "/leads/import", "import_invalid")
		return
	}
	rawCSV := string(raw)

	rows, code := parseLeadImportCSV(strings.NewReader(rawCSV))
	if code != "" {
		wsRedirect(w, r, "/leads/import", code)
		return
	}

	// resolveLeadImportRows sendiri TAK butuh tenantID — HANYA menyentuh
	// `regions` (global, tanpa RLS/tenant_id), beda dari resolveImportRows
	// milik Account yang juga cek duplikat per-tenant. tenantID di sini
	// dipakai belakangan oleh leadImportRowWarnings (peringatan duplikat).
	tenantID := session.TenantID(ctx)
	resolved, anyFailed := resolveLeadImportRows(ctx, h, rows)
	warnings := leadImportRowWarnings(ctx, h, tenantID, rows, resolved)

	// Kolom "Kecamatan" BUKAN kolom CSV (leadImportCSVHeader tak berubah,
	// template unduhan tetap sama) — nama hasil RESOLUSI kode_kecamatan ke
	// master data (rr.districtName), disisipkan tepat setelah
	// "kode_kecamatan" agar operator langsung lihat kecamatan mana yang
	// dimaksud kode itu tanpa buka tab lain.
	columns := make([]panel.LeadImportColumn, 0, len(leadImportCSVHeader)+1)
	for _, key := range leadImportCSVHeader {
		columns = append(columns, panel.LeadImportColumn{Key: key, Label: leadImportColumnLabels[key]})
		if key == "kode_kecamatan" {
			columns = append(columns, leadImportDistrictNameCol)
		}
	}

	base := wsPath(slugFromRequest(r), "")
	v := panel.LeadImportPreviewView{
		Base: base,
		Upload: panel.LeadImportFormView{
			Base:         base,
			Action:       base + "/leads/import",
			TemplateURL:  base + "/leads/import/template",
			MaxRows:      maxLeadImportRows,
			MaxFileBytes: maxLeadImportFileBytes,
		},
		ConfirmURL: base + "/leads/import/confirm",
		RawCSV:     rawCSV,
		AnyFailed:  anyFailed,
		Columns:    columns,
		Rows:       make([]panel.LeadImportPreviewRow, 0, len(resolved)),
	}
	// rows[i] & resolved[i] selalu berpasangan (resolveLeadImportRows menjaga
	// urutan & jumlah sama dgn input) — nilai MENTAH dari rows dipakai agar
	// tabel tetap menampilkan data baris walau gagal ter-resolve (rr.form
	// kosong pada baris error). ValidCount/InvalidCount dihitung di sini
	// (bukan di view) — view murni-data, tak boleh menghitung ulang dari Rows.
	for i, rr := range resolved {
		row := panel.LeadImportPreviewRow{Values: rawImportLeadRowValues(rows[i])}
		// districtName kosong pada baris gagal (belum sempat ter-resolve) —
		// tampilkan "-" alih-alih sel kosong yang gampang terlihat seperti
		// data hilang/rusak.
		name := rr.districtName
		if name == "" {
			name = "-"
		}
		row.Values[leadImportDistrictNameCol.Key] = name
		if rr.errCode != "" {
			row.ErrMsg = wsErrMsg(rr.errCode)
			v.InvalidCount++
		} else {
			v.ValidCount++
			if warnings[i] != "" {
				row.WarnMsg = warnings[i]
				v.WarnCount++
			}
		}
		v.Rows = append(v.Rows, row)
	}

	h.renderWorkspaceShell(w, r, "Impor Lead", "/leads", panel.LeadImportPreview(v))
}

// rawImportLeadRowValues memetakan balik satu leadImportRow ke map[header
// CSV]nilai APA ADANYA (belum diresolusi) — dipakai pratinjau agar kolom
// tabel persis mengikuti leadImportCSVHeader, termasuk utk baris yang gagal
// validasi.
func rawImportLeadRowValues(row leadImportRow) map[string]string {
	values := make(map[string]string, len(leadImportCSVHeader))
	values["kode_kecamatan"] = row.districtCode
	for header, field := range leadImportColumnMap {
		values[header] = row.fields[field]
	}
	return values
}
