package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// customer_success_test.go — Modul 6 slice B1 (Health/Journey+Onboarding/
// Adoption). Tiga sumbu izin, semua di atas fondasi accounts_test.go
// (setupAccounts/accountsReq/runAccount/seedAccount):
//
//   - F2 per-section (BUKAN satu gerbang tunggal seperti modul lain): baca
//     `crm:health`/`crm:journey`/`crm:adoption` independen — section yang
//     tak berhak baca disembunyikan dari body (bukan seluruh halaman 403);
//     tulis MASKED per-section saat SAVE, digerbangi minimal-satu-section
//     saat GET edit.
//   - F3 (ownership): DIWARISI account induk lewat loadOwnedAccount — sama
//     persis pola accounts_fls_test.go, termasuk temuan penting: Support
//     (DataScope=None) 404 di SEMUA desa, bukan cuma nol-baris di daftar,
//     jadi skenario "support: health saja" TAK BISA dibuktikan lewat HTTP
//     nyata (F3 menolak sebelum F2 section-visibility sempat jalan) — diuji
//     eksplisit sebagai temuan tersendiri, bukan diam-diam dilewati.
//
// Masking per-section (F2 tulis) diuji LANGSUNG ke applyCustomerSuccessMasking
// (fungsi murni), BUKAN lewat peran nyata: business_policy.csv TAK punya satu
// pun role dengan akses tulis SEBAGIAN dari 3 section (admin/manager/csm penuh,
// sales/support nol) — jalur itu memang tak bisa dilatih lewat HTTP hari ini.

// customerSuccessFormValues merakit form values LENGKAP & SAH mencakup ketiga
// section — dipakai admin/manager/csm (full-access) di test create/update/enum.
// adoption+engagement+support+sentiment = 80+90+70+60 → overall_health_score
// = 75 pas (habis dibagi 4, tanpa pembulatan yang perlu didebat).
func customerSuccessFormValues() url.Values {
	return url.Values{
		"health_status":         {"Healthy"},
		"adoption_score":        {"80"},
		"engagement_score":      {"90"},
		"support_score":         {"70"},
		"sentiment_score":       {"60"},
		"score_trend":           {"Improving"},
		"lifecycle_stage":       {"Onboarding"},
		"stage_entry_date":      {"2026-01-15"},
		"onboarding_status":     {"In Progress"},
		"kickoff_date":          {"2026-01-10"},
		"target_go_live_date":   {"2026-03-01"},
		"onboarding_progress":   {"50"},
		"last_login_date":       {"2026-08-10"},
		"active_users":          {"12"},
		"login_frequency":       {"Weekly"},
		"feature_adoption_rate": {"65.5"},
		"key_features_used":     {"Dashboard, Laporan"},
		"usage_trend":           {"Increasing"},
	}
}

// seedCustomerSuccess menaruh satu baris CS langsung lewat pool (bypass
// handler, tanpa masking) — untuk test yang butuh baris SUDAH ADA (read-gate,
// update). Mengisi ketiga section agar assert kemunculan judul kartu berarti.
func (e *testEnv) seedCustomerSuccess(t *testing.T, accountID int64) db.CustomerSuccess {
	t.Helper()
	health, trend := "Healthy", "Improving"
	lifecycle, onboarding, freq, usage := "Onboarding", "In Progress", "Weekly", "Increasing"
	adoption, engagement, support, sentiment := int16(80), int16(90), int16(70), int16(60)
	progress := int16(50)
	activeUsers := int32(12)
	cs, err := e.q.CreateCustomerSuccess(t.Context(), db.CreateCustomerSuccessParams{
		TenantID:           e.tenantID,
		AccountID:          accountID,
		OverallHealthScore: computeOverallHealthScore(&adoption, &engagement, &support, &sentiment),
		HealthStatus:       &health,
		AdoptionScore:      &adoption,
		EngagementScore:    &engagement,
		SupportScore:       &support,
		SentimentScore:     &sentiment,
		ScoreTrend:         &trend,
		LifecycleStage:     &lifecycle,
		OnboardingStatus:   &onboarding,
		OnboardingProgress: &progress,
		ActiveUsers:        &activeUsers,
		LoginFrequency:     &freq,
		UsageTrend:         &usage,
	})
	if err != nil {
		t.Fatalf("seed customer success: %v", err)
	}
	return cs
}

// assertSection memastikan judul kartu section muncul (atau sengaja tak
// muncul) di body — persis 3 string yang dipakai customer_success_view.go.
func assertSection(t *testing.T, body, title string, want bool) {
	t.Helper()
	has := strings.Contains(body, title)
	if has != want {
		t.Errorf("section %q: want muncul=%v, got %v", title, want, has)
	}
}

// Marker UNIK per section — BUKAN judul kartu ("Health Score"/"Product
// Adoption" bertabrakan dengan label nav sidebar workspaceCSGroup yang
// SELALU tampil disabled; "Journey & Onboarding" lolos escape gomponents jadi
// "Journey &amp; Onboarding" di HTML mentah). Label detailField Indonesia ini
// hanya muncul di dalam kartu section itu sendiri, tak pernah di nav.
const (
	healthMarker   = "Skor Kesehatan Keseluruhan"
	journeyMarker  = "Tahap Siklus Hidup"
	adoptionMarker = "Tingkat Adopsi Fitur"
)

// --- F2: baca per-section ----------------------------------------------

// TestCustomerSuccess_GateReadPerSection: admin/manager/csm melihat SEMUA
// section (write policy row otomatis mencakup read, business.conf matcher);
// sales melihat Health+Journey TAPI BUKAN Adoption (tak ada baris crm:adoption
// sama sekali di business_defaults.go untuk sales). Support DIKECUALIKAN dari
// tabel ini — lihat TestCustomerSuccess_F3_SupportSelaluDitolak.
func TestCustomerSuccess_GateReadPerSection(t *testing.T) {
	cases := []struct {
		role                                  string
		wantHealth, wantJourney, wantAdoption bool
	}{
		{"admin", true, true, true},
		{"manager", true, true, true},
		{"csm", true, true, true},
		{"sales", true, true, false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			var owner, csm *int64
			if c.role == "csm" {
				csm = &uid
			} else {
				owner = &uid
			}
			a := env.seedAccount(t, "Desa CS", owner, csm, nil)
			env.seedCustomerSuccess(t, a.ID)

			req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", nil, itoa(a.ID))
			rec := env.runAccount(uid, "owner", c.role, req, env.h.CustomerSuccessDetail)
			if rec.Code != http.StatusOK {
				t.Fatalf("role %q harus 200, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			assertSection(t, body, healthMarker, c.wantHealth)
			assertSection(t, body, journeyMarker, c.wantJourney)
			assertSection(t, body, adoptionMarker, c.wantAdoption)
		})
	}
}

// TestCustomerSuccess_F3_SupportSelaluDitolak: Support lolos gate F2 read
// (crm:health) TAPI DataScope=None → loadOwnedAccount menolak SEMUA desa
// (F3, diwarisi accounts, bukan CS-spesifik — mirror TestAccounts_F3_
// SupportNolBaris tapi di rute DETAIL: F3 menolak SEBELUM F2 section-
// visibility sempat merender apa pun, jadi "support hanya lihat Health"
// tak pernah teruji lewat HTTP nyata — dicatat eksplisit di sini, bukan
// diam-diam dilewati).
func TestCustomerSuccess_F3_SupportSelaluDitolak(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Ada", &uid, nil, nil)
	env.seedCustomerSuccess(t, a.ID)

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "support", req, env.h.CustomerSuccessDetail)
	if rec.Code != http.StatusNotFound {
		t.Errorf("support F3 ScopeNone harus 404 di SEMUA desa (warisan loadOwnedAccount), got %d", rec.Code)
	}
}

// --- F2: tulis (minimal satu section) -----------------------------------

// TestCustomerSuccess_GateWrite: admin/manager/csm boleh buka form edit &
// simpan; sales & support (tak ada satu pun baris "write" crm:*) & ""
// ditolak di KEDUA rute (GET edit, POST save), tak pernah menyentuh DB.
func TestCustomerSuccess_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"sales", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			var owner, csm *int64
			if c.role == "csm" {
				csm = &uid
			} else {
				owner = &uid
			}
			a := env.seedAccount(t, "Desa Tulis", owner, csm, nil)

			editReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/edit", nil, itoa(a.ID))
			editRec := env.runAccount(uid, "owner", c.role, editReq, env.h.CustomerSuccessEdit)
			if c.allow && editRec.Code != http.StatusOK {
				t.Errorf("role %q harus 200 di GET edit, got %d\n%s", c.role, editRec.Code, editRec.Body.String())
			}
			if !c.allow && editRec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403 di GET edit, got %d", c.role, editRec.Code)
			}

			form := customerSuccessFormValues()
			saveReq := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
			saveRec := env.runAccount(uid, "owner", c.role, saveReq, env.h.CustomerSuccessSave)
			if c.allow && saveRec.Code != http.StatusSeeOther {
				t.Errorf("role %q harus 303 di POST save, got %d\n%s", c.role, saveRec.Code, saveRec.Body.String())
			}
			if !c.allow && saveRec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403 di POST save, got %d", c.role, saveRec.Code)
			}
			if !c.allow {
				if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID); err == nil {
					t.Error("role ditolak tak boleh menyimpan baris CS apa pun")
				}
			}
		})
	}
}

// --- F3: di luar cakupan (mirror TestAccounts_F3_SalesLihatMiliknya) ---

// TestCustomerSuccess_F3_AccountDiLuarCakupan404_Detail: sales (F2-read lolos
// utk health/journey) melihat CS desa miliknya sendiri, tapi 404 di desa milik
// sales lain — F3 (loadOwnedAccount) yang menolak, bukan gerbang F2.
func TestCustomerSuccess_F3_AccountDiLuarCakupan404_Detail(t *testing.T) {
	env, salesA := setupAccounts(t)
	salesB := env.seedMember(t, "salesb@local", "member", 0).ID
	mine := env.seedAccount(t, "Desa Milik A", &salesA, nil, nil)
	theirs := env.seedAccount(t, "Desa Milik B", &salesB, nil, nil)
	env.seedCustomerSuccess(t, mine.ID)
	env.seedCustomerSuccess(t, theirs.ID)

	dReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(theirs.ID)+"/customer-success", nil, itoa(theirs.ID))
	if rec := env.runAccount(salesA, "member", "sales", dReq, env.h.CustomerSuccessDetail); rec.Code != http.StatusNotFound {
		t.Errorf("detail CS di luar cakupan harus 404, got %d", rec.Code)
	}

	okReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(mine.ID)+"/customer-success", nil, itoa(mine.ID))
	if rec := env.runAccount(salesA, "member", "sales", okReq, env.h.CustomerSuccessDetail); rec.Code != http.StatusOK {
		t.Errorf("detail CS desa sendiri harus 200, got %d", rec.Code)
	}
}

// TestCustomerSuccess_F3_AccountDiLuarCakupan404_Edit: rute edit/save gerbang
// F2-write (canWriteCS) SEBELUM F3 — sales tak punya SATU PUN baris write
// crm:*, jadi selalu 403 di edit terlepas dari kepemilikan (bukan bug, lihat
// TestCustomerSuccess_GateWrite). Untuk membuktikan F3 benar-benar berjalan
// di rute tulis, dipakai csm (DataScope=Own, write penuh ke 3 section) —
// gerbang F2-nya lolos utk desa mana pun, sehingga F3 (loadOwnedAccount) yang
// membedakan binaannya sendiri (200) dari binaan csm lain (404).
func TestCustomerSuccess_F3_AccountDiLuarCakupan404_Edit(t *testing.T) {
	env, csmA := setupAccounts(t)
	csmB := env.seedMember(t, "csmb@local", "member", 0).ID
	mine := env.seedAccount(t, "Desa Binaan A", nil, &csmA, nil)
	theirs := env.seedAccount(t, "Desa Binaan B", nil, &csmB, nil)
	env.seedCustomerSuccess(t, mine.ID)
	env.seedCustomerSuccess(t, theirs.ID)

	eReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(theirs.ID)+"/customer-success/edit", nil, itoa(theirs.ID))
	if rec := env.runAccount(csmA, "member", "csm", eReq, env.h.CustomerSuccessEdit); rec.Code != http.StatusNotFound {
		t.Errorf("edit CS di luar cakupan harus 404, got %d", rec.Code)
	}

	okReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(mine.ID)+"/customer-success/edit", nil, itoa(mine.ID))
	if rec := env.runAccount(csmA, "member", "csm", okReq, env.h.CustomerSuccessEdit); rec.Code != http.StatusOK {
		t.Errorf("edit CS binaan sendiri harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
}

// --- Create (baris belum ada) -------------------------------------------

// TestCustomerSuccess_CreateEmptyState: GET detail pada desa TANPA baris CS
// → 200 + pesan empty-state (bukan 404 — desanya tetap ada).
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
