package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// customer_success_crud_test.go — dipisah dari customer_success_test.go agar tiap
// file di bawah ambang file-health. Berisi test perilaku CRUD: create (empty
// state + skor kesehatan), update, masking per-section (fungsi murni), dan
// penolakan enum tak sah. Gerbang izin (F2/F3) tetap di customer_success_test.go.

func TestCustomerSuccess_CreateEmptyState(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Baru", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("desa tanpa baris CS harus 200 empty-state, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Belum ada data Customer Success untuk desa ini.") {
		t.Error("harus menampilkan pesan empty-state persis")
	}
}

// TestCustomerSuccess_CreateThenOverallHealthScore: POST save pertama kali →
// baris baru (get-then-branch), overall_health_score dihitung server-side dari
// komponennya (BUKAN dari field form — form tak pernah mengirimnya), dan
// health_last_calculated terisi karena section Health benar-benar ditulis.
func TestCustomerSuccess_CreateThenOverallHealthScore(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Baru", &uid, nil, nil)

	form := customerSuccessFormValues()
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create pertama kali harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("harus ok=created, got %q", loc)
	}

	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get customer success: %v", err)
	}
	if got.OverallHealthScore == nil || *got.OverallHealthScore != 75 {
		t.Errorf("overall_health_score harus 75 (rata2 80/90/70/60), got %v", got.OverallHealthScore)
	}
	if !got.HealthLastCalculated.Valid {
		t.Error("health_last_calculated harus terisi saat section Health ditulis kali ini")
	}
	env.assertAudited(t, "customer_success.save")
}

// TestCustomerSuccess_UpdateSuccess: baris sudah ada → jalur Update (bukan
// Create), field baru tersimpan, ok=saved (bukan ok=created).
func TestCustomerSuccess_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Ada", &uid, nil, nil)
	env.seedCustomerSuccess(t, a.ID)

	form := customerSuccessFormValues()
	form.Set("health_status", "Critical")
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("update harus ok=saved, got %q", loc)
	}

	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.HealthStatus == nil || *got.HealthStatus != "Critical" {
		t.Errorf("health_status harus terupdate ke Critical, got %v", got.HealthStatus)
	}
}

// --- Masking F2 per-section: fungsi murni, TANPA HTTP -------------------

// TestApplyCustomerSuccessMasking: skenario defensif (plan slice B1) — tak
// ada business_role bawaan dengan akses tulis SEBAGIAN section, jadi kontrak
// ini dikunci langsung ke applyCustomerSuccessMasking, bukan lewat peran
// nyata. Field section yang TAK berhak ditulis harus tetap nilai existing;
// yang berhak harus lolos nilai form baru.
func TestApplyCustomerSuccessMasking(t *testing.T) {
	newHealth, oldHealth := "Critical", "Healthy"
	newAdoption, oldAdoption := int16(10), int16(90)
	newLifecycle, oldLifecycle := "Renewal", "Onboarding"
	newFreq, oldFreq := "Daily", "Weekly"

	form := customerSuccessForm{
		HealthStatus:   &newHealth,
		AdoptionScore:  &newAdoption,
		LifecycleStage: &newLifecycle,
		LoginFrequency: &newFreq,
	}
	existing := db.CustomerSuccess{
		HealthStatus:   &oldHealth,
		AdoptionScore:  &oldAdoption,
		LifecycleStage: &oldLifecycle,
		LoginFrequency: &oldFreq,
	}

	// Tak berhak menulis section apa pun → seluruh field kembali ke existing.
	none := applyCustomerSuccessMasking(form, existing, false, false, false)
	if none.HealthStatus != existing.HealthStatus || none.AdoptionScore != existing.AdoptionScore {
		t.Error("Health harus di-mask penuh ke nilai existing")
	}
	if none.LifecycleStage != existing.LifecycleStage {
		t.Error("Journey harus di-mask penuh ke nilai existing")
	}
	if none.LoginFrequency != existing.LoginFrequency {
		t.Error("Adoption harus di-mask penuh ke nilai existing")
	}

	// Hanya berhak menulis Journey → Health & Adoption di-mask, Journey lolos.
	partial := applyCustomerSuccessMasking(form, existing, false, true, false)
	if partial.HealthStatus != existing.HealthStatus {
		t.Error("Health tetap harus di-mask walau Journey berhak ditulis")
	}
	if partial.LifecycleStage != form.LifecycleStage {
		t.Error("Journey yang berhak ditulis harus lolos nilai form baru")
	}
	if partial.LoginFrequency != existing.LoginFrequency {
		t.Error("Adoption tetap harus di-mask")
	}

	// Berhak menulis ketiganya → form lolos utuh, tak ada yang di-mask.
	all := applyCustomerSuccessMasking(form, existing, true, true, true)
	if all.HealthStatus != form.HealthStatus || all.LifecycleStage != form.LifecycleStage || all.LoginFrequency != form.LoginFrequency {
		t.Error("berhak tulis semua section harus lolos form baru, tak ada masking")
	}
}

// --- Enum invalid → ditolak SEBELUM DB -----------------------------------

// TestCustomerSuccess_EnumInvalidRejected: nilai enum/angka/tanggal tak sah
// ditolak parseCustomerSuccessForm dengan kode yang benar, PRG (?err=CODE),
// dan TAK PERNAH menyentuh DB (baris tak tersimpan).
func TestCustomerSuccess_EnumInvalidRejected(t *testing.T) {
	cases := []struct {
		field, value, wantErr string
	}{
		{"health_status", "Bogus", "health_status"},
		{"adoption_score", "150", "score"},
		{"score_trend", "Sideways", "score_trend"},
		{"lifecycle_stage", "Unknown", "lifecycle_stage"},
		{"onboarding_status", "Unknown", "onboarding_status"},
		{"login_frequency", "Sometimes", "login_frequency"},
		{"usage_trend", "Flat", "usage_trend"},
		{"stage_entry_date", "31-12-2026", "date"},
		{"active_users", "-5", "number"},
		{"feature_adoption_rate", "abc", "feature_adoption_rate"},
	}
	for _, c := range cases {
		t.Run("field="+c.field, func(t *testing.T) {
			env, uid := setupAccounts(t)
			a := env.seedAccount(t, "Desa Invalid", &uid, nil, nil)

			form := customerSuccessFormValues()
			form.Set(c.field, c.value)
			req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
			rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("form invalid tetap redirect PRG, got %d\n%s", rec.Code, rec.Body.String())
			}
			loc := rec.Header().Get("Location")
			if !strings.Contains(loc, "err="+c.wantErr) {
				t.Errorf("field %q=%q harus err=%s, got %q", c.field, c.value, c.wantErr, loc)
			}
			if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID); err == nil {
				t.Error("form invalid tak boleh menyimpan baris CS apa pun")
			}
		})
	}
}
