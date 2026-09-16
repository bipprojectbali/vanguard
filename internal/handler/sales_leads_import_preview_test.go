package handler

import (
	"net/http"
	"strings"
	"testing"
)

// sales_leads_import_preview_test.go — BL-133 follow-up: peringatan
// (non-blocking) duplikat KOMBINASI lead_name+kecamatan di pratinjau impor
// CSV Lead (leadImportRowWarnings, sales_leads_import_warn.go). Mirror gaya
// sales_leads_import_confirm_test.go (butuh TEST_DATABASE_URL, auto-skip via
// setupAccounts→setupTest bila kosong). BELUM ada test lain utk
// LeadImportPreview happy-path sebelum ini — gate-nya sendiri sudah ditest di
// sales_leads_import_gate_test.go.

// TestLeadImportPreview_NameAndDistrictDuplicateWarning: satu lead sudah ada
// ("Lead Satu", district code1). Baris CSV baru dgn nama SAMA (beda kapital —
// membuktikan pencocokan case-insensitive) DAN kecamatan SAMA harus dapat
// SATU peringatan gabungan (bukan dua peringatan independen — revisi user:
// nama-sama-saja atau kecamatan-sama-saja saja TIDAK memicu apa pun), tapi
// tetap dihitung VALID (bukan gagal) — tombol konfirmasi tetap aktif.
func TestLeadImportPreview_NameAndDistrictDuplicateWarning(t *testing.T) {
	env, uid := setupAccounts(t)
	_, code1, _ := firstDistrictCode(t, env)

	seedCSV := "lead_name,kode_kecamatan\nLead Satu," + code1 + "\n"
	seedReq := accountsReq(http.MethodPost, "/w/test/leads/import/confirm", leadConfirmReq(seedCSV), "")
	seedRec := env.runAccount(uid, "owner", "sales", seedReq, env.h.LeadImportConfirm)
	if seedRec.Code != http.StatusSeeOther {
		t.Fatalf("seed lead gagal: %d\n%s", seedRec.Code, seedRec.Body.String())
	}

	csvText := "lead_name,kode_kecamatan\nlead satu," + code1 + "\n"
	req := newMultipartUploadRequest(t, "/w/test/leads/import", "csv_file", "import.csv", csvText)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadImportPreview)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, leadImportWarnDup) {
		t.Error("harus ada peringatan duplikat gabungan (nama DAN kecamatan sama-sama cocok)")
	}
	if !strings.Contains(body, "badge-warning") {
		t.Error("badge status baris harus badge-warning (soft-warning, bukan error)")
	}
	if strings.Contains(body, "DITOLAK SELURUHNYA") {
		t.Error("peringatan duplikat TIDAK boleh memicu all-or-nothing (baris tetap valid)")
	}
	if !strings.Contains(body, `type="submit" class="btn btn-primary w-fit"`) {
		t.Error("tombol konfirmasi TIDAK boleh dinonaktifkan hanya krn peringatan duplikat (atribut disabled ikut menempel)")
	}
}

// TestLeadImportPreview_NameOnlyMatch_NoWarning: lead lain sudah ada dgn nama
// SAMA tapi kecamatan BEDA → kombinasi tak cocok, TIDAK boleh memicu
// peringatan apa pun (nama-sama-saja saja tak cukup, revisi user 16 Sep).
func TestLeadImportPreview_NameOnlyMatch_NoWarning(t *testing.T) {
	env, uid := setupAccounts(t)
	code1, code2, ok := firstTwoDistrictCodes(t, env)
	if !ok {
		t.Skip("butuh minimal dua kecamatan di seed data test")
	}

	seedCSV := "lead_name,kode_kecamatan\nLead Satu," + code1 + "\n"
	seedReq := accountsReq(http.MethodPost, "/w/test/leads/import/confirm", leadConfirmReq(seedCSV), "")
	seedRec := env.runAccount(uid, "owner", "sales", seedReq, env.h.LeadImportConfirm)
	if seedRec.Code != http.StatusSeeOther {
		t.Fatalf("seed lead gagal: %d\n%s", seedRec.Code, seedRec.Body.String())
	}

	csvText := "lead_name,kode_kecamatan\nlead satu," + code2 + "\n"
	req := newMultipartUploadRequest(t, "/w/test/leads/import", "csv_file", "import.csv", csvText)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadImportPreview)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if strings.Contains(body, leadImportWarnDup) {
		t.Error("nama sama tapi kecamatan beda TIDAK boleh memicu peringatan duplikat")
	}
}

// firstTwoDistrictCodes = dua Kecamatan REAL (regions level 3) BERBEDA,
// dipakai TestLeadImportPreview_NameOnlyMatch_NoWarning utk membuktikan
// kecamatan BEDA menggagalkan kecocokan kombinasi walau nama sama. Menelusuri
// provinsi→kab/kota→kecamatan spt firstDistrictCode (accounts_test.go); ok
// false bila seed data test cuma punya satu kecamatan (test caller wajib
// skip, bukan gagal — data seed di luar kendali test ini).
func firstTwoDistrictCodes(t *testing.T, env *testEnv) (code1, code2 string, ok bool) {
	t.Helper()
	ctx := t.Context()
	provinces, err := env.q.ListProvinces(ctx)
	if err != nil {
		t.Fatalf("list provinces: %v", err)
	}
	var codes []string
	for _, p := range provinces {
		pid := p.ID
		regencies, err := env.q.ListRegenciesByProvince(ctx, &pid)
		if err != nil {
			t.Fatalf("list regencies: %v", err)
		}
		for _, reg := range regencies {
			rid := reg.ID
			districts, err := env.q.ListDistrictsByRegency(ctx, &rid)
			if err != nil {
				t.Fatalf("list districts: %v", err)
			}
			for _, d := range districts {
				codes = append(codes, d.Code)
				if len(codes) == 2 {
					return codes[0], codes[1], true
				}
			}
		}
	}
	return "", "", false
}

// TestLeadImportPreview_NoExistingData_NoWarning: tenant baru (belum ada lead
// sama sekali) → CSV baris valid biasa tak boleh memicu peringatan duplikat
// apa pun (memastikan leadImportRowWarnings tak false-positive saat kosong).
func TestLeadImportPreview_NoExistingData_NoWarning(t *testing.T) {
	env, uid := setupAccounts(t)
	_, code1, _ := firstDistrictCode(t, env)

	csvText := "lead_name,kode_kecamatan\nLead Baru," + code1 + "\n"
	req := newMultipartUploadRequest(t, "/w/test/leads/import", "csv_file", "import.csv", csvText)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadImportPreview)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if strings.Contains(body, leadImportWarnDup) {
		t.Error("tak boleh ada peringatan duplikat saat tenant belum punya lead sama sekali")
	}
}

// TestLeadImportPreview_InvalidRow_NoWarningComputed: baris dgn kode
// kecamatan tak dikenal (sudah gagal validasi) tak boleh ikut ditempeli
// peringatan duplikat — statusnya sudah jelas dari ErrMsg saja.
func TestLeadImportPreview_InvalidRow_NoWarningComputed(t *testing.T) {
	env, uid := setupAccounts(t)

	csvText := "lead_name,kode_kecamatan\nLead Gagal,99.99.99\n"
	req := newMultipartUploadRequest(t, "/w/test/leads/import", "csv_file", "import.csv", csvText)
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadImportPreview)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "badge-error") {
		t.Error("baris gagal harus tampil badge-error")
	}
	if strings.Contains(body, leadImportWarnDup) {
		t.Error("baris gagal validasi tak boleh ikut dapat peringatan duplikat")
	}
}
