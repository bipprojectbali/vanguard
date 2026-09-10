package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// health_score_bl114_test.go — BL-114: populasi Health Score = desa PELANGGAN
// (punya langganan), bukan semua desa; dipecah dua segmen (active/churned) dan
// penyuntingan Customer Success dikunci untuk non-pelanggan (A8).
//
//   - Segmen populasi: default "active" = punya langganan HIDUP (Trial/Active/
//     Suspended); "churned" = punya langganan tapi tak ada yang hidup; PROSPEK
//     (tanpa langganan) dikecualikan dari KEDUA segmen. Churn hidup di
//     subscriptions.status, BUKAN lifecycle_stage.
//   - A8: CustomerSuccess hanya bisa DITULIS untuk pelanggan aktif — detail
//     mengunci tombol + catatan; edit GET & save POST memantul ke detail.
//
// makeCustomer(t, accountID, status) di health_score_test.go menyeed langganan.

// --- segmen populasi (daftar) ---------------------------------------------

// TestHealthScore_SegmentActiveExcludesProspectAndChurned: default (tanpa
// ?segment) = segmen "active". Hanya desa dengan langganan HIDUP tampil;
// prospek (tanpa langganan) & churned (langganan mati semua) tidak.
func TestHealthScore_SegmentActiveExcludesProspectAndChurned(t *testing.T) {
	env, uid := setupAccounts(t)

	prospek := env.seedAccount(t, "Desa Prospek", &uid, nil, nil)
	env.seedHealthScore(t, prospek.ID, 80, "Healthy") // punya skor tapi TANPA langganan

	aktif := env.seedAccount(t, "Desa Pelanggan Aktif", &uid, nil, nil)
	env.seedHealthScore(t, aktif.ID, 70, "At-Risk")
	env.makeCustomer(t, aktif.ID, "Active")

	churned := env.seedAccount(t, "Desa Mantan Pelanggan", &uid, nil, nil)
	env.seedHealthScore(t, churned.ID, 30, "Critical")
	env.makeCustomer(t, churned.ID, "Churned")

	req := accountsReq(http.MethodGet, "/w/test/health-scores", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Pelanggan Aktif") {
		t.Error("segmen active harus memuat desa berlangganan hidup")
	}
	if strings.Contains(body, "Desa Prospek") {
		t.Error("segmen active TAK boleh memuat prospek (tanpa langganan)")
	}
	if strings.Contains(body, "Desa Mantan Pelanggan") {
		t.Error("segmen active TAK boleh memuat churned (langganan mati semua)")
	}
}

// TestHealthScore_SegmentChurned: ?segment=churned = eks-pelanggan (punya
// langganan, tak ada yang hidup). Desa aktif & prospek tak muncul.
func TestHealthScore_SegmentChurned(t *testing.T) {
	env, uid := setupAccounts(t)

	prospek := env.seedAccount(t, "Desa Prospek", &uid, nil, nil)
	env.seedHealthScore(t, prospek.ID, 80, "Healthy")

	aktif := env.seedAccount(t, "Desa Pelanggan Aktif", &uid, nil, nil)
	env.seedHealthScore(t, aktif.ID, 70, "At-Risk")
	env.makeCustomer(t, aktif.ID, "Active")

	churned := env.seedAccount(t, "Desa Mantan Pelanggan", &uid, nil, nil)
	env.seedHealthScore(t, churned.ID, 30, "Critical")
	env.makeCustomer(t, churned.ID, "Churned")

	req := accountsReq(http.MethodGet, "/w/test/health-scores?segment=churned", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Mantan Pelanggan") {
		t.Error("segmen churned harus memuat eks-pelanggan")
	}
	if strings.Contains(body, "Desa Pelanggan Aktif") {
		t.Error("segmen churned TAK boleh memuat pelanggan aktif")
	}
	if strings.Contains(body, "Desa Prospek") {
		t.Error("segmen churned TAK boleh memuat prospek (tanpa langganan)")
	}
}

// TestHealthScore_SegmentInvalidFallsBackToActive: nilai ?segment= janggal jatuh
// ke "active" (default aman) — tak pernah membuka churned tanpa diminta eksplisit.
func TestHealthScore_SegmentInvalidFallsBackToActive(t *testing.T) {
	env, uid := setupAccounts(t)

	aktif := env.seedAccount(t, "Desa Aktif Fallback", &uid, nil, nil)
	env.seedHealthScore(t, aktif.ID, 70, "At-Risk")
	env.makeCustomer(t, aktif.ID, "Active")

	churned := env.seedAccount(t, "Desa Churned Fallback", &uid, nil, nil)
	env.seedHealthScore(t, churned.ID, 30, "Critical")
	env.makeCustomer(t, churned.ID, "Cancelled")

	req := accountsReq(http.MethodGet, "/w/test/health-scores?segment=bogus", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Aktif Fallback") {
		t.Error("segmen janggal harus jatuh ke active (memuat pelanggan aktif)")
	}
	if strings.Contains(body, "Desa Churned Fallback") {
		t.Error("segmen janggal TAK boleh membuka churned")
	}
}

// --- segmen populasi (KPI) -------------------------------------------------

// TestHealthScore_KPISegmented: CountHealthScoreKPIs dihitung atas populasi yang
// SAMA dengan daftar per segmen. Prospek dikecualikan dari kedua segmen; active
// hanya hitung pelanggan hidup; churned hanya eks-pelanggan.
func TestHealthScore_KPISegmented(t *testing.T) {
	env, uid := setupAccounts(t)

	// Prospek: punya skor Healthy, TANPA langganan → tak dihitung di mana pun.
	prospek := env.seedAccount(t, "Desa KPI Prospek", &uid, nil, nil)
	env.seedHealthScore(t, prospek.ID, 85, "Healthy")

	// Dua pelanggan aktif (Healthy + At-Risk).
	a1 := env.seedAccount(t, "Desa KPI Aktif 1", &uid, nil, nil)
	env.seedHealthScore(t, a1.ID, 85, "Healthy")
	env.makeCustomer(t, a1.ID, "Active")
	a2 := env.seedAccount(t, "Desa KPI Aktif 2", &uid, nil, nil)
	env.seedHealthScore(t, a2.ID, 55, "At-Risk")
	env.makeCustomer(t, a2.ID, "Trial")

	// Satu churned (Critical).
	c1 := env.seedAccount(t, "Desa KPI Churned", &uid, nil, nil)
	env.seedHealthScore(t, c1.ID, 20, "Critical")
	env.makeCustomer(t, c1.ID, "Churned")

	active, err := env.q.CountHealthScoreKPIs(t.Context(), db.CountHealthScoreKPIsParams{
		ScopeAll: true, Segment: "active",
	})
	if err != nil {
		t.Fatalf("CountHealthScoreKPIs active: %v", err)
	}
	if active.Total != 2 {
		t.Errorf("segmen active Total harus 2 (dua pelanggan hidup, prospek+churned dikecualikan), got %d", active.Total)
	}
	if active.Healthy != 1 || active.AtRisk != 1 {
		t.Errorf("segmen active: harap 1 Healthy + 1 AtRisk, got Healthy=%d AtRisk=%d", active.Healthy, active.AtRisk)
	}
	if active.Critical != 0 {
		t.Errorf("segmen active: Critical (churned) harus 0, got %d", active.Critical)
	}

	churned, err := env.q.CountHealthScoreKPIs(t.Context(), db.CountHealthScoreKPIsParams{
		ScopeAll: true, Segment: "churned",
	})
	if err != nil {
		t.Fatalf("CountHealthScoreKPIs churned: %v", err)
	}
	if churned.Total != 1 {
		t.Errorf("segmen churned Total harus 1 (satu eks-pelanggan), got %d", churned.Total)
	}
	if churned.Critical != 1 {
		t.Errorf("segmen churned: harap 1 Critical, got %d", churned.Critical)
	}
}

// --- A8: kunci tulis Customer Success ke langganan aktif -------------------

const csWriteLockMarker = "belum menjadi pelanggan aktif"

// TestCustomerSuccess_A8_DetailLockNonCustomer: desa non-pelanggan → halaman
// detail menampilkan catatan kunci & TAK merender aksi tulis; desa pelanggan
// aktif → tombol sunting tampil, tanpa catatan kunci.
func TestCustomerSuccess_A8_DetailLockNonCustomer(t *testing.T) {
	env, uid := setupAccounts(t)

	// Non-pelanggan: admin (F2 tulis penuh) tetap terkunci.
	prospek := env.seedAccount(t, "Desa A8 Prospek", &uid, nil, nil)
	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(prospek.ID)+"/customer-success", nil, itoa(prospek.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail non-pelanggan harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), csWriteLockMarker) {
		t.Error("non-pelanggan harus menampilkan catatan kunci A8")
	}

	// Pelanggan aktif: tak ada catatan kunci.
	pelanggan := env.seedAccount(t, "Desa A8 Pelanggan", &uid, nil, nil)
	env.makeCustomer(t, pelanggan.ID, "Active")
	req2 := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(pelanggan.ID)+"/customer-success", nil, itoa(pelanggan.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.CustomerSuccessDetail)
	if rec2.Code != http.StatusOK {
		t.Fatalf("detail pelanggan harus 200, got %d\n%s", rec2.Code, rec2.Body.String())
	}
	if strings.Contains(rec2.Body.String(), csWriteLockMarker) {
		t.Error("pelanggan aktif TAK boleh menampilkan catatan kunci A8")
	}
}

// TestCustomerSuccess_A8_EditGETRedirectNonCustomer: GET form sunting pada desa
// non-pelanggan → 303 kembali ke detail (tanpa ?err), bukan 200 form.
func TestCustomerSuccess_A8_EditGETRedirectNonCustomer(t *testing.T) {
	env, uid := setupAccounts(t)
	prospek := env.seedAccount(t, "Desa A8 Edit", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(prospek.ID)+"/customer-success/edit", nil, itoa(prospek.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessEdit)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("edit non-pelanggan harus 303 ke detail, got %d\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasSuffix(loc, "/accounts/"+itoa(prospek.ID)+"/customer-success") {
		t.Errorf("harus memantul ke detail customer-success, got %q", loc)
	}
	if strings.Contains(loc, "err=") {
		t.Errorf("pantulan A8 bukan galat (tanpa ?err), got %q", loc)
	}
}

// TestCustomerSuccess_A8_SavePOSTRejectedNonCustomer: POST simpan pada desa
// non-pelanggan → 303 ke detail & TAK PERNAH menyimpan baris CS, walau admin
// (F2 tulis penuh) & form sah. Gate berjalan SEBELUM parse form.
func TestCustomerSuccess_A8_SavePOSTRejectedNonCustomer(t *testing.T) {
	env, uid := setupAccounts(t)
	prospek := env.seedAccount(t, "Desa A8 Save", &uid, nil, nil)

	form := customerSuccessFormValues()
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(prospek.ID)+"/customer-success", form, itoa(prospek.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save non-pelanggan harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasSuffix(loc, "/accounts/"+itoa(prospek.ID)+"/customer-success") {
		t.Errorf("save A8 harus memantul ke detail (bukan form edit), got %q", loc)
	}
	if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), prospek.ID); err == nil {
		t.Error("desa non-pelanggan tak boleh menyimpan baris CS apa pun")
	}
}

// TestCustomerSuccess_A8_SaveAllowedForCustomer: desa pelanggan aktif → save
// berjalan normal (303 ok=created, baris tersimpan) — membuktikan gate hanya
// menutup non-pelanggan, tak mematikan jalur tulis yang sah.
func TestCustomerSuccess_A8_SaveAllowedForCustomer(t *testing.T) {
	env, uid := setupAccounts(t)
	pelanggan := env.seedAccount(t, "Desa A8 OK", &uid, nil, nil)
	env.makeCustomer(t, pelanggan.ID, "Active")

	form := customerSuccessFormValues()
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(pelanggan.ID)+"/customer-success", form, itoa(pelanggan.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save pelanggan harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("save pelanggan harus ok=created, got %q", loc)
	}
	if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), pelanggan.ID); err != nil {
		t.Errorf("baris CS pelanggan harus tersimpan: %v", err)
	}
}
