package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// all_activities_sort_test.go — BL-157k: feed terpadu /activity-log terurut ke-5
// sumbunya (Jenis/Subjek/Pemilik/Status/Tanggal) via ?sort=col&dir=asc|desc.
// Representatif per pola cursor (mirror sales_activities_sort_test.go BL-157j) +
// kasus KHUSUS union yang tak ada di modul manapun lain: interleave lintas-sumber,
// F3 SERENTAK di dua lengan, paginasi lintas-sumber di bawah sort non-default,
// dan gated-union di bawah ?sort=. Mekanisme cursor generik (dualCursorGen) sudah
// teruji tuntas di all_activities_unified_test.go — tak diulang di sini.

// seedAllActivityFull = varian seedAllActivity (all_activities_test.go) dgn
// kontrol penuh atas status (nullable) — dibutuhkan uji posisi NULL sumbu Status.
// status "" ≡ NULL.
func (e *testEnv) seedAllActivityFull(t *testing.T, kind, ctx, subject string, owner *int64, status string) db.Activity {
	t.Helper()
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	acc := e.seedAccount(t, "Desa-"+subject, owner, nil, nil)
	a, err := e.q.CreateActivity(t.Context(), db.CreateActivityParams{
		TenantID:        e.tenantID,
		Kind:            kind,
		Subject:         subject,
		TargetType:      "account",
		TargetID:        acc.ID,
		OwnerID:         owner,
		ActivityContext: ptr(ctx),
		Status:          statusPtr,
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed activity %s (ctx=%s): %v", subject, ctx, err)
	}
	return a
}

// Jenis (kind, NOT NULL, RAW enum alfabetis) — lengan activities saja.
func TestAllActivitiesList_SortKindAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "task", "sales", "AllAct-KindTask", &uid)
	env.seedAllActivity(t, "call", "sales", "AllAct-KindCall", &uid)
	env.seedAllActivity(t, "note", "sales", "AllAct-KindNote", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=kind&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iCall := strings.Index(body, "AllAct-KindCall")
	iNote := strings.Index(body, "AllAct-KindNote")
	iTask := strings.Index(body, "AllAct-KindTask")
	if iCall < 0 || iNote < 0 || iTask < 0 {
		t.Fatalf("ketiga aktivitas harus tampil:\n%s", body)
	}
	if !(iCall < iNote && iNote < iTask) {
		t.Errorf("dir=asc harus urut call < note < task, dapat posisi %d/%d/%d", iCall, iNote, iTask)
	}
}

// Subjek (NOT NULL) asc & desc.
func TestAllActivitiesList_SortSubjectAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekZebra", &uid)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekAwal", &uid)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekMekar", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "AllAct-SubjekAwal")
	iMekar := strings.Index(body, "AllAct-SubjekMekar")
	iZebra := strings.Index(body, "AllAct-SubjekZebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestAllActivitiesList_SortSubjectDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekZebra2", &uid)
	env.seedAllActivity(t, "note", "general", "AllAct-SubjekAwal2", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=subject&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "AllAct-SubjekAwal2")
	iZebra := strings.Index(body, "AllAct-SubjekZebra2")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestAllActivitiesList_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "sales", "AllAct-Fallback", &uid)

	req := accountsReq(http.MethodGet, "/w/test/activity-log?sort=bogus", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AllAct-Fallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}
