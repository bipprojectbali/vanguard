package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/session"
)

// contacts_import_confirm.go — BL-134: ContactImportConfirm (POST
// /contacts/import/confirm). raw_csv datang dari FORM BIASA (bukan multipart
// lagi — file sudah jadi teks di langkah pratinjau), dan di sini
// parseContactImportCSV+resolveContactImportRows dipanggil ULANG dari nol —
// TIDAK percaya hasil pratinjau sebelumnya, menutup celah TOCTOU (mis. desa
// direbut kontak primary lain di antara kedua request) by construction. Pola
// identik accounts_import_confirm.go (BL-63).
//
// Atomisitas all-or-nothing TANPA SAVEPOINT (pola sama LeadConvert &
// AccountImportConfirm): (1) SEMUA validasi selesai SEBELUM insert pertama;
// (2) insert berurutan berhenti+redirect di galat DB PERTAMA — Postgres
// membatalkan SELURUH tx `Scope` pada galat query apa pun.
func (h *Handler) ContactImportConfirm(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()

	rawCSV := r.FormValue("raw_csv")
	if strings.TrimSpace(rawCSV) == "" {
		wsRedirect(w, r, "/contacts/import", "import_empty")
		return
	}

	rows, code := parseContactImportCSV(strings.NewReader(rawCSV))
	if code != "" {
		wsRedirect(w, r, "/contacts/import", code)
		return
	}

	resolved, anyFailed := resolveContactImportRows(ctx, h, rows)
	if anyFailed {
		// Re-validasi confirm menemukan baris tak valid yang lolos di
		// pratinjau (TOCTOU) — tolak SELURUH file, nol baris ditulis.
		wsRedirect(w, r, "/contacts/import", "import_row_invalid")
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	// insertContact (contacts_create.go) DIPAKAI ULANG UTUH tanpa modifikasi
	// — sudah menangani set-primary dua-langkah, alokasi entity_code, INSERT
	// & audit contact.create per baris. Resolusi tahap sebelumnya sudah
	// menolak SELURUH file utk konflik primary manapun, jadi loop ini tak
	// pernah melanggar idx_contacts_primary saat benar-benar jalan. FLS
	// canEditPhone SENGAJA TIDAK dipanggil di sini (keputusan 14 Sep BL-134)
	// — sama seperti jalur create manual (insertContact tak pernah
	// memanggilnya), importer dgn crm:contacts write selalu boleh isi HP/WA.
	created := 0
	for _, rr := range resolved {
		if _, err := h.insertContact(ctx, rr.accountID, rr.form); err != nil {
			h.Log.Error("contacts import: create", "err", err, "row", rr.rowNum)
			wsRedirect(w, r, "/contacts/import", "failed")
			return
		}
		created++
	}

	h.auditWorkspace(ctx, uid, "contact.import", tenantID, map[string]string{
		"count": strconv.Itoa(created),
	})
	wsRedirectOK(w, r, "/contacts", "imported")
}
