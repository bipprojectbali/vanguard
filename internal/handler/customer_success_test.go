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

// TestDeriveHealthStatus mengunci pemetaan skor→status (BL-24) di ambang
// ≥80 Healthy · 40–79 At-Risk · <40 Critical. Batas atas/bawah tiap band diuji
// eksplisit (79 vs 80, 39 vs 40) karena di situlah off-by-one paling mungkin;
// nil (belum ada dasar hitung) HARUS nil, bukan default ke status apa pun.
func TestDeriveHealthStatus(t *testing.T) {
	i16 := func(v int16) *int16 { return &v }
	cases := []struct {
		name string
		in   *int16
		want *string // nil = harap nil
	}{
		{"nil belum dinilai", nil, nil},
		{"0 kritis", i16(0), strptr("Critical")},
		{"39 kritis (batas atas band)", i16(39), strptr("Critical")},
		{"40 at-risk (batas bawah band)", i16(40), strptr("At-Risk")},
		{"79 at-risk (batas atas band)", i16(79), strptr("At-Risk")},
		{"80 healthy (batas bawah band)", i16(80), strptr("Healthy")},
		{"100 healthy", i16(100), strptr("Healthy")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deriveHealthStatus(c.in)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("in=%v: harap nil, got %q", c.in, *got)
			case c.want != nil && got == nil:
				t.Errorf("in=%v: harap %q, got nil", c.in, *c.want)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("in=%v: harap %q, got %q", c.in, *c.want, *got)
			}
		})
	}
}

// TestDeriveScoreTrend: arah tren dari (prev, curr) dgn dead-band. Batas eksplisit
// di sekitar ±deadband + kasus nil (belum ada pembanding → nil, BUKAN "Stable").
func TestDeriveScoreTrend(t *testing.T) {
	i16 := func(v int16) *int16 { return &v }
	const dead = healthTrendDeadband // 3
	cases := []struct {
		name       string
		prev, curr *int16
		want       *string // nil = harap nil
	}{
		{"prev nil → nil (snapshot pertama)", nil, i16(80), nil},
		{"curr nil → nil (skor kosong)", i16(80), nil, nil},
		{"keduanya nil → nil", nil, nil, nil},
		{"delta 0 → Stable", i16(50), i16(50), strptr("Stable")},
		{"delta +3 (= dead-band) → Stable", i16(50), i16(53), strptr("Stable")},
		{"delta +4 (> dead-band) → Improving", i16(50), i16(54), strptr("Improving")},
		{"delta −3 (= dead-band) → Stable", i16(50), i16(47), strptr("Stable")},
		{"delta −4 (< −dead-band) → Declining", i16(50), i16(46), strptr("Declining")},
		{"lonjakan besar naik → Improving", i16(10), i16(90), strptr("Improving")},
		{"terjun besar → Declining", i16(90), i16(10), strptr("Declining")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deriveScoreTrend(c.prev, c.curr, dead)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("prev=%v curr=%v: harap nil, got %q", c.prev, c.curr, *got)
			case c.want != nil && got == nil:
				t.Errorf("prev=%v curr=%v: harap %q, got nil", c.prev, c.curr, *c.want)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("prev=%v curr=%v: harap %q, got %q", c.prev, c.curr, *c.want, *got)
			}
		})
	}
}

func strptr(s string) *string { return &s }

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
			env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar gate F2 write yang teruji (bukan gate pelanggan)

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
	env.makeCustomer(t, mine.ID, "Active") // BL-114: pelanggan agar F3 (bukan gate pelanggan) yang buka edit binaan sendiri

	eReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(theirs.ID)+"/customer-success/edit", nil, itoa(theirs.ID))
	if rec := env.runAccount(csmA, "member", "csm", eReq, env.h.CustomerSuccessEdit); rec.Code != http.StatusNotFound {
		t.Errorf("edit CS di luar cakupan harus 404, got %d", rec.Code)
	}

	okReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(mine.ID)+"/customer-success/edit", nil, itoa(mine.ID))
	if rec := env.runAccount(csmA, "member", "csm", okReq, env.h.CustomerSuccessEdit); rec.Code != http.StatusOK {
		t.Errorf("edit CS binaan sendiri harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
}

// --- BL-108: penugasan CS pindah ke halaman detail Customer Success -----

// TestCustomerSuccess_AssignCardDitampilkan: kartu "Penugasan CS" (form POST
// /assign) dirender di halaman detail CS bagi aktor ber-tulis-account — bahkan
// pada EMPTY-STATE (baris CS belum ada), sebab penugasan harus bisa dilakukan
// sebelum data CS pernah diisi. csm = pemilik binaan (F3) + crm:accounts write.
// BL-146: penugasan CS kini juga digerbangi hasLiveSub (aksi tulis khusus
// pelanggan aktif) — desa dibuat pelanggan di sini supaya test tetap menguji
// niat aslinya (empty-state, bukan status pelanggan).
func TestCustomerSuccess_AssignCardDitampilkan(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Tugas CS", nil, &uid, nil) // csm = owner binaan
	env.makeCustomer(t, a.ID, "Active")

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", nil, itoa(a.ID))
	rec := env.runAccount(uid, "member", "csm", req, env.h.CustomerSuccessDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Penugasan CS") {
		t.Error("BL-108: kartu Penugasan CS harus tampil di halaman detail Customer Success")
	}
	// SUBMIT: form harus posting ke aksi /assign desa ini (verifikasi UI mencakup
	// jalur simpan, bukan cuma keberadaan judul).
	if !strings.Contains(body, "/accounts/"+itoa(a.ID)+"/assign") {
		t.Error("BL-108: form Penugasan CS harus posting ke /accounts/{id}/assign")
	}
}

// Catatan: gerbang CanAssign=false (aktor bisa BACA halaman CS tapi TAK berhak
// tulis account) diuji di level view murni — TestCustomerSuccessDetail_
// AssignCardTanpaTulisTersembunyi di paket panel — karena tak ada role bawaan
// begini yang lolos F3 lewat HTTP (support ber-accounts-read ditolak F3;
// sales/csm/admin/manager semua ber-accounts-write).

// TestAccounts_EditFormTanpaKartuAssign: form SUNTING desa TAK lagi memuat kartu
// Penugasan CS (BL-108 memindahkannya ke halaman Customer Success) — mencegah
// dua pintu penugasan yang sama.
func TestAccounts_EditFormTanpaKartuAssign(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Sunting", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/edit", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountEdit)
	if rec.Code != http.StatusOK {
		t.Fatalf("form edit harus 200, got %d", rec.Code)
	}
	// Tombol submit unik kartu assign ("Simpan Penugasan") sbg penanda — BUKAN
	// frasa "Penugasan CS" yang kini sah muncul di hint form (mengarahkan ke
	// halaman Customer Success). Yang harus hilang: kartu interaktifnya.
	body := rec.Body.String()
	if strings.Contains(body, "Simpan Penugasan") {
		t.Error("BL-108: form edit desa TAK boleh lagi memuat kartu Penugasan CS (tombol Simpan Penugasan)")
	}
	if strings.Contains(body, "/assign") {
		t.Error("BL-108: form edit desa TAK boleh lagi memuat form POST /assign")
	}
}

// --- Create (baris belum ada) -------------------------------------------

// TestCustomerSuccess_CreateEmptyState: GET detail pada desa TANPA baris CS
// → 200 + pesan empty-state (bukan 404 — desanya tetap ada).
