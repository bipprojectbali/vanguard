package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sales_activities_test.go — halaman & aksi Sales Activity Log (4.4, GET /activities
// + POST create). Yang dijaga khusus di lapis handler (di luar activities_test.go
// yang menguji query murni):
//
//   - F3 pada LIST: aktor scope IsOwn (sales) hanya melihat aktivitas MILIKNYA
//     (owner_id); ScopeAll (admin) lintas-owner.
//   - Baris hidup saja: aktivitas ter-soft-delete tak muncul di daftar.
//   - Gate izin: tanpa crm:sales_activity → 403 (bukan halaman kosong senyap).
//   - Create: picker target "type:id" diurai & baris tersimpan lalu tampil.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA filter/gerbang handler;
// isolasi RLS antar-workspace diuji terpisah.

// seedSalesActivity menaruh satu aktivitas Sales langsung (bypass handler) dengan
// owner tertentu — jangkar F3 (list menyaring pada owner_id, bukan target).
func (e *testEnv) seedSalesActivity(t *testing.T, kind, targetType string, targetID int64, subject string, owner *int64) db.Activity {
	t.Helper()
	a, err := e.q.CreateActivity(t.Context(), db.CreateActivityParams{
		TenantID:        e.tenantID,
		Kind:            kind,
		Subject:         subject,
		TargetType:      targetType,
		TargetID:        targetID,
		OwnerID:         owner,
		ActivityContext: ptr(activityContextSales),
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed activity %s: %v", subject, err)
	}
	return a
}

// activitiesListBody menjalankan ActivitiesList untuk (uid, role) → HTML shell.
func (e *testEnv) activitiesListBody(t *testing.T, uid int64, wsRole, bizRole string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/activities", nil, "")
	rec := e.runAccount(uid, wsRole, bizRole, req, e.h.ActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("ActivitiesList status %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestActivitiesList_F3_ScopeFiltersByOwner: aktivitas disaring pada owner_id.
// IsOwn (sales) lihat punya sendiri, TIDAK punya owner lain; ScopeAll (admin) lihat
// keduanya. Membuktikan ActivitiesListFilterFor ikut menyaring list.
func TestActivitiesList_F3_ScopeFiltersByOwner(t *testing.T) {
	env, actor := setupAccounts(t)
	ownerB := env.seedMember(t, "ownerb@local", "member", 0).ID
	acc := env.seedAccount(t, "Desa A", &actor, nil, nil)

	env.seedSalesActivity(t, "note", "account", acc.ID, "Catatan-Sendiri", &actor)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Catatan-Lain", &ownerB)

	own := env.activitiesListBody(t, actor, "member", "sales")
	if !strings.Contains(own, "Catatan-Sendiri") {
		t.Errorf("IsOwn harus melihat aktivitas sendiri")
	}
	if strings.Contains(own, "Catatan-Lain") {
		t.Errorf("IsOwn tak boleh melihat aktivitas owner lain")
	}

	all := env.activitiesListBody(t, actor, "owner", "admin")
	if !strings.Contains(all, "Catatan-Sendiri") || !strings.Contains(all, "Catatan-Lain") {
		t.Errorf("ScopeAll harus melihat kedua aktivitas")
	}
}

// TestActivitiesList_SoftDeletedHidden: aktivitas ter-soft-delete disaring dari
// daftar (ListActivities deleted_at IS NULL); baris hidup tetap tampil.
func TestActivitiesList_SoftDeletedHidden(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	env.seedSalesActivity(t, "note", "account", acc.ID, "Hidup", &uid)
	gone := env.seedSalesActivity(t, "note", "account", acc.ID, "Terhapus", &uid)

	if err := env.q.SoftDeleteActivity(t.Context(), db.SoftDeleteActivityParams{
		ID: gone.ID, UpdatedBy: &uid,
	}); err != nil {
		t.Fatalf("soft delete activity: %v", err)
	}

	body := env.activitiesListBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Hidup") {
		t.Errorf("aktivitas hidup harus tampil")
	}
	if strings.Contains(body, "Terhapus") {
		t.Errorf("aktivitas terhapus tak boleh tampil")
	}
}

// TestActivitiesList_ForbiddenWithoutPerm: peran tanpa crm:sales_activity (CSM)
// ditolak 403 + penjelasan — bukan halaman kosong yang senyap.
func TestActivitiesList_ForbiddenWithoutPerm(t *testing.T) {
	env, uid := setupAccounts(t)
	req := accountsReq(http.MethodGet, "/activities", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.ActivitiesList)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tanpa crm:sales_activity harus 403, got %d", rec.Code)
	}
}

// TestActivityCreate_ParsesTargetAndPersists: POST create dengan picker target
// "account:<id>" → 303 ke detail, dan aktivitas tampil di daftar aktor (scope own).
// Membuktikan parse target + integritas (targetInScope) + persist jalur end-to-end.
func TestActivityCreate_ParsesTargetAndPersists(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Target", &uid, nil, nil)

	form := url.Values{}
	form.Set("kind", "note")
	form.Set("target", "account:"+itoa(acc.ID))
	form.Set("subject", "Aktivitas Baru")
	form.Set("body", "isi catatan")

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	// Slug tak lolos via helper test (param "slug" vs "workspace") → asersi pada
	// kode PRG sukses, konvensi sama dengan contacts/accounts. Persist + parse
	// target dibuktikan oleh daftar di bawah.
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("create sukses harus redirect ok=created, got %q", loc)
	}

	body := env.activitiesListBody(t, uid, "member", "sales")
	if !strings.Contains(body, "Aktivitas Baru") {
		t.Errorf("aktivitas baru harus tampil di daftar")
	}
}

// TestActivityCreate_RejectsTargetOutOfScope: target milik owner lain di luar
// cakupan aktor (sales IsOwn) → tolak sebagai target tak sah (redirect ?err), tak
// tersimpan. Menjaga integritas polimorfik target_id (bukan FK).
func TestActivityCreate_RejectsTargetOutOfScope(t *testing.T) {
	env, actor := setupAccounts(t)
	ownerB := env.seedMember(t, "ownerb@local", "member", 0).ID
	accB := env.seedAccount(t, "Desa Orang Lain", &ownerB, nil, nil)

	form := url.Values{}
	form.Set("kind", "note")
	form.Set("target", "account:"+itoa(accB.ID))
	form.Set("subject", "Curi Target")

	req := accountsReq(http.MethodPost, "/activities", form, "")
	rec := env.runAccount(actor, "member", "sales", req, env.h.ActivityCreate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus redirect (303), got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=activity_target") {
		t.Errorf("target di luar cakupan harus err=activity_target, got %q", loc)
	}
	// Tak boleh tersimpan (aktor tak melihat apa pun di daftarnya).
	body := env.activitiesListBody(t, actor, "member", "sales")
	if strings.Contains(body, "Curi Target") {
		t.Errorf("aktivitas dengan target di luar cakupan tak boleh tersimpan")
	}
}

// ── Unit (tanpa DB) ─────────────────────────────────────────────────────────

// TestParseActivityTarget: picker "type:id" diurai & divalidasi bentuk+enum di
// handler. Nilai user-controlled → penolakan tegas (keberadaan baris dicek terpisah).
func TestParseActivityTarget(t *testing.T) {
	cases := []struct {
		in       string
		wantType string
		wantID   int64
		wantOK   bool
	}{
		{"deal:123", "deal", 123, true},
		{"account:7", "account", 7, true},
		{"contact:9", "contact", 9, true},
		{"deal:0", "", 0, false},   // id harus > 0
		{"ticket:5", "", 0, false}, // tipe di luar picker v1
		{"deal", "", 0, false},     // tanpa ":"
		{"deal:abc", "", 0, false}, // id bukan angka
		{"", "", 0, false},         // kosong
	}
	for _, c := range cases {
		gt, gid, gok := parseActivityTarget(c.in)
		if gt != c.wantType || gid != c.wantID || gok != c.wantOK {
			t.Errorf("parseActivityTarget(%q)=(%q,%d,%v), want (%q,%d,%v)",
				c.in, gt, gid, gok, c.wantType, c.wantID, c.wantOK)
		}
	}
}
