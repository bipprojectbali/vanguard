package handler

import (
	"net/http"
	"strings"
	"testing"
)

// activities_search_test.go — regresi BL-6 slice 5: pencarian daftar aktivitas
// polimorfik (?q=) untuk KEDUA halaman: /activities (ActivitiesList, context=sales)
// & /activity-log (AllActivitiesList, lintas-context). Dua sifat WAJIB benar
// bersama di sisi handler:
//   - MENYEMPITKAN: ?q= menyaring baris via ILIKE contains (case-insensitive) pada
//     subject — satu-satunya kolom teks bebas yang TAMPIL. kind/status/target
//     (enum/id) & owner (nama diresolusi handler) bukan kunci cari.
//   - TAK MEMPERLUAS: pencarian berjalan DI ATAS filter kepemilikan F3 — aktor
//     'own' (sales) yang mencari kata cocok dengan aktivitas milik orang lain tetap
//     tak melihatnya. q hanya menyaring di dalam cakupan, bukan menembusnya.
// (Kontrak URL kotak cari/pager diuji di view: panel/activities_search_test.go.)

// activitiesSearchBody menjalankan ActivitiesList dengan ?q= tertentu.
func (e *testEnv) activitiesSearchBody(t *testing.T, uid int64, wsRole, bizRole, q string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/activities?q="+q, nil, "")
	rec := e.runAccount(uid, wsRole, bizRole, req, e.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivitiesList status %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// allActivitiesSearchBody menjalankan AllActivitiesList dengan ?q= tertentu.
func (e *testEnv) allActivitiesSearchBody(t *testing.T, uid int64, wsRole, bizRole, q string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/activity-log?q="+q, nil, "")
	rec := e.runAccount(uid, wsRole, bizRole, req, e.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("AllActivitiesList status %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestActivitiesList_SearchNarrows: ?q= menyaring subject (case-insensitive) di
// halaman Sales Activities.
func TestActivitiesList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa A", &uid, nil, nil)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Panggilan Follow-Up", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Kirim Proposal", &uid)

	body := env.activitiesSearchBody(t, uid, "owner", "admin", "follow")
	if !strings.Contains(body, "Panggilan Follow-Up") {
		t.Errorf("q=follow harus memuat aktivitas dengan subjek cocok (case-insensitive)")
	}
	if strings.Contains(body, "Kirim Proposal") {
		t.Errorf("q=follow tak boleh memuat aktivitas yang subjeknya tak cocok")
	}
}

// TestActivitiesList_SearchCannotBypassF3: pencarian tak menembus F3 — aktor 'own'
// (sales) yang mencari subjek milik owner lain tetap tak melihatnya.
func TestActivitiesList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)
	acc := env.seedAccount(t, "Desa Bersama", &uid, nil, nil)

	env.seedSalesActivity(t, "note", "account", acc.ID, "Rahasia Milikku", &uid)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Rahasia Orang", &lain.ID)

	body := env.activitiesSearchBody(t, uid, "member", "sales", "Rahasia")
	if !strings.Contains(body, "Rahasia Milikku") {
		t.Errorf("sales harus melihat aktivitas MILIKNYA yang cocok pencarian")
	}
	if strings.Contains(body, "Rahasia Orang") {
		t.Errorf("pencarian tak boleh menembus F3: sales melihat aktivitas milik orang lain")
	}
}

// TestAllActivitiesList_SearchNarrows: ?q= menyaring subject lintas-context di
// halaman Activities top-level.
func TestAllActivitiesList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "sales", "Demo Produk", &uid)
	env.seedAllActivity(t, "note", "cs", "Onboarding Desa", &uid)

	body := env.allActivitiesSearchBody(t, uid, "owner", "admin", "onboarding")
	if !strings.Contains(body, "Onboarding Desa") {
		t.Errorf("q=onboarding harus memuat aktivitas cocok lintas-context (case-insensitive)")
	}
	if strings.Contains(body, "Demo Produk") {
		t.Errorf("q=onboarding tak boleh memuat aktivitas yang tak cocok")
	}
}

// TestAllActivitiesList_SearchCannotBypassF3: F3 tetap penjaga — aktor 'own' tak
// melihat aktivitas milik orang lain walau subjek cocok pencarian.
func TestAllActivitiesList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain2@x", "member", env.tenantID)

	env.seedAllActivity(t, "note", "sales", "Sensitif Milikku", &uid)
	env.seedAllActivity(t, "note", "cs", "Sensitif Orang", &lain.ID)

	body := env.allActivitiesSearchBody(t, uid, "member", "sales", "Sensitif")
	if !strings.Contains(body, "Sensitif Milikku") {
		t.Errorf("aktor 'own' harus melihat aktivitas MILIKNYA yang cocok pencarian")
	}
	if strings.Contains(body, "Sensitif Orang") {
		t.Errorf("pencarian tak boleh menembus F3 di halaman lintas-context")
	}
}
