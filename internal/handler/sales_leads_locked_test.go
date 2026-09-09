package handler

import (
	"net/http"
	"strings"
	"testing"
)

// sales_leads_locked_test.go — lead TERKONVERSI = terminal: sunting (GET+POST) &
// hapus ditolak backend dgn err=lead_locked, tanpa mutasi. UI menyembunyikan
// tombolnya (sales_leads_detail_test), tapi backend tetap penjaga sesungguhnya
// (endpoint tak boleh dijangkau via POST langsung). Setup converted meniru
// TestLeadStatus_RejectsConverted: flag converted tanpa deal (FK deals(id) tak
// mengizinkan id palsu — cukup flag + status terminal).

// markConverted menandai lead terkonversi (tanpa deal nyata) untuk uji guard.
func markConverted(t *testing.T, env *testEnv, id int64) {
	t.Helper()
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE leads SET converted = true, lead_status = 'Converted' WHERE id = $1`, id); err != nil {
		t.Fatalf("mark converted: %v", err)
	}
}

// TestLeadEdit_RejectsConverted: GET form sunting lead terkonversi → redirect
// err=lead_locked (bukan render form).
func TestLeadEdit_RejectsConverted(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead Terkunci", uid, "Qualified")
	markConverted(t, env, l.ID)

	req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(l.ID)+"/edit", nil, itoa(l.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadEdit)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=lead_locked") {
		t.Errorf("sunting lead terkonversi harus err=lead_locked, got %q", loc)
	}
}

// TestLeadUpdate_RejectsConverted: POST simpan sunting lead terkonversi →
// err=lead_locked & nama TAK berubah (jaring balapan dikonversi setelah form dibuka).
func TestLeadUpdate_RejectsConverted(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Nama Asli", uid, "Qualified")
	markConverted(t, env, l.ID)

	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID),
		leadFormValues("Nama Baru", "New"), itoa(l.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=lead_locked") {
		t.Errorf("update lead terkonversi harus err=lead_locked, got %q", loc)
	}
	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.LeadName != "Nama Asli" {
		t.Errorf("nama lead terkonversi harus tetap %q, got %q", "Nama Asli", got.LeadName)
	}
}

// TestLeadDelete_RejectsConverted: POST hapus lead terkonversi → err=lead_locked
// & lead TAK ter-soft-delete (deleted_at tetap NULL).
func TestLeadDelete_RejectsConverted(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead Hidup", uid, "Qualified")
	markConverted(t, env, l.ID)

	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID)+"/delete", nil, itoa(l.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadDelete)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=lead_locked") {
		t.Errorf("hapus lead terkonversi harus err=lead_locked, got %q", loc)
	}
	if _, err := env.q.GetLead(t.Context(), l.ID); err != nil {
		t.Errorf("lead terkonversi TAK boleh terhapus (GetLead gagal): %v", err)
	}
}
