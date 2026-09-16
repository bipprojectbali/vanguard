package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_leads_import_confirm.go — BL-133: LeadImportConfirm (POST
// /leads/import/confirm). raw_csv datang dari FORM BIASA (bukan multipart
// lagi — file sudah jadi teks di langkah pratinjau), dan di sini
// parseLeadImportCSV+resolveLeadImportRows dipanggil ULANG dari nol — TIDAK
// percaya hasil pratinjau sebelumnya, menutup celah TOCTOU (mis. kode
// kecamatan yang tadinya valid dihapus dari master data di antara kedua
// request) by construction. Mirror accounts_import_confirm.go (BL-63).
//
// Atomisitas all-or-nothing TANPA SAVEPOINT (pola sama AccountImportConfirm):
// (1) SEMUA validasi selesai SEBELUM insert pertama — satu baris gagal
// berarti loop insert tak pernah mulai; (2) insert berurutan berhenti+redirect
// di galat DB PERTAMA — Postgres membatalkan SELURUH tx `Scope` pada galat
// query apa pun, jadi baris yang sudah ter-INSERT di request ini ikut batal
// saat commit.
//
// *** KEPUTUSAN FLS BL-133 (SENGAJA, BUKAN BUG) — lihat juga
// sales_leads_import_resolve.go *** Loop di bawah menulis rr.form.MobilePhone
// APA ADANYA dari hasil parseLeadForm — TIDAK ADA pemanggilan canEditPhone(ctx)
// di mana pun di jalur impor ini. Importer yang lolos requireLeadWrite
// (crm:leads write) SELALU boleh menulis HP/WhatsApp dari CSV, walau
// business_role-nya dikunci dari kolom itu di form manual (LeadCreate/
// LeadEdit, lihat sales_leads.go/sales_leads_update.go). Keputusan final 14
// Sep (docs/crm/tasks.md BL-133) — JANGAN tambahkan canEditPhone di sini sbg
// "perbaikan FLS".
//
// LeadSource & Whatsapp SENGAJA tak diisi (nil) di CreateLeadParams bawah —
// CSV tak lagi punya kolom itu (lead_source dihapus, whatsapp digabung ke
// mobile_phone), mengikuti form manual Tambah Lead apa adanya.
func (h *Handler) LeadImportConfirm(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()

	rawCSV := r.FormValue("raw_csv")
	if strings.TrimSpace(rawCSV) == "" {
		wsRedirect(w, r, "/leads/import", "import_empty")
		return
	}

	rows, code := parseLeadImportCSV(strings.NewReader(rawCSV))
	if code != "" {
		wsRedirect(w, r, "/leads/import", code)
		return
	}

	tenantID := session.TenantID(ctx)
	uid := session.UserID(ctx)

	resolved, anyFailed := resolveLeadImportRows(ctx, h, rows)
	if anyFailed {
		// Re-validasi confirm menemukan baris tak valid yang lolos di pratinjau
		// (TOCTOU) — tolak SELURUH file, nol baris ditulis, minta ulang dari awal.
		wsRedirect(w, r, "/leads/import", "import_row_invalid")
		return
	}

	created := 0
	for _, rr := range resolved {
		// Leads tak dukung override entity_code manual (beda dari Account/
		// h.allocEntityCode) — selalu GenerateEntityCode langsung, sama pola
		// LeadCreate (sales_leads.go).
		entityCode, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityLead)
		if err != nil {
			h.Log.Error("leads import: generate code", "err", err)
			wsRedirect(w, r, "/leads/import", "failed")
			return
		}

		_, err = h.q(ctx).CreateLead(ctx, db.CreateLeadParams{
			TenantID:          tenantID,
			EntityCode:        &entityCode,
			LeadName:          rr.form.LeadName,
			LeadOwner:         &uid, // BL-133: owner SELALU importer, terlepas isi CSV
			ContactPerson:     rr.form.ContactPerson,
			JobTitle:          rr.form.JobTitle,
			LeadStatus:        rr.form.LeadStatus,
			Rating:            rr.form.Rating,
			UnqualifiedReason: rr.form.UnqualifiedReason,
			EstimatedValue:    rr.form.EstimatedValue,
			DistrictID:        rr.form.DistrictID,
			MobilePhone:       rr.form.MobilePhone, // FLS SENGAJA di-skip, lihat komentar file
			Email:             rr.form.Email,
			CreatedBy:         &uid,
		})
		if err != nil {
			if wcode, ok := leadWriteErr(err); ok {
				wsRedirect(w, r, "/leads/import", wcode)
				return
			}
			h.Log.Error("leads import: create", "err", err, "row", rr.rowNum)
			wsRedirect(w, r, "/leads/import", "failed")
			return
		}
		created++
	}

	// Audit RINGKAS (count) — bukan satu event lead.create per baris (yang
	// disebut draf tasks.md): konsisten dgn pola account.import (BL-63) dan
	// menghindari ratusan baris audit utk satu aksi impor.
	h.auditWorkspace(ctx, uid, "lead.import", tenantID, map[string]string{
		"count": strconv.Itoa(created),
	})
	wsRedirectOK(w, r, "/leads", "imported")
}
