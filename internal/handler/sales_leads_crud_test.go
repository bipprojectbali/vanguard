package handler

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sales_leads_crud_test.go — jalur create Lead terkait district_id (ADR 0009).
// Happy-path CRUD lead lain (edit/update F4) sudah dicakup sales_leads_fls_test.go
// & sales_convert_test.go; file ini fokus SEMPIT pada wiring district_id yang baru
// (parse form → CreateLeadParams → FK constraint), sama pola dgn
// accounts_crud_test.go (TestAccounts_CreateSuccess / TestAccounts_CreateRejectsUnknownDistrict).

// TestLeadCreate_WithDistrictID: district_id valid dari master regions tersimpan
// apa adanya di lead baru.
func TestLeadCreate_WithDistrictID(t *testing.T) {
	env, uid := setupAccounts(t)
	districtID := firstDistrictID(t, env)

	form := leadFormValues("Lead Wilayah", "New")
	form.Set("district_id", strconv.FormatInt(districtID, 10))
	req := accountsReq(http.MethodPost, "/w/test/leads", form, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}

	rows := env.allLeads(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 lead, ada %d", len(rows))
	}
	if rows[0].DistrictID == nil || *rows[0].DistrictID != districtID {
		t.Errorf("district_id harus tersimpan sesuai input, got %v want %d", rows[0].DistrictID, districtID)
	}
}

// TestLeadCreate_RejectsUnknownDistrict: district_id yang tak ada di master
// regions (FK violation, SQLSTATE 23503) ditolak dgn pesan spesifik
// ("district_id"), bukan 500 mentah — leadWriteErr mengenali constraint
// leads_district_id_fkey (ADR 0009).
func TestLeadCreate_RejectsUnknownDistrict(t *testing.T) {
	env, uid := setupAccounts(t)
	form := leadFormValues("Lead X", "New")
	form.Set("district_id", "999999999") // tak pernah ada di seed ~7.817 baris.
	req := accountsReq(http.MethodPost, "/w/test/leads", form, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=district_id") {
		t.Errorf("redirect harus err=district_id, got %q", loc)
	}
	if rows := env.allLeads(t); len(rows) != 0 {
		t.Errorf("FK violation harus membatalkan create, ada %d baris", len(rows))
	}
}

// allLeads mengembalikan semua lead tenant test (ScopeAll, halaman pertama —
// cukup utk kasus test yang cuma menyeed <=beberapa baris). Pola sama
// env.allAccounts di accounts_test.go.
func (e *testEnv) allLeads(t *testing.T) []db.Lead {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListLeads(t.Context(), db.ListLeadsParams{
		CursorCreatedAt: at, CursorID: id, ScopeAll: true, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("list leads: %v", err)
	}
	return rows
}
