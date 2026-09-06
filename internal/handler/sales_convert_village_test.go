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

// sales_convert_village_test.go — BL-67: konversi Lead → Akun kini MENGAMBIL desa
// dari master Kemendagri (regions level 4), sejajar AccountCreate langsung, agar
// akun hasil convert ber-village_code asli & tunduk "satu desa hidup = satu akun".
// Yang dijaga di sini:
//
//   (1) village_id sah → akun baru mewarisi village_code/village_name/district_id
//       dari baris master, redirect ok=converted.
//   (2) village_id kosong → village_required, TAK ada akun (WAJIB saat convert).
//   (3) village_id bukan level 4 (mis. id provinsi) → village_id, TAK ada akun.
//   (4) desa itu sudah punya akun HIDUP → village_code_dup + dup=<id>, TAK ada
//       akun baru (blok keras berbasis village_code, bukan sekadar nama).
//   (5) GET halaman review dgn ?err=village_code_dup&dup=<id> merender pesan +
//       TAUTAN ke akun eksisting (/accounts/<id>).
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler, pola sama
// sales_convert_test.go.

// seedAccountAtVillage menaruh satu akun HIDUP yang MENDUDUKI village_code sebuah
// Desa master (mengisi idx_accounts_code) — untuk menguji blok duplikat BL-67.
func (e *testEnv) seedAccountAtVillage(t *testing.T, owner int64, v villageRow) db.Account {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityAccount)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	vcode, did := v.Code, v.DistrictID
	a, err := e.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:     e.tenantID,
		EntityCode:   &code,
		VillageName:  v.Name,
		VillageCode:  &vcode,
		AccountType:  "prospect",
		AccountOwner: &owner,
		DistrictID:   &did,
		CreatedBy:    &owner,
	})
	if err != nil {
		t.Fatalf("seed account at village %s: %v", v.Name, err)
	}
	return a
}

// convertFormBase merakit form konversi minimal valid (tanpa village_id —
// pemanggil menambah lewat withVillage bila perlu).
func convertFormBase() url.Values {
	f := url.Values{}
	f.Set("account_type", "prospect")
	f.Set("first_name", "Kontak Utama")
	f.Set("deal_name", "Deal Konversi")
	return f
}

// TestLeadConvert_DerivesVillageFromMaster: village_id sah → akun baru mewarisi
// village_code/village_name/district_id dari master, redirect ok=converted.
func TestLeadConvert_DerivesVillageFromMaster(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	lead := env.seedQualifiedLead(t, "Lead Desa", uid, nil)

	form := withVillage(convertFormBase(), v)
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(lead.ID)+"/convert", form, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvert)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("konversi valid harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=converted") {
		t.Errorf("harus redirect ok=converted, got %q", loc)
	}

	rows := env.allAccounts(t)
	if len(rows) != 1 {
		t.Fatalf("harus tepat 1 akun hasil convert, ada %d", len(rows))
	}
	got := rows[0]
	if got.VillageCode == nil || *got.VillageCode != v.Code {
		t.Errorf("village_code = %v, want %q (dari master)", got.VillageCode, v.Code)
	}
	if got.VillageName != v.Name {
		t.Errorf("village_name = %q, want %q (dari master)", got.VillageName, v.Name)
	}
	if got.DistrictID == nil || *got.DistrictID != v.DistrictID {
		t.Errorf("district_id = %v, want %d (Kecamatan induk master)", got.DistrictID, v.DistrictID)
	}
}

// TestLeadConvert_MissingVillage_Rejected: tanpa village_id → village_required,
// TAK ada akun dibuat (WAJIB saat convert — lead pasti Qualified, desa diketahui).
func TestLeadConvert_MissingVillage_Rejected(t *testing.T) {
	env, uid := setupAccounts(t)
	lead := env.seedQualifiedLead(t, "Lead Tanpa Desa", uid, nil)

	form := convertFormBase() // tanpa withVillage.
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(lead.ID)+"/convert", form, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvert)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus redirect (303), got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=village_required") {
		t.Errorf("harus err=village_required, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 0 {
		t.Errorf("konversi ditolak harus TAK membuat akun, ada %d", len(rows))
	}
}

// TestLeadConvert_NonLevel4Village_Rejected: village_id yang menunjuk baris BUKAN
// level 4 (id provinsi) → village_id, TAK ada akun (GetVillageRegion saring level 4).
func TestLeadConvert_NonLevel4Village_Rejected(t *testing.T) {
	env, uid := setupAccounts(t)
	provinces, err := env.q.ListProvinces(t.Context())
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	lead := env.seedQualifiedLead(t, "Lead Provinsi", uid, nil)

	form := convertFormBase()
	form.Set("village_id", strconv.FormatInt(provinces[0].ID, 10)) // level 1, bukan Desa.
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(lead.ID)+"/convert", form, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvert)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus redirect (303), got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=village_id") {
		t.Errorf("harus err=village_id, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 0 {
		t.Errorf("konversi ditolak harus TAK membuat akun, ada %d", len(rows))
	}
}

// TestLeadConvert_DuplicateVillageCode_Blocked: desa target sudah punya akun HIDUP
// → village_code_dup + dup=<id akun eksisting>, TAK ada akun baru (tetap 1). Blok
// KERAS berbasis village_code (beda dari soft-warning nama).
func TestLeadConvert_DuplicateVillageCode_Blocked(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	existing := env.seedAccountAtVillage(t, uid, v) // menduduki village_code v.Code.
	lead := env.seedQualifiedLead(t, "Lead Desa Sama", uid, nil)

	form := withVillage(convertFormBase(), v) // desa yang SAMA.
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(lead.ID)+"/convert", form, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvert)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus redirect (303), got %d\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=village_code_dup") {
		t.Errorf("harus err=village_code_dup, got %q", loc)
	}
	if !strings.Contains(loc, "dup="+strconv.FormatInt(existing.ID, 10)) {
		t.Errorf("harus menautkan dup=%d (akun eksisting), got %q", existing.ID, loc)
	}
	if rows := env.allAccounts(t); len(rows) != 1 { // hanya akun eksisting.
		t.Errorf("konversi diblokir harus TAK membuat akun baru, ada %d", len(rows))
	}
}

// TestLeadConvertPage_DuplicateVillageCode_RendersLink: GET review dgn
// ?err=village_code_dup&dup=<id> merender pesan galat + TAUTAN ke akun eksisting.
func TestLeadConvertPage_DuplicateVillageCode_RendersLink(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	existing := env.seedAccountAtVillage(t, uid, v)
	lead := env.seedQualifiedLead(t, "Lead Desa Sama", uid, nil)

	target := "/w/test/leads/" + itoa(lead.ID) + "/convert?err=village_code_dup&dup=" +
		strconv.FormatInt(existing.ID, 10)
	req := accountsReq(http.MethodGet, target, nil, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvertPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("LeadConvertPage status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "sudah dipakai desa lain") {
		t.Errorf("harus menampilkan pesan village_code_dup, body:\n%s", body)
	}
	if !strings.Contains(body, "/accounts/"+strconv.FormatInt(existing.ID, 10)) {
		t.Errorf("harus menautkan akun eksisting id=%d, body:\n%s", existing.ID, body)
	}
	if !strings.Contains(body, v.Name) {
		t.Errorf("label tautan harus memuat nama desa %q, body:\n%s", v.Name, body)
	}
}
