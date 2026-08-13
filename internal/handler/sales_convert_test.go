package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// sales_convert_test.go — cek duplikat desa SAAT REVIEW konversi lead (M4-6
// follow-up, soft-warning). Yang dijaga di sini:
//
//   - GET review menampilkan banner peringatan saat ada desa lain dgn nama sama
//     (case-insensitive) di tenant yang sama.
//   - Tanpa kandidat serupa, banner TAK muncul (tak menakut-nakuti tanpa sebab).
//   - POST konversi TETAP SUKSES walau kandidat duplikat ada — SOFT-WARNING,
//     bukan hard block (nama desa sama bisa valid beda dusun/kabupaten).
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler, sama pola dgn
// accounts_test.go/sales_activities_test.go.

// seedQualifiedLead menaruh satu lead Qualified (siap konversi) milik owner
// tertentu, langsung lewat pool (bypass handler form).
func (e *testEnv) seedQualifiedLead(t *testing.T, name string, owner int64, regency *string) db.Lead {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityLead)
	if err != nil {
		t.Fatalf("generate lead code: %v", err)
	}
	l, err := e.q.CreateLead(t.Context(), db.CreateLeadParams{
		TenantID:   e.tenantID,
		EntityCode: &code,
		LeadName:   name,
		LeadOwner:  &owner,
		LeadStatus: "Qualified",
		Regency:    regency,
		CreatedBy:  &owner,
	})
	if err != nil {
		t.Fatalf("seed lead %s: %v", name, err)
	}
	return l
}

// TestLeadConvertPage_DuplicateWarning_Shown: desa lain dgn nama sama (beda
// kapitalisasi/spasi) di tenant yang sama → banner peringatan tampil di GET
// review, tertaut ke account yang sudah ada.
func TestLeadConvertPage_DuplicateWarning_Shown(t *testing.T) {
	env, uid := setupAccounts(t)
	existing := env.seedAccount(t, "Sukamaju", &uid, nil, nil)
	lead := env.seedQualifiedLead(t, "  SUKAMAJU  ", uid, nil)

	req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(lead.ID)+"/convert", nil, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvertPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("LeadConvertPage status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "nama serupa") {
		t.Errorf("harus menampilkan banner peringatan duplikat, body:\n%s", body)
	}
	if !strings.Contains(body, "/accounts/"+strconv.FormatInt(existing.ID, 10)) {
		t.Errorf("banner harus tertaut ke account yang sudah ada (id=%d)", existing.ID)
	}
}

// TestLeadConvertPage_DuplicateWarning_HiddenWhenNoMatch: tanpa desa lain
// bernama sama → banner tak muncul sama sekali.
func TestLeadConvertPage_DuplicateWarning_HiddenWhenNoMatch(t *testing.T) {
	env, uid := setupAccounts(t)
	lead := env.seedQualifiedLead(t, "Desa Unik Sendirian", uid, nil)

	req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(lead.ID)+"/convert", nil, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvertPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("LeadConvertPage status %d\n%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "nama serupa") {
		t.Errorf("tanpa kandidat duplikat, banner tak boleh muncul")
	}
}

// TestLeadConvert_SucceedsDespiteDuplicate: SOFT-WARNING — POST konversi tetap
// menghasilkan account+contact+deal baru walau kandidat duplikat ada, TIDAK
// diblokir. Membuktikan cek duplikat murni informatif di sisi GET.
func TestLeadConvert_SucceedsDespiteDuplicate(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Sukamaju", &uid, nil, nil) // kandidat duplikat.
	lead := env.seedQualifiedLead(t, "Sukamaju", uid, nil)

	form := url.Values{}
	form.Set("village_name", "Sukamaju")
	form.Set("account_type", "prospect")
	form.Set("first_name", "Kontak Utama")
	form.Set("deal_name", "Deal Sukamaju")

	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(lead.ID)+"/convert", form, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvert)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("konversi dgn duplikat ada harus tetap sukses (303), got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=converted") {
		t.Errorf("harus redirect ok=converted, got %q", loc)
	}

	rows := env.allAccounts(t)
	if len(rows) != 2 { // seed awal + hasil konversi.
		t.Errorf("konversi harus tetap membuat desa baru walau duplikat ada, ada %d desa", len(rows))
	}
}
