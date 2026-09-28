package handler

import (
	"net/http"
	"net/url"
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

// --- Unit: normalizeOnboardingDates (BL-178 Rule B) ----------------------

// TestNormalizeOnboardingDates: "In Progress" mengisi kickoff_date HANYA bila
// kosong; "Completed" mengisi actual_go_live_date HANYA bila kosong; "Not
// Started"/"Stalled"/nil tak menyentuh field apa pun (Stalled: input bebas
// manual; target_go_live_date tak pernah disentuh fungsi ini sama sekali).
func TestNormalizeOnboardingDates(t *testing.T) {
	empty := pgtype.Date{}
	filled := dateValid() // 2026-03-15, lihat helper di bawah
	cases := []struct {
		name             string
		status           *string
		kickoff          pgtype.Date
		actualGoLive     pgtype.Date
		wantKickoff      pgtype.Date
		wantActualGoLive pgtype.Date
	}{
		{"In Progress + kickoff kosong → terisi hari ini", strptr("In Progress"), empty, empty, dateOnly(todayInAppTZ()), empty},
		{"In Progress + kickoff sudah terisi → tak berubah", strptr("In Progress"), filled, empty, filled, empty},
		{"Completed + go-live kosong → terisi hari ini", strptr("Completed"), empty, empty, empty, dateOnly(todayInAppTZ())},
		{"Completed + go-live sudah terisi → tak berubah", strptr("Completed"), empty, filled, empty, filled},
		{"Not Started → tak ada auto-isi", strptr("Not Started"), empty, empty, empty, empty},
		{"Stalled → tak ada auto-isi (input bebas manual)", strptr("Stalled"), empty, empty, empty, empty},
		{"status nil → tak ada auto-isi", nil, empty, empty, empty, empty},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotKickoff, gotActualGoLive := normalizeOnboardingDates(c.status, c.kickoff, c.actualGoLive)
			if gotKickoff.Valid != c.wantKickoff.Valid || (gotKickoff.Valid && !gotKickoff.Time.Equal(c.wantKickoff.Time)) {
				t.Errorf("kickoff_date: harap %v, got %v", c.wantKickoff, gotKickoff)
			}
			if gotActualGoLive.Valid != c.wantActualGoLive.Valid || (gotActualGoLive.Valid && !gotActualGoLive.Time.Equal(c.wantActualGoLive.Time)) {
				t.Errorf("actual_go_live_date: harap %v, got %v", c.wantActualGoLive, gotActualGoLive)
			}
		})
	}
}

// --- Unit: normalizeStageEntryDate (BL-178 Rule E) -----------------------

// TestNormalizeStageEntryDate: lifecycle_stage BERUBAH dari existing (termasuk
// baris baru, existing kosong) → today; TAK berubah → pertahankan existing
// (bukan reset tiap save); stage nil → pertahankan existing apa adanya.
func TestNormalizeStageEntryDate(t *testing.T) {
	old := dateValid() // 2026-03-15
	cases := []struct {
		name     string
		existing db.CustomerSuccess
		stage    *string
		want     pgtype.Date
	}{
		{
			"baris baru (existing kosong) + stage terisi → hari ini",
			db.CustomerSuccess{},
			strptr("Onboarding"),
			dateOnly(todayInAppTZ()),
		},
		{
			"stage berubah dari existing → hari ini (timpa nilai lama)",
			db.CustomerSuccess{LifecycleStage: strptr("Onboarding"), StageEntryDate: old},
			strptr("Adoption"),
			dateOnly(todayInAppTZ()),
		},
		{
			"stage SAMA dgn existing → pertahankan nilai lama (bukan reset)",
			db.CustomerSuccess{LifecycleStage: strptr("Adoption"), StageEntryDate: old},
			strptr("Adoption"),
			old,
		},
		{
			"stage nil → pertahankan existing apa adanya",
			db.CustomerSuccess{LifecycleStage: strptr("Adoption"), StageEntryDate: old},
			nil,
			old,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeStageEntryDate(c.existing, c.stage)
			if got.Valid != c.want.Valid || (got.Valid && !got.Time.Equal(c.want.Time)) {
				t.Errorf("harap %v, got %v", c.want, got)
			}
		})
	}
}

// --- Unit: checkOnboardingRegression (BL-178 Guard C/D1) -----------------

// TestCheckOnboardingRegression: Guard C (onboarding_status sudah maju lewat
// "Not Started" tak boleh balik ke "Not Started"/blank) & Guard D1
// (lifecycle_stage sudah post-onboarding tak boleh balik ke "Onboarding"
// literal/blank) — keduanya diuji atas baris `existing` TERSIMPAN, bukan form
// saat ini. Baris baru (existing zero-value) & baris yang BELUM pernah maju
// lolos kedua guard (belum ada apa pun untuk dimundurkan).
func TestCheckOnboardingRegression(t *testing.T) {
	cases := []struct {
		name     string
		existing db.CustomerSuccess
		form     customerSuccessForm
		wantCode string
		wantOK   bool
	}{
		{
			"Guard C: existing In Progress, form Not Started → blok",
			db.CustomerSuccess{OnboardingStatus: strptr("In Progress")},
			customerSuccessForm{OnboardingStatus: strptr("Not Started")},
			errOnboardingStatusRegression, false,
		},
		{
			"Guard C: existing In Progress, form blank/nil → blok",
			db.CustomerSuccess{OnboardingStatus: strptr("In Progress")},
			customerSuccessForm{OnboardingStatus: nil},
			errOnboardingStatusRegression, false,
		},
		{
			"Guard C: existing In Progress, form Completed (maju) → lolos",
			db.CustomerSuccess{OnboardingStatus: strptr("In Progress")},
			customerSuccessForm{OnboardingStatus: strptr("Completed")},
			"", true,
		},
		{
			"Guard C: existing baris baru (nil) → lolos apa pun form",
			db.CustomerSuccess{},
			customerSuccessForm{OnboardingStatus: strptr("Not Started")},
			"", true,
		},
		{
			"Guard C: existing masih Not Started (belum maju) → lolos",
			db.CustomerSuccess{OnboardingStatus: strptr("Not Started")},
			customerSuccessForm{OnboardingStatus: strptr("Not Started")},
			"", true,
		},
		{
			"Guard D1: existing Adoption (post), form Onboarding literal → blok",
			db.CustomerSuccess{LifecycleStage: strptr("Adoption")},
			customerSuccessForm{LifecycleStage: strptr("Onboarding")},
			errLifecycleStageRegression, false,
		},
		{
			"Guard D1: existing Adoption (post), form blank/nil → blok",
			db.CustomerSuccess{LifecycleStage: strptr("Adoption")},
			customerSuccessForm{LifecycleStage: nil},
			errLifecycleStageRegression, false,
		},
		{
			"Guard D1: existing Adoption (post), form Retention (maju) → lolos",
			db.CustomerSuccess{LifecycleStage: strptr("Adoption")},
			customerSuccessForm{LifecycleStage: strptr("Retention")},
			"", true,
		},
		{
			"Guard D1: existing masih Onboarding (belum post) → lolos apa pun form",
			db.CustomerSuccess{LifecycleStage: strptr("Onboarding")},
			customerSuccessForm{LifecycleStage: nil},
			"", true,
		},
		{
			"Guard D1: existing lifecycle nil (baris baru) → lolos",
			db.CustomerSuccess{},
			customerSuccessForm{LifecycleStage: nil},
			"", true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, ok := checkOnboardingRegression(c.existing, c.form)
			if ok != c.wantOK || code != c.wantCode {
				t.Errorf("harap (%q,%v), got (%q,%v)", c.wantCode, c.wantOK, code, ok)
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

// --- Save-path: Rule B (auto-isi tanggal dari transisi status) ----------

// TestCustomerSuccess_OnboardingDatesAutoFilledOnSave: "In Progress" mengisi
// kickoff_date hari ini bila kosong TAPI tak menimpa isian manual pada save
// berikutnya; "Completed" mengisi actual_go_live_date dgn pola sama.
func TestCustomerSuccess_OnboardingDatesAutoFilledOnSave(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Tanggal", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis terbuka

	// In Progress + kickoff_date kosong → terisi otomatis hari ini.
	form := customerSuccessFormValues()
	form.Set("onboarding_status", "In Progress")
	form.Del("kickoff_date")
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave); rec.Code != http.StatusSeeOther {
		t.Fatalf("save pertama harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	wantToday := dateOnly(todayInAppTZ())
	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.KickoffDate.Valid || !got.KickoffDate.Time.Equal(wantToday.Time) {
		t.Errorf("kickoff_date harus terisi hari ini (%v), got %v", wantToday.Time, got.KickoffDate)
	}

	// Save lagi dgn kickoff_date manual beda → TAK ditimpa (field sudah terisi).
	form2 := customerSuccessFormValues()
	form2.Set("onboarding_status", "In Progress")
	form2.Set("kickoff_date", "2026-02-01")
	req2 := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form2, itoa(a.ID))
	if rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.CustomerSuccessSave); rec2.Code != http.StatusSeeOther {
		t.Fatalf("save kedua harus 303, got %d", rec2.Code)
	}
	wantManual := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	got2, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if !got2.KickoffDate.Valid || !got2.KickoffDate.Time.Equal(wantManual) {
		t.Errorf("kickoff_date manual harus tersimpan %v, got %v", wantManual, got2.KickoffDate)
	}

	// Baris terpisah: Completed + actual_go_live_date kosong → terisi otomatis.
	env2, uid2 := setupAccounts(t)
	b := env2.seedAccount(t, "Desa Tanggal Golive", &uid2, nil, nil)
	env2.makeCustomer(t, b.ID, "Active")
	form3 := customerSuccessFormValues()
	form3.Set("onboarding_status", "Completed")
	req3 := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(b.ID)+"/customer-success", form3, itoa(b.ID))
	if rec3 := env2.runAccount(uid2, "owner", "admin", req3, env2.h.CustomerSuccessSave); rec3.Code != http.StatusSeeOther {
		t.Fatalf("save Completed harus 303, got %d\n%s", rec3.Code, rec3.Body.String())
	}
	got3, err := env2.q.GetCustomerSuccessByAccountID(t.Context(), b.ID)
	if err != nil {
		t.Fatalf("get3: %v", err)
	}
	if !got3.ActualGoLiveDate.Valid || !got3.ActualGoLiveDate.Time.Equal(wantToday.Time) {
		t.Errorf("actual_go_live_date harus terisi hari ini, got %v", got3.ActualGoLiveDate)
	}
}

// --- Save-path: Rule E (auto-isi stage_entry_date dari transisi stage) ---

// TestCustomerSuccess_StageEntryDateAutoFilledOnSave: save PERTAMA (baris baru)
// → stage_entry_date terisi hari ini. Save KEDUA dgn lifecycle_stage SAMA →
// stage_entry_date TAK berubah (bukan reset tiap save). Save KETIGA dgn
// lifecycle_stage BERBEDA (maju, lolos Guard D1) → stage_entry_date reset ke
// hari ini lagi. Field stage_entry_date TAK dikirim sama sekali di form (BL-178
// Rule E: dihapus dari parseCustomerSuccessForm) — murni turunan server.
func TestCustomerSuccess_StageEntryDateAutoFilledOnSave(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Stage Entry", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis terbuka

	// Save pertama: baris baru, lifecycle default "Onboarding" → stage_entry_date hari ini.
	form := customerSuccessFormValues()
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave); rec.Code != http.StatusSeeOther {
		t.Fatalf("save pertama harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	wantToday := dateOnly(todayInAppTZ())
	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.StageEntryDate.Valid || !got.StageEntryDate.Time.Equal(wantToday.Time) {
		t.Errorf("stage_entry_date save pertama harus hari ini (%v), got %v", wantToday.Time, got.StageEntryDate)
	}
	firstEntryDate := got.StageEntryDate

	// Save kedua: lifecycle_stage SAMA ("Onboarding") → stage_entry_date TAK berubah.
	form2 := customerSuccessFormValues()
	req2 := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form2, itoa(a.ID))
	if rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.CustomerSuccessSave); rec2.Code != http.StatusSeeOther {
		t.Fatalf("save kedua harus 303, got %d\n%s", rec2.Code, rec2.Body.String())
	}
	got2, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if !got2.StageEntryDate.Time.Equal(firstEntryDate.Time) {
		t.Errorf("stage_entry_date tak boleh berubah saat stage sama, got %v, want %v", got2.StageEntryDate, firstEntryDate)
	}

	// Save ketiga: lifecycle_stage BERUBAH (maju ke Adoption, lolos Guard D1) →
	// stage_entry_date reset ke hari ini lagi (walau harinya kebetulan sama di
	// test ini, ini menegaskan jalur "berubah" tetap dipanggil, bukan skip).
	form3 := customerSuccessFormValues()
	form3.Set("lifecycle_stage", "Adoption")
	form3.Set("onboarding_status", "Completed") // selaras K1, lolos Guard D1 (maju)
	req3 := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form3, itoa(a.ID))
	if rec3 := env.runAccount(uid, "owner", "admin", req3, env.h.CustomerSuccessSave); rec3.Code != http.StatusSeeOther {
		t.Fatalf("save ketiga harus 303, got %d\n%s", rec3.Code, rec3.Body.String())
	}
	got3, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get3: %v", err)
	}
	if !got3.StageEntryDate.Valid || !got3.StageEntryDate.Time.Equal(wantToday.Time) {
		t.Errorf("stage_entry_date harus hari ini setelah stage berubah, got %v", got3.StageEntryDate)
	}
}

// --- Save-path: Guard C/D1 menolak transisi mundur ----------------------

// TestCustomerSuccess_GuardCRejectsOnboardingStatusRegression: sekali
// onboarding_status maju ke "In Progress", form yang mencoba balik ke "Not
// Started" ATAUPUN mengosongkannya ditolak PRG ?err= — baris DB tak berubah.
func TestCustomerSuccess_GuardCRejectsOnboardingStatusRegression(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Guard C", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis terbuka

	// Maju dulu: onboarding_status "In Progress" (lolos K1/K3, tersimpan).
	form := customerSuccessFormValues()
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave); rec.Code != http.StatusSeeOther {
		t.Fatalf("save maju harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	cases := []struct {
		name   string
		mutate func(f url.Values)
	}{
		{"balik ke Not Started", func(f url.Values) { f.Set("onboarding_status", "Not Started") }},
		{"balik ke blank/null", func(f url.Values) { f.Del("onboarding_status") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f2 := customerSuccessFormValues()
			c.mutate(f2)
			req2 := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", f2, itoa(a.ID))
			rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.CustomerSuccessSave)
			if rec2.Code != http.StatusSeeOther {
				t.Fatalf("guard tetap redirect PRG, got %d\n%s", rec2.Code, rec2.Body.String())
			}
			if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "err="+errOnboardingStatusRegression) {
				t.Errorf("harus err=%s, got %q", errOnboardingStatusRegression, loc)
			}
			got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.OnboardingStatus == nil || *got.OnboardingStatus != "In Progress" {
				t.Errorf("onboarding_status tak boleh berubah, got %v", got.OnboardingStatus)
			}
		})
	}
}

// TestCustomerSuccess_GuardD1RejectsLifecycleStageRegression: sekali
// lifecycle_stage masuk postOnboardingStages ("Adoption"), form yang mencoba
// balik ke "Onboarding" literal ATAUPUN mengosongkannya ditolak PRG ?err= —
// keputusan user eksplisit: blank diperlakukan SAMA seperti literal Onboarding.
func TestCustomerSuccess_GuardD1RejectsLifecycleStageRegression(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Guard D1", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis terbuka

	// Maju dulu: Adoption + Completed (selaras K1), tersimpan post-onboarding.
	form := customerSuccessFormValues()
	form.Set("lifecycle_stage", "Adoption")
	form.Set("onboarding_status", "Completed")
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave); rec.Code != http.StatusSeeOther {
		t.Fatalf("save maju harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	cases := []struct {
		name   string
		mutate func(f url.Values)
	}{
		{"balik ke Onboarding literal", func(f url.Values) { f.Set("lifecycle_stage", "Onboarding") }},
		{"balik ke blank/null", func(f url.Values) { f.Del("lifecycle_stage") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f2 := customerSuccessFormValues()
			f2.Set("onboarding_status", "Completed") // hindari K1 palsu, fokus ke Guard D1
			c.mutate(f2)
			req2 := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", f2, itoa(a.ID))
			rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.CustomerSuccessSave)
			if rec2.Code != http.StatusSeeOther {
				t.Fatalf("guard tetap redirect PRG, got %d\n%s", rec2.Code, rec2.Body.String())
			}
			if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "err="+errLifecycleStageRegression) {
				t.Errorf("harus err=%s, got %q", errLifecycleStageRegression, loc)
			}
			got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.LifecycleStage == nil || *got.LifecycleStage != "Adoption" {
				t.Errorf("lifecycle_stage tak boleh berubah, got %v", got.LifecycleStage)
			}
		})
	}
}

// TestCustomerSuccess_GuardsAllowForwardTransitions: kontrol positif — guard
// mundur TAK memblok transisi MAJU lebih lanjut (Adoption→Retention), baik
// onboarding_status maupun lifecycle_stage. Guard tak terbukti "menolak apa pun".
func TestCustomerSuccess_GuardsAllowForwardTransitions(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Guard Maju", &uid, nil, nil)
	env.makeCustomer(t, a.ID, "Active") // BL-114: pelanggan agar jalur tulis terbuka

	form := customerSuccessFormValues()
	form.Set("lifecycle_stage", "Adoption")
	form.Set("onboarding_status", "Completed")
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CustomerSuccessSave); rec.Code != http.StatusSeeOther {
		t.Fatalf("save pertama harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	form2 := customerSuccessFormValues()
	form2.Set("lifecycle_stage", "Retention") // maju lagi, bukan mundur
	form2.Set("onboarding_status", "Completed")
	req2 := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/customer-success", form2, itoa(a.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.CustomerSuccessSave)
	if rec2.Code != http.StatusSeeOther {
		t.Fatalf("maju lebih lanjut harus tetap 303, got %d\n%s", rec2.Code, rec2.Body.String())
	}
	if loc := rec2.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Errorf("transisi maju tak boleh kena guard, location=%q", loc)
	}
	got, err := env.q.GetCustomerSuccessByAccountID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.LifecycleStage == nil || *got.LifecycleStage != "Retention" {
		t.Errorf("lifecycle_stage harus Retention, got %v", got.LifecycleStage)
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

	// Keadaan pemicu (Onboarding + Completed, TANPA go-live) diseed LANGSUNG
	// lewat pool (bypass handler+Rule B) — sejak BL-178, save via handler
	// otomatis mengisi actual_go_live_date saat status "Completed" & field
	// masih kosong (normalizeOnboardingDates), jadi K2 tak lagi bisa dipicu
	// lewat SAVE untuk baris baru. K2 tetap relevan utk data lama/tersinkron
	// di luar jalur normalisasi ini — banner-nya sendiri TAK diubah BL-178.
	lifecycle, onboarding := "Onboarding", "Completed"
	if _, err := env.q.CreateCustomerSuccess(t.Context(), db.CreateCustomerSuccessParams{
		TenantID:         env.tenantID,
		AccountID:        a.ID,
		LifecycleStage:   &lifecycle,
		OnboardingStatus: &onboarding,
		UsageDataSource:  "Manual",
	}); err != nil {
		t.Fatalf("seed keadaan pemicu K2/K4: %v", err)
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
