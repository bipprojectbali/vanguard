package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
)

// cs_journey_scope_test.go — bagian F3 (ownership) & keyset dari
// cs_journey_test.go, dipisah agar tiap file di bawah ambang tipe Test (400).
// Setup & gerbang F2 tetap di cs_journey_test.go (paket sama).

// --- F3: ownership -----------------------------------------------------------

// TestCSJourney_F3_CSMLihatBinaan: CSM (data_scope='own') hanya melihat desa di
// mana ia terdaftar (account_owner/assigned_csm/backup_csm). Desa milik user
// lain tak muncul di tabel utama.
func TestCSJourney_F3_CSMLihatBinaan(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-csm-journey@local", "member", 0).ID

	// Desa A: uid sebagai assigned_csm → tampil ke uid (CSM).
	accA := env.seedAccount(t, "Desa Binaan Journey", nil, &uid, nil)
	env.seedCSJourneyRow(t, accA.ID, "Adoption", daysAgoDate(10), nil, nil, pgtype.Date{})

	// Desa B: other sebagai assigned_csm → TIDAK tampil ke uid.
	accB := env.seedAccount(t, "Desa Orang Lain Journey", nil, &other, nil)
	env.seedCSJourneyRow(t, accB.ID, "Adoption", daysAgoDate(20), nil, nil, pgtype.Date{})

	code, body := env.csJourneyBody(t, uid, "member", "csm")
	if code != http.StatusOK {
		t.Fatalf("CSM harus 200, got %d\n%s", code, body)
	}
	if !strings.Contains(body, "Desa Binaan Journey") {
		t.Errorf("CSM harus melihat desa binaannya")
	}
	if strings.Contains(body, "Desa Orang Lain Journey") {
		t.Errorf("CSM TAK boleh melihat desa binaan orang lain (F3 bocor)")
	}
}

// TestCSJourney_F3_AdminLihatSemua: Admin (data_scope='all') melihat SEMUA desa
// dalam workspace, termasuk yang assigned_csm-nya user lain.
func TestCSJourney_F3_AdminLihatSemua(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-admin-journey@local", "member", 0).ID

	accA := env.seedAccount(t, "Desa Alpha Journey", &uid, nil, nil)
	env.seedCSJourneyRow(t, accA.ID, "Adoption", daysAgoDate(10), nil, nil, pgtype.Date{})

	accB := env.seedAccount(t, "Desa Beta Journey", nil, &other, nil)
	env.seedCSJourneyRow(t, accB.ID, "Retention", daysAgoDate(20), nil, nil, pgtype.Date{})

	code, body := env.csJourneyBody(t, uid, "owner", "admin")
	if code != http.StatusOK {
		t.Fatalf("Admin harus 200, got %d\n%s", code, body)
	}
	if !strings.Contains(body, "Desa Alpha Journey") {
		t.Errorf("Admin harus melihat Desa Alpha")
	}
	if !strings.Contains(body, "Desa Beta Journey") {
		t.Errorf("Admin harus melihat Desa Beta")
	}
}

// TestCSJourney_F3_KPIRespectScope: KPI CSM (own) hanya menghitung desa
// binaannya — desa orang lain tak masuk hitungan.
func TestCSJourney_F3_KPIRespectScope(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-kpi-journey@local", "member", 0).ID

	own := env.seedAccount(t, "Desa Own KPI", nil, &uid, nil)
	env.seedCSJourneyRow(t, own.ID, "Onboarding", daysAgoDate(10), nil, nil, pgtype.Date{})

	foreign := env.seedAccount(t, "Desa Asing KPI", nil, &other, nil)
	env.seedCSJourneyRow(t, foreign.ID, "Onboarding", daysAgoDate(15), nil, nil, pgtype.Date{})

	filter := db.CSJourneyListFilterFor("own")
	kpis, err := env.q.CountCSJourneyKPIs(t.Context(), db.CountCSJourneyKPIsParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		t.Fatalf("CountCSJourneyKPIs: %v", err)
	}
	if kpis.OnboardingCount != 1 {
		t.Errorf("onboarding_count (own scope) = %d, want 1 (hanya desa binaan)", kpis.OnboardingCount)
	}
}

// --- Keyset pagination -------------------------------------------------------

// TestCSJourney_KeysetRoundTrip: ListCSJourneyAccounts mengurut ASC lama-di-fase
// (stage_entry_date lebih tua = lebih dulu) dan cursor keyset (csJourneyKeyOf)
// melanjutkan ke baris berikutnya tanpa lompat/ulang.
func TestCSJourney_KeysetRoundTrip(t *testing.T) {
	env, uid := setupAccounts(t)

	// Tiga desa, stage_entry_date makin lama → urutan ASC: C(30) → B(20) → A(10).
	accA := env.seedAccount(t, "Desa Urut A", &uid, nil, nil)
	accB := env.seedAccount(t, "Desa Urut B", &uid, nil, nil)
	accC := env.seedAccount(t, "Desa Urut C", &uid, nil, nil)
	env.seedCSJourneyRow(t, accA.ID, "Adoption", daysAgoDate(10), nil, nil, pgtype.Date{})
	env.seedCSJourneyRow(t, accB.ID, "Adoption", daysAgoDate(20), nil, nil, pgtype.Date{})
	env.seedCSJourneyRow(t, accC.ID, "Adoption", daysAgoDate(30), nil, nil, pgtype.Date{})

	at, id := firstPageCursorAsc()
	page1, err := env.q.ListCSJourneyAccounts(t.Context(), db.ListCSJourneyAccountsParams{
		CursorAt: at, CursorID: id, ScopeAll: true, Uid: &uid, FilterStage: "", PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListCSJourneyAccounts page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}
	// ASC lama-di-fase: baris pertama = paling lama (C, 30 hari).
	if page1[0].AccountName != "Desa Urut C" {
		t.Errorf("page1[0] = %q, want 'Desa Urut C' (paling lama di fase)", page1[0].AccountName)
	}
	if page1[1].AccountName != "Desa Urut B" {
		t.Errorf("page1[1] = %q, want 'Desa Urut B'", page1[1].AccountName)
	}

	// Cursor dari baris terakhir page1 → page2 tak mengulang & lanjut ke A.
	nextAt, nextID := csJourneyKeyOf(page1[1])
	page2, err := env.q.ListCSJourneyAccounts(t.Context(), db.ListCSJourneyAccountsParams{
		CursorAt: nextAt, CursorID: nextID, ScopeAll: true, Uid: &uid, FilterStage: "", PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListCSJourneyAccounts page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("page2 len = %d, want 1 (sisa satu baris)", len(page2))
	}
	if page2[0].AccountName != "Desa Urut A" {
		t.Errorf("page2[0] = %q, want 'Desa Urut A'", page2[0].AccountName)
	}
}

// TestCSJourney_FilterStage: filter_stage tak-kosong hanya mengembalikan fase
// yang cocok; kosong -> semua.
func TestCSJourney_FilterStage(t *testing.T) {
	env, uid := setupAccounts(t)

	accOn := env.seedAccount(t, "Desa Filter Onboarding", &uid, nil, nil)
	accAd := env.seedAccount(t, "Desa Filter Adoption", &uid, nil, nil)
	env.seedCSJourneyRow(t, accOn.ID, "Onboarding", daysAgoDate(10), nil, nil, pgtype.Date{})
	env.seedCSJourneyRow(t, accAd.ID, "Adoption", daysAgoDate(20), nil, nil, pgtype.Date{})

	at, id := firstPageCursorAsc()
	rows, err := env.q.ListCSJourneyAccounts(t.Context(), db.ListCSJourneyAccountsParams{
		CursorAt: at, CursorID: id, ScopeAll: true, Uid: &uid, FilterStage: "Adoption", PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListCSJourneyAccounts filter: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("filter Adoption → len = %d, want 1", len(rows))
	}
	if rows[0].AccountName != "Desa Filter Adoption" {
		t.Errorf("filter Adoption → %q, want 'Desa Filter Adoption'", rows[0].AccountName)
	}
}
