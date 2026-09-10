package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_consistency_test.go — BL-26: keselarasan onboarding↔lifecycle.
// Tiga fungsi MURNI (normalisasi progress, guard K1/K3, peringatan K2/K4) diuji
// langsung tanpa HTTP; jalur SAVE (normalisasi + guard) & RENDER (field progres
// kondisional + banner) diuji lewat handler nyata di atas fondasi accounts_test.go.

// --- Unit: normalizeOnboardingProgress ----------------------------------

// TestNormalizeOnboardingProgress: status terminal MEMAKSA progres (Not Started→0,
// Completed→100) apa pun angka terkirim; {In Progress, Stalled} menghormati input;
// status nil (belum dipilih) pertahankan raw apa adanya.
func TestNormalizeOnboardingProgress(t *testing.T) {
	i16 := func(v int16) *int16 { return &v }
	cases := []struct {
		name   string
		status *string
		raw    *int16
		want   *int16 // nil = harap nil
	}{
		{"Not Started paksa 0 (abaikan 50)", strptr("Not Started"), i16(50), i16(0)},
		{"Completed paksa 100 (abaikan 50)", strptr("Completed"), i16(50), i16(100)},
		{"Not Started paksa 0 walau raw nil", strptr("Not Started"), nil, i16(0)},
		{"In Progress hormati input 50", strptr("In Progress"), i16(50), i16(50)},
		{"Stalled hormati input 30", strptr("Stalled"), i16(30), i16(30)},
		{"In Progress raw nil tetap nil", strptr("In Progress"), nil, nil},
		{"status nil pertahankan raw", nil, i16(42), i16(42)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeOnboardingProgress(c.status, c.raw)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("harap nil, got %d", *got)
			case c.want != nil && got == nil:
				t.Errorf("harap %d, got nil", *c.want)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("harap %d, got %d", *c.want, *got)
			}
		})
	}
}

// --- Unit: checkOnboardingLifecycleConsistency (guard K1/K3) -------------

// dateValid = pgtype.Date terisi (untuk menandai actual_go_live_date ada).
func dateValid() pgtype.Date {
	return pgtype.Date{Time: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), Valid: true}
}

// TestCheckOnboardingLifecycleConsistency: K1 (tahap lewat Onboarding tapi belum
// Completed) & K3 (Not Started tapi go-live terisi) memblok; kombinasi selaras
// lolos. K2/K4 (lunak) SENGAJA tidak diblok di sini — diuji terpisah.
func TestCheckOnboardingLifecycleConsistency(t *testing.T) {
	cases := []struct {
		name     string
		form     customerSuccessForm
		wantCode string
		wantOK   bool
	}{
		{
			"K1: Adoption + In Progress → blok",
			customerSuccessForm{LifecycleStage: strptr("Adoption"), OnboardingStatus: strptr("In Progress")},
			errOnboardingLifecycleMismatch, false,
		},
		{
			"K1: Renewal + status nil → blok",
			customerSuccessForm{LifecycleStage: strptr("Renewal")},
			errOnboardingLifecycleMismatch, false,
		},
		{
			"K3: Not Started + go-live terisi → blok",
			customerSuccessForm{OnboardingStatus: strptr("Not Started"), ActualGoLiveDate: dateValid()},
			errOnboardingGoLiveMismatch, false,
		},
		{
			"selaras: Onboarding + In Progress → lolos",
			customerSuccessForm{LifecycleStage: strptr("Onboarding"), OnboardingStatus: strptr("In Progress")},
			"", true,
		},
		{
			"selaras: Adoption + Completed → lolos",
			customerSuccessForm{LifecycleStage: strptr("Adoption"), OnboardingStatus: strptr("Completed")},
			"", true,
		},
		{
			"selaras: Not Started tanpa go-live → lolos",
			customerSuccessForm{OnboardingStatus: strptr("Not Started")},
			"", true,
		},
		{
			"selaras: keduanya nil → lolos",
			customerSuccessForm{},
			"", true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, ok := checkOnboardingLifecycleConsistency(c.form)
			if ok != c.wantOK || code != c.wantCode {
				t.Errorf("harap (%q,%v), got (%q,%v)", c.wantCode, c.wantOK, code, ok)
			}
		})
	}
}

// --- Unit: onboardingConsistencyWarnings (K2/K4 lunak) ------------------

// TestOnboardingConsistencyWarnings: K2 (Completed tapi go-live kosong) & K4
// (masih Onboarding tapi sudah Completed) memunculkan peringatan; kombinasi
// bersih tak memunculkan apa pun. Onboarding+Completed+tanpa go-live memicu
// KEDUANYA (K2 & K4) — banner bertumpuk, bukan saling menutup.
func TestOnboardingConsistencyWarnings(t *testing.T) {
	has := func(out []string, want string) bool {
		for _, w := range out {
			if w == want {
				return true
			}
		}
		return false
	}

	// K4 murni: Onboarding + Completed TAPI go-live terisi (K2 tak aktif).
	k4 := onboardingConsistencyWarnings(db.CustomerSuccess{
		LifecycleStage: strptr("Onboarding"), OnboardingStatus: strptr("Completed"),
		ActualGoLiveDate: dateValid(),
	})
	if !has(k4, warnOnboardingCompletedStillOnboarding) {
		t.Errorf("K4 harus muncul, got %v", k4)
	}
	if has(k4, warnOnboardingCompletedNoGoLive) {
		t.Errorf("K2 tak boleh muncul saat go-live terisi, got %v", k4)
	}

	// K2 murni: Adoption + Completed TANPA go-live (K4 tak aktif, bukan Onboarding).
	k2 := onboardingConsistencyWarnings(db.CustomerSuccess{
		LifecycleStage: strptr("Adoption"), OnboardingStatus: strptr("Completed"),
	})
	if !has(k2, warnOnboardingCompletedNoGoLive) {
		t.Errorf("K2 harus muncul, got %v", k2)
	}
	if has(k2, warnOnboardingCompletedStillOnboarding) {
		t.Errorf("K4 tak boleh muncul saat tahap bukan Onboarding, got %v", k2)
	}

	// K2 & K4 bersama: Onboarding + Completed + go-live kosong.
	both := onboardingConsistencyWarnings(db.CustomerSuccess{
		LifecycleStage: strptr("Onboarding"), OnboardingStatus: strptr("Completed"),
	})
	if len(both) != 2 {
		t.Errorf("harus 2 peringatan (K2+K4), got %d: %v", len(both), both)
	}

	// Bersih: Onboarding + In Progress → tak ada peringatan.
	none := onboardingConsistencyWarnings(db.CustomerSuccess{
		LifecycleStage: strptr("Onboarding"), OnboardingStatus: strptr("In Progress"),
	})
	if len(none) != 0 {
		t.Errorf("kombinasi bersih tak boleh ada peringatan, got %v", none)
	}
}

// --- Save-path: normalisasi progress lewat handler ----------------------

// TestCustomerSuccess_OnboardingProgressNormalizedOnSave: status terminal
// menormalkan onboarding_progress di backend meski form mengirim 50 (field
// tersembunyi di UI tetap terkirim) — bukti (c) ditegakkan server-side.
func TestCustomerSuccess_OnboardingProgressNormalizedOnSave(t *testing.T) {
	cases := []struct {
		name         string
		status       string
		wantProgress int16
	}{
		{"Completed → 100", "Completed", 100},
		{"Not Started → 0", "Not Started", 0},
		{"In Progress → hormati 50", "In Progress", 50},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			a := env.seedAccount(t, "Desa Progress", &uid, nil, nil)
			env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis terbuka

			form := customerSuccessFormValues()
			form.Set("onboarding_status", c.status)
			form.Set("onboarding_progress", "50")
			form.Del("actual_go_live_date") // hindari K3 saat Not Started
			// lifecycle default Onboarding → tak memicu K1 untuk status apa pun di sini.

			req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
			rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("save harus 303, got %d\n%s", rec.Code, rec.Body.String())
			}
			got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.OnboardingProgress == nil || *got.OnboardingProgress != c.wantProgress {
				t.Errorf("onboarding_progress harus %d, got %v", c.wantProgress, got.OnboardingProgress)
			}
		})
	}
}

// --- Save-path: guard K1/K3 menolak simpan ------------------------------

// TestCustomerSuccess_ConsistencyGuardRejectsOnSave: kombinasi mustahil (K1/K3)
// ditolak dengan PRG ?err=CODE (gotcha #16) dan TAK PERNAH menyentuh DB.
func TestCustomerSuccess_ConsistencyGuardRejectsOnSave(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(f map[string][]string)
		wantErr string
	}{
		{
			"K1: Adoption tapi onboarding belum Completed",
			func(f map[string][]string) {
				f["lifecycle_stage"] = []string{"Adoption"}
				f["onboarding_status"] = []string{"In Progress"}
			},
			errOnboardingLifecycleMismatch,
		},
		{
			"K3: Not Started tapi go-live terisi",
			func(f map[string][]string) {
				f["onboarding_status"] = []string{"Not Started"}
				f["actual_go_live_date"] = []string{"2026-03-15"}
			},
			errOnboardingGoLiveMismatch,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			a := env.seedAccount(t, "Desa Guard", &uid, nil, nil)
			env.makeCustomer(t, a.ID, "Active") // BL-114: lolos gate pelanggan agar guard K1/K3 yang teruji

			form := customerSuccessFormValues()
			c.mutate(form)
			req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
			rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("guard tetap redirect PRG, got %d\n%s", rec.Code, rec.Body.String())
			}
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err="+c.wantErr) {
				t.Errorf("harus err=%s, got %q", c.wantErr, loc)
			}
			if _, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID); err == nil {
				t.Error("kombinasi kontradiktif tak boleh menyimpan baris CS")
			}
		})
	}
}

// TestCustomerSuccess_ConsistentCombinationSaves: kombinasi selaras (Adoption +
// Completed + go-live terisi) lolos guard & tersimpan (kontrol positif agar guard
// tak terbukti "menolak apa pun").
func TestCustomerSuccess_ConsistentCombinationSaves(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Selaras", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis terbuka

	form := customerSuccessFormValues()
	form.Set("lifecycle_stage", "Adoption")
	form.Set("onboarding_status", "Completed")
	form.Set("actual_go_live_date", "2026-03-15")
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("kombinasi selaras harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.OnboardingProgress == nil || *got.OnboardingProgress != 100 {
		t.Errorf("Completed → progress 100, got %v", got.OnboardingProgress)
	}
}

// --- Render: field progres kondisional + binding ------------------------

// TestCustomerSuccess_ProgressFieldConditionalMarkup: form edit (aktor berhak
// tulis Journey) merender select onboarding_status ter-bind $onbstatus + field
// progres dibungkus data-show — jaring UX klien BL-26 (c). Backend tetap penegak
// (diuji di TestCustomerSuccess_OnboardingProgressNormalizedOnSave).
func TestCustomerSuccess_ProgressFieldConditionalMarkup(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Markup", &uid, nil, nil)
	env.seedCustomerSuccess(t, a.ID)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar form sunting terbuka

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/edit", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessEdit)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET edit harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-bind="onbstatus"`) {
		t.Error("select onboarding_status harus ter-bind ke signal $onbstatus")
	}
	if !strings.Contains(body, `data-show="$onbstatus ==`) {
		t.Error("field progres harus dibungkus data-show berbasis $onbstatus")
	}
	if !strings.Contains(body, `name="onboarding_progress"`) {
		t.Error("field onboarding_progress tetap dirender (disembunyikan klien, bukan dihapus)")
	}
}

// --- Render: banner peringatan K2/K4 di detail & edit -------------------

// TestCustomerSuccess_WarningBannersRendered: baris memicu K2+K4 (Onboarding +
// Completed + go-live kosong) → banner peringatan muncul di detail DAN form edit
// (keputusan user: banner di kedua tempat). Cocokkan fragmen tanpa tanda kutip
// (gomponents meng-escape " → &#34;).
func TestCustomerSuccess_WarningBannersRendered(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Warn", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis & form sunting terbuka

	// Buat keadaan pemicu lewat handler (Onboarding + Completed, tanpa go-live).
	form := customerSuccessFormValues()
	form.Set("onboarding_status", "Completed")
	form.Del("actual_go_live_date")
	saveReq := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", saveReq, env.h.CustomerSuccessSave); rec.Code != http.StatusSeeOther {
		t.Fatalf("save pemicu harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	const (
		fragK2 = "tetapi Tanggal Go-Live Aktual belum diisi" // dari warnOnboardingCompletedNoGoLive
		fragK4 = "tahap siklus hidup masih"                  // dari warnOnboardingCompletedStillOnboarding
	)

	detReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", nil, itoa(a.ID))
	detBody := env.runAccount(uid, "owner", "admin", detReq, env.h.CustomerSuccessDetail).Body.String()
	if !strings.Contains(detBody, fragK2) || !strings.Contains(detBody, fragK4) {
		t.Error("banner K2 & K4 harus muncul di halaman DETAIL")
	}

	editReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID)+"/customer-success/edit", nil, itoa(a.ID))
	editBody := env.runAccount(uid, "owner", "admin", editReq, env.h.CustomerSuccessEdit).Body.String()
	if !strings.Contains(editBody, fragK2) || !strings.Contains(editBody, fragK4) {
		t.Error("banner K2 & K4 harus muncul di FORM edit")
	}
}
