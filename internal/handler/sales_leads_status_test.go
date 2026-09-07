package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_leads_status_test.go — BL-83: transisi STATUS lead sebagai aksi tersendiri
// (LeadStatus, POST /leads/{id}/status), dipisah dari sunting profil (LeadUpdate).
// Menegakkan: status tersimpan + audit lead.status.changed; alasan Unqualified
// hanya bertahan saat status Unqualified; lead terkonversi & status non-manual
// ditolak; dan LeadUpdate TAK lagi menyentuh status/alasan.

// leadStatusForm merakit form values untuk LeadStatus (status + alasan opsional).
func leadStatusForm(status, reason string) url.Values {
	v := url.Values{"lead_status": {status}}
	if reason != "" {
		v.Set("unqualified_reason", reason)
	}
	return v
}

// TestLeadStatus_UpdatesAndAudits: transisi bebas antar-status manual tersimpan
// & tercatat audit lead.status.changed (dari→ke).
func TestLeadStatus_UpdatesAndAudits(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead A", uid, "New")

	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID)+"/status",
		leadStatusForm("Contacted", ""), itoa(l.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadStatus)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("redirect harus ok=saved, got %q", loc)
	}
	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.LeadStatus != "Contacted" {
		t.Errorf("status harus Contacted, got %q", got.LeadStatus)
	}
	env.assertAudited(t, "lead.status.changed")
}

// TestLeadStatus_DropsReasonWhenNotUnqualified: alasan Unqualified dikirim tapi
// status ≠ Unqualified → alasan dibuang (tak ada "alasan yatim").
func TestLeadStatus_DropsReasonWhenNotUnqualified(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead B", uid, "New")

	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID)+"/status",
		leadStatusForm("Qualified", "alasan basi"), itoa(l.ID))
	if rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.LeadStatus != "Qualified" {
		t.Errorf("status harus Qualified, got %q", got.LeadStatus)
	}
	if got.UnqualifiedReason != nil {
		t.Errorf("alasan harus dibuang saat status ≠ Unqualified, got %v", got.UnqualifiedReason)
	}
}

// TestLeadStatus_KeepsReasonWhenUnqualified: status Unqualified + alasan → alasan
// tersimpan (terkopel status).
func TestLeadStatus_KeepsReasonWhenUnqualified(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead C", uid, "New")

	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID)+"/status",
		leadStatusForm("Unqualified", "Tak ada anggaran"), itoa(l.ID))
	if rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.LeadStatus != "Unqualified" {
		t.Errorf("status harus Unqualified, got %q", got.LeadStatus)
	}
	if got.UnqualifiedReason == nil || *got.UnqualifiedReason != "Tak ada anggaran" {
		t.Errorf("alasan harus tersimpan saat Unqualified, got %v", got.UnqualifiedReason)
	}
}

// TestLeadStatus_RejectsConverted: lead terkonversi (status terminal Converted)
// tak bisa diubah statusnya — ditolak err=lead_status, status tetap Converted.
func TestLeadStatus_RejectsConverted(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead D", uid, "Qualified")
	// Tandai terkonversi TANPA deal (converted_deal_id NULL) — FK deals(id) tak
	// mengizinkan id palsu; kita hanya butuh flag converted + status terminal.
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE leads SET converted = true, lead_status = 'Converted' WHERE id = $1`, l.ID); err != nil {
		t.Fatalf("mark converted: %v", err)
	}

	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID)+"/status",
		leadStatusForm("New", ""), itoa(l.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=lead_status") {
		t.Errorf("lead terkonversi harus ditolak err=lead_status, got %q", loc)
	}
	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.LeadStatus != "Converted" {
		t.Errorf("status lead terkonversi harus tetap Converted, got %q", got.LeadStatus)
	}
}

// TestLeadStatus_RejectsInvalidStatus: status non-manual ('Converted' sistem-only,
// atau nilai liar) ditolak err=lead_status, status lama dipertahankan.
func TestLeadStatus_RejectsInvalidStatus(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead E", uid, "New")

	for _, bad := range []string{"Converted", "Bogus"} {
		req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID)+"/status",
			leadStatusForm(bad, ""), itoa(l.ID))
		rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadStatus)
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=lead_status") {
			t.Errorf("status %q harus ditolak err=lead_status, got %q", bad, loc)
		}
	}
	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.LeadStatus != "New" {
		t.Errorf("status harus tetap New setelah tolak, got %q", got.LeadStatus)
	}
}

// TestLeadUpdate_DoesNotChangeStatus: BL-83 — LeadUpdate (sunting profil) TAK
// menyentuh status/alasan meski form membawanya. Status tersimpan tetap nilai lama.
func TestLeadUpdate_DoesNotChangeStatus(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadStatus(t, "Lead F", uid, "Qualified")

	// leadFormValues menyisipkan lead_status=New; LeadUpdate harus mengabaikannya.
	form := leadFormValues("Lead F", "New")
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID), form, itoa(l.ID))
	if rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.LeadStatus != "Qualified" {
		t.Errorf("BL-83: LeadUpdate TAK boleh mengubah status, got %q (want Qualified)", got.LeadStatus)
	}
}
