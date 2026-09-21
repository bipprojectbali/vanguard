package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/desaplus"
)

// customer_success_sync_test.go — BL-27 tombol "Sinkron dari Desa+": gerbang
// tulis + BL-114 + ragam respons desaplus.Client (mocked via httptest.Server,
// tanpa jaringan nyata) di CustomerSuccessSync. Fungsi murni
// mapDesaPlusSummary/formatTopFeatures diuji terpisah di
// customer_success_sync_map_test.go (tanpa DB/HTTP).

// withDesaPlusClient menunjuk desaPlusClient (var paket, gotcha: global,
// dipulihkan nil di akhir test) ke server mock httptest. handler memanggil
// header/query PERSIS seperti produksi (desaplus.Client asli, bukan stub).
func withDesaPlusClient(t *testing.T, handlerFn http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handlerFn)
	t.Cleanup(func() {
		srv.Close()
		desaPlusClient = nil
	})
	desaPlusClient = desaplus.NewClient(srv.URL, "test-token")
}

// villageSummaryOK menulis balasan sukses village-summary persis kontrak API.
func villageSummaryOK(w http.ResponseWriter, lastActivityTimestamp string, activeUsers int32, topFeatures []map[string]any) {
	data := map[string]any{
		"village":            map[string]any{"id": "v1", "name": "Desa Uji", "codeVanguard": "VG-1"},
		"activeUsers":        activeUsers,
		"topFeatures":        topFeatures,
		"todayActivityCount": 3,
	}
	if lastActivityTimestamp != "" {
		data["lastActivity"] = map[string]any{
			"action": "CREATE", "feature": "Surat", "desc": "Buat surat",
			"user": "Kades", "timestamp": lastActivityTimestamp, "since": "1 hari lalu",
		}
	} else {
		data["lastActivity"] = nil
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "OK", "data": data})
}

// seedAccountWithVillageCode = seedAccount (owner/assignedCSM/backupCSM, sama
// pola accounts_test.go) + village_code terisi lewat UpdateAccount (BL-27:
// dipakai apa adanya sebagai codeVanguard). seedAccount biasa tak menyetel
// kolom ini. code=="" → tetap tanpa village_code (utk test NoVillageCode).
func (e *testEnv) seedAccountWithVillageCode(t *testing.T, name string, owner, assignedCSM, backupCSM *int64, code string) db.Account {
	t.Helper()
	a := e.seedAccount(t, name, owner, assignedCSM, backupCSM)
	if code == "" {
		return a
	}
	updated, err := e.q.UpdateAccount(t.Context(), db.UpdateAccountParams{
		ID:          a.ID,
		VillageName: a.VillageName,
		VillageCode: &code,
		AccountType: a.AccountType,
	})
	if err != nil {
		t.Fatalf("set village_code for %s: %v", name, err)
	}
	return updated
}

// --- Gerbang -----------------------------------------------------------

// TestCustomerSuccessSync_Gate403: cermin persis TestCustomerSuccess_GateWrite
// (Adoption numpang crm:journey, BL-169) — role tanpa tulis crm:journey selalu
// 403, TAK PERNAH menyentuh DB walau desaPlusClient terkonfigurasi & village_code
// terisi (gerbang F2 sebelum apa pun lain).
func TestCustomerSuccessSync_Gate403(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"csm", true},
		{"sales", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
				villageSummaryOK(w, "21 Sep 2026, 15.05", 5, nil)
			})
			var owner, csm *int64
			if c.role == "csm" {
				csm = &uid
			} else {
				owner = &uid
			}
			a := env.seedAccountWithVillageCode(t, "Desa Gate", owner, csm, nil, "VG-001")
			env.makeCustomer(t, a.ID, "Active")

			req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
			rec := env.runAccount(uid, "owner", c.role, req, env.h.CustomerSuccessSync)
			if c.allow && rec.Code != http.StatusSeeOther {
				t.Errorf("role %q harus 303, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID); err == nil {
					t.Error("role ditolak tak boleh membuat baris CS apa pun")
				}
			}
		})
	}
}

// TestCustomerSuccessSync_BL114Lock: bukan pelanggan (tanpa langganan hidup) →
// redirect TANPA aksi (sama CustomerSuccessSave), TAK menyentuh DB & TAK
// memanggil desa-plus sama sekali (server mock diberi handler yang gagal test
// bila terpanggil, membuktikan gerbang BL-114 berhenti SEBELUM panggilan API).
func TestCustomerSuccessSync_BL114Lock(t *testing.T) {
	env, uid := setupAccounts(t)
	called := false
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		villageSummaryOK(w, "21 Sep 2026, 15.05", 5, nil)
	})
	a := env.seedAccountWithVillageCode(t, "Desa Non Pelanggan", &uid, nil, nil, "VG-003")
	// SENGAJA tak dipanggil makeCustomer — tanpa langganan hidup.

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("bukan pelanggan harus tetap 303 (redirect tanpa aksi), got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "ok=") || strings.Contains(loc, "err=") {
		t.Errorf("bukan pelanggan harus redirect POLOS tanpa ok=/err=, got %q", loc)
	}
	if called {
		t.Error("BL-114: desa-plus TAK BOLEH dipanggil untuk desa bukan pelanggan")
	}
	if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID); err == nil {
		t.Error("bukan pelanggan tak boleh membuat baris CS apa pun")
	}
}

// TestCustomerSuccessSync_DesaPlusDisabled: desaPlusClient == nil (default,
// DESA_PLUS_URL/TOKEN kosong) → err=desaplus_disabled, tak menyentuh DB.
func TestCustomerSuccessSync_DesaPlusDisabled(t *testing.T) {
	env, uid := setupAccounts(t)
	desaPlusClient = nil // eksplisit — pastikan test lain tak membocorkan state
	a := env.seedAccountWithVillageCode(t, "Desa Disabled", &uid, nil, nil, "VG-004")
	env.makeCustomer(t, a.ID, "Active")

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=desaplus_disabled") {
		t.Errorf("harus err=desaplus_disabled, got %q", loc)
	}
	if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID); err == nil {
		t.Error("client nil tak boleh membuat baris CS apa pun")
	}
}

// TestCustomerSuccessSync_NoVillageCode: village_code kosong/NULL →
// err=desaplus_no_code, tak menyentuh DB, desa-plus TAK dipanggil.
func TestCustomerSuccessSync_NoVillageCode(t *testing.T) {
	env, uid := setupAccounts(t)
	called := false
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		villageSummaryOK(w, "21 Sep 2026, 15.05", 5, nil)
	})
	a := env.seedAccountWithVillageCode(t, "Desa Tanpa Kode", &uid, nil, nil, "")
	env.makeCustomer(t, a.ID, "Active")

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=desaplus_no_code") {
		t.Errorf("harus err=desaplus_no_code, got %q", loc)
	}
	if called {
		t.Error("village_code kosong TAK BOLEH memanggil desa-plus")
	}
}

// TestCustomerSuccessSync_NotFound: 404 business-level (success:false,
// data:null) → err=desaplus_not_found, baris CS TAK dibuat.
func TestCustomerSuccessSync_NotFound(t *testing.T) {
	env, uid := setupAccounts(t)
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Desa tidak ditemukan", "data": nil})
	})
	a := env.seedAccountWithVillageCode(t, "Desa Belum Berpasangan", &uid, nil, nil, "VG-BELUM-ADA")
	env.makeCustomer(t, a.ID, "Active")

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=desaplus_not_found") {
		t.Errorf("harus err=desaplus_not_found, got %q", loc)
	}
	if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID); err == nil {
		t.Error("desa tak ditemukan di desa-plus tak boleh membuat baris CS")
	}
}

// TestCustomerSuccessSync_Unauthorized: token ditolak (401) → err=desaplus_unauthorized.
func TestCustomerSuccessSync_Unauthorized(t *testing.T) {
	env, uid := setupAccounts(t)
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Unauthorized"})
	})
	a := env.seedAccountWithVillageCode(t, "Desa Token Salah", &uid, nil, nil, "VG-005")
	env.makeCustomer(t, a.ID, "Active")

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=desaplus_unauthorized") {
		t.Errorf("harus err=desaplus_unauthorized, got %q", loc)
	}
}

// TestCustomerSuccessSync_ServerError: 500 desa-plus → err=desaplus_failed
// (error umum, bukan kode spesifik).
func TestCustomerSuccessSync_ServerError(t *testing.T) {
	env, uid := setupAccounts(t)
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	})
	a := env.seedAccountWithVillageCode(t, "Desa Server Error", &uid, nil, nil, "VG-006")
	env.makeCustomer(t, a.ID, "Active")

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=desaplus_failed") {
		t.Errorf("harus err=desaplus_failed, got %q", loc)
	}
}

// --- Sukses: cakupan kolom -----------------------------------------------

// TestCustomerSuccessSync_Success_ScopedColumns: sync sukses menimpa HANYA
// last_login_date/active_users/key_features_used + usage_data_source; SELURUH
// field Health/Lifecycle/Onboarding & field manual Adoption lain
// (login_frequency/feature_adoption_rate/usage_trend/adoption_score) tetap
// UTUH dari baris seed sebelumnya (BL-27 asumsi #5 di plan).
func TestCustomerSuccessSync_Success_ScopedColumns(t *testing.T) {
	env, uid := setupAccounts(t)
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("codeVanguard"); got != "VG-007" {
			t.Errorf("codeVanguard harus 'VG-007' (village_code apa adanya), got %q", got)
		}
		if got := r.Header.Get("x-api-key"); got != "test-token" {
			t.Errorf("header x-api-key harus terkirim, got %q", got)
		}
		villageSummaryOK(w, "21 Sep 2026, 15.05", 7, []map[string]any{
			{"feature": "presensi", "count": 12},
			{"feature": "surat", "count": 8},
		})
	})
	a := env.seedAccountWithVillageCode(t, "Desa Sync Sukses", &uid, nil, nil, "VG-007")
	env.makeCustomer(t, a.ID, "Active")
	before := env.seedCustomerSuccess(t, a.ID) // health/lifecycle/onboarding/manual adoption terisi penuh

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=synced") {
		t.Errorf("harus ok=synced, got %q", loc)
	}

	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	// --- 3 kolom yang HARUS berubah ---
	if got.LastLoginDate.Time.Format("2006-01-02") != "2026-09-21" {
		t.Errorf("last_login_date harus 2026-09-21 (dari lastActivity.timestamp), got %v", got.LastLoginDate)
	}
	if got.ActiveUsers == nil || *got.ActiveUsers != 7 {
		t.Errorf("active_users harus 7, got %v", got.ActiveUsers)
	}
	if got.KeyFeaturesUsed == nil || *got.KeyFeaturesUsed != "presensi (12x), surat (8x)" {
		t.Errorf("key_features_used harus terformat dari topFeatures, got %v", got.KeyFeaturesUsed)
	}
	if got.UsageDataSource != "Product Telemetry" {
		t.Errorf("usage_data_source harus 'Product Telemetry', got %q", got.UsageDataSource)
	}

	// --- field lain HARUS UTUH dari seed sebelumnya (tak pernah ditimpa sync) ---
	if got.HealthStatus == nil || before.HealthStatus == nil || *got.HealthStatus != *before.HealthStatus {
		t.Errorf("health_status harus utuh (%v), got %v", before.HealthStatus, got.HealthStatus)
	}
	if got.OverallHealthScore == nil || before.OverallHealthScore == nil || *got.OverallHealthScore != *before.OverallHealthScore {
		t.Errorf("overall_health_score harus utuh (sync tak menghitung ulang), got %v want %v", got.OverallHealthScore, before.OverallHealthScore)
	}
	if got.LifecycleStage == nil || before.LifecycleStage == nil || *got.LifecycleStage != *before.LifecycleStage {
		t.Errorf("lifecycle_stage harus utuh, got %v want %v", got.LifecycleStage, before.LifecycleStage)
	}
	if got.OnboardingStatus == nil || before.OnboardingStatus == nil || *got.OnboardingStatus != *before.OnboardingStatus {
		t.Errorf("onboarding_status harus utuh, got %v want %v", got.OnboardingStatus, before.OnboardingStatus)
	}
	if got.LoginFrequency == nil || before.LoginFrequency == nil || *got.LoginFrequency != *before.LoginFrequency {
		t.Errorf("login_frequency (manual) harus utuh, got %v want %v", got.LoginFrequency, before.LoginFrequency)
	}
	if got.AdoptionScore == nil || before.AdoptionScore == nil || *got.AdoptionScore != *before.AdoptionScore {
		t.Errorf("adoption_score (manual, bukan telemetry) harus utuh, got %v want %v", got.AdoptionScore, before.AdoptionScore)
	}
	env.assertAudited(t, "customer_success.sync")
}

// TestCustomerSuccessSync_Success_CreatesRow: desa PELANGGAN tanpa baris CS
// sama sekali → sync membuat baris baru (jalur create, bukan hanya update),
// usage_data_source langsung "Product Telemetry".
func TestCustomerSuccessSync_Success_CreatesRow(t *testing.T) {
	env, uid := setupAccounts(t)
	withDesaPlusClient(t, func(w http.ResponseWriter, r *http.Request) {
		villageSummaryOK(w, "", 0, nil) // lastActivity nil, activeUsers 0 (valid), topFeatures kosong
	})
	a := env.seedAccountWithVillageCode(t, "Desa Baru Sync", &uid, nil, nil, "VG-008")
	env.makeCustomer(t, a.ID, "Active")

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/sync", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSync)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("baris CS harus terbuat: %v", err)
	}
	if got.LastLoginDate.Valid {
		t.Errorf("lastActivity nil → last_login_date harus tetap NULL (tak ada fallback existing utk baris baru), got %v", got.LastLoginDate)
	}
	if got.ActiveUsers == nil || *got.ActiveUsers != 0 {
		t.Errorf("active_users harus 0 (nilai VALID dari API, bukan data hilang), got %v", got.ActiveUsers)
	}
	if got.UsageDataSource != "Product Telemetry" {
		t.Errorf("usage_data_source harus 'Product Telemetry', got %q", got.UsageDataSource)
	}
}
