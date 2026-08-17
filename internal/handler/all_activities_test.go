package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// all_activities_test.go — halaman Activities lintas-context (M7, GET /activity-log).
// Menguji AllActivitiesList handler. Yang dijaga:
//   - F3 ownership: IsOwn hanya lihat milik sendiri; ScopeAll lihat semua.
//   - Lintas-context: aktivitas context=sales, context=cs, context=general
//     semua muncul (berbeda dari ActivitiesList yang hanya context=sales).
//   - Soft-delete: aktivitas terhapus tak tampil.
//   - Gate izin: tanpa crm:sales_activity → 403.
//
// Seed via seedSalesActivity (context=sales) + seedAllActivity (context lain).

// seedAllActivity menaruh satu aktivitas dengan activity_context tertentu.
func (e *testEnv) seedAllActivity(t *testing.T, kind, ctx, subject string, owner *int64) db.Activity {
	t.Helper()
	acc := e.seedAccount(t, "Desa-"+subject, owner, nil, nil)
	a, err := e.q.CreateActivity(t.Context(), db.CreateActivityParams{
		TenantID:        e.tenantID,
		Kind:            kind,
		Subject:         subject,
		TargetType:      "account",
		TargetID:        acc.ID,
		OwnerID:         owner,
		ActivityContext: ptr(ctx),
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed activity %s (ctx=%s): %v", subject, ctx, err)
	}
	return a
}

// allActivitiesListBody menjalankan AllActivitiesList → HTML shell.
func (e *testEnv) allActivitiesListBody(t *testing.T, uid int64, wsRole, bizRole string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/activity-log", nil, "")
	rec := e.runAccount(uid, wsRole, bizRole, req, e.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("AllActivitiesList status %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestAllActivitiesList_ShowsAllContexts: aktivitas dari context sales, cs, dan
// general semuanya muncul di halaman lintas-context. Membuktikan ListAllActivities
// tidak memfilter per activity_context.
func TestAllActivitiesList_ShowsAllContexts(t *testing.T) {
	env, uid := setupAccounts(t)

	env.seedAllActivity(t, "note", "sales", "Aktivitas-Sales", &uid)
	env.seedAllActivity(t, "note", "cs", "Aktivitas-CS", &uid)
	env.seedAllActivity(t, "note", "general", "Aktivitas-General", &uid)

	body := env.allActivitiesListBody(t, uid, "owner", "admin")

	for _, want := range []string{"Aktivitas-Sales", "Aktivitas-CS", "Aktivitas-General"} {
		if !strings.Contains(body, want) {
			t.Errorf("AllActivitiesList harus menampilkan %q (lintas-context)", want)
		}
	}
}

// TestAllActivitiesList_F3_ScopeFiltersByOwner: F3 ownership berlaku — IsOwn (sales)
// hanya lihat aktivitas milik sendiri; ScopeAll (admin) lihat semua.
func TestAllActivitiesList_F3_ScopeFiltersByOwner(t *testing.T) {
	env, actor := setupAccounts(t)
	ownerB := env.seedMember(t, "ownerb2@local", "member", 0).ID

	env.seedAllActivity(t, "note", "sales", "Milik-Sendiri", &actor)
	env.seedAllActivity(t, "note", "cs", "Milik-Orang-Lain", &ownerB)

	// IsOwn: hanya lihat milik sendiri.
	own := env.allActivitiesListBody(t, actor, "member", "sales")
	if !strings.Contains(own, "Milik-Sendiri") {
		t.Errorf("IsOwn harus melihat aktivitas sendiri")
	}
	if strings.Contains(own, "Milik-Orang-Lain") {
		t.Errorf("IsOwn tidak boleh melihat aktivitas owner lain")
	}

	// ScopeAll: lihat keduanya.
	all := env.allActivitiesListBody(t, actor, "owner", "admin")
	if !strings.Contains(all, "Milik-Sendiri") || !strings.Contains(all, "Milik-Orang-Lain") {
		t.Errorf("ScopeAll harus melihat semua aktivitas lintas-owner")
	}
}

// TestAllActivitiesList_SoftDeletedHidden: aktivitas ter-soft-delete tidak tampil
// (ListAllActivities WHERE deleted_at IS NULL).
func TestAllActivitiesList_SoftDeletedHidden(t *testing.T) {
	env, uid := setupAccounts(t)

	env.seedAllActivity(t, "note", "general", "Aktivitas-Hidup", &uid)
	terhapus := env.seedAllActivity(t, "note", "sales", "Aktivitas-Terhapus", &uid)

	if err := env.q.SoftDeleteActivity(t.Context(), db.SoftDeleteActivityParams{
		ID: terhapus.ID, UpdatedBy: &uid,
	}); err != nil {
		t.Fatalf("soft delete activity: %v", err)
	}

	body := env.allActivitiesListBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Aktivitas-Hidup") {
		t.Errorf("aktivitas hidup harus tampil")
	}
	if strings.Contains(body, "Aktivitas-Terhapus") {
		t.Errorf("aktivitas terhapus tidak boleh tampil")
	}
}

// TestAllActivitiesList_ForbiddenWithoutPerm: peran tanpa crm:sales_activity (csm)
// mendapat 403 — bukan halaman kosong yang senyap.
func TestAllActivitiesList_ForbiddenWithoutPerm(t *testing.T) {
	env, uid := setupAccounts(t)
	req := accountsReq(http.MethodGet, "/activity-log", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.AllActivitiesList)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tanpa crm:sales_activity harus 403, got %d", rec.Code)
	}
}

// TestAllActivitiesList_PlatformBypass: super_admin (platform role) tanpa
// business_role tetap bisa mengakses /activity-log dengan ScopeAll — platform
// operator butuh visibilitas sistem tanpa harus diberi peran CRM.
func TestAllActivitiesList_PlatformBypass(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAllActivity(t, "note", "sales", "Aktivitas-Super", &uid)

	req := accountsReq(http.MethodGet, "/activity-log", nil, "")
	rec := env.runAccount(uid, "super_admin", "" /* tanpa business_role */, req, env.h.AllActivitiesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("super_admin harus bisa akses /activity-log, got %d\n%s", rec.Code, rec.Body.String())
	}
	// super_admin dengan ScopeAll melihat semua aktivitas.
	if !strings.Contains(rec.Body.String(), "Aktivitas-Super") {
		t.Errorf("super_admin harus melihat semua aktivitas (ScopeAll bypass)")
	}
}
