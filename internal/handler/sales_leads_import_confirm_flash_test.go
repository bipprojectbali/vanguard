package handler

import (
	"net/http"
	"strings"
	"testing"
)

// sales_leads_import_confirm_flash_test.go — BL-133 follow-up: toast sukses
// SETELAH impor CSV Lead. leadsMsg sebelumnya tak punya kasus "imported"
// walau LeadImportConfirm sudah redirect PRG dgn ?ok=imported & LeadsList
// sudah merender ui.Toast dari v.Msg — pesan jatuh ke default "" & toast tak
// pernah tampil. Mirror TestQuoteDetail_RendersOKFlash
// (sales_quotes_detail_flash_test.go): GET halaman daftar dgn ?ok=imported,
// assert body memuat leadsMsg("imported").

// TestLeadsList_RendersImportOKFlash: GET /leads?ok=imported memuat toast
// sukses (bukan halaman polos yang menelan kode ok= senyap).
func TestLeadsList_RendersImportOKFlash(t *testing.T) {
	env, uid := setupAccounts(t)

	req := accountsReq(http.MethodGet, "/w/test/leads?ok=imported", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if want := leadsMsg("imported"); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("daftar lead harus memuat toast sukses %q, body tak mengandungnya", want)
	}
}
