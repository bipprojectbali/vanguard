package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// dev_workspace_detail_test.go — menutup identitas & siklus hidup workspace yang
// PINDAH ke ruang developer (BL-53): ganti nama + arsip/unarsip/hapus sebagai
// aksi PLATFORM lintas-tenant BY ID. Menggantikan cakupan lama di
// workspace_test.go (TestWorkspaceUpdate_*) yang menguji handler owner-scoped
// yang kini dihapus. Invarian yang dijaga: rumah aplikasi (is_primary) tak bisa
// diarsip/dihapus; rename tak butuh keanggotaan operator; tiap aksi ter-audit.

// doDevWorkspace menjalankan handler dev workspace dalam session super-admin
// dengan chi param {id} + form values (pola sama doDevAction). h.q(ctx) di test
// = pool-bound (bypass RLS), jadi aksi beroperasi di workspace mana pun by-id
// tanpa keanggotaan — persis perilaku scope /dev (WithSuper) di produksi.
func (e *testEnv) doDevWorkspace(actorID, tenantID int64, form url.Values, fn http.HandlerFunc) *httptest.ResponseRecorder {
	base := httptest.NewRequest(http.MethodPost, "/dev/workspaces/x", strings.NewReader(form.Encode()))
	base.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	base = withChiParam(base, "id", itoa(tenantID))

	rec := httptest.NewRecorder()
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), actorID, "root@local", "super_admin", true, 1, "Acme", "acme", "")
		fn(w, r.WithContext(withQueries(r.Context(), e.q)))
	}))
	wrapped.ServeHTTP(rec, base)
	return rec
}

// makePrimary menandai satu tenant sebagai rumah aplikasi (is_primary). Lewat
// pool langsung: is_primary tak disetel CreateTenant, dan ini prasyarat test,
// bukan perilaku yang diuji.
func (e *testEnv) makePrimary(t *testing.T, tenantID int64) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		"UPDATE tenants SET is_primary = true WHERE id = $1", tenantID); err != nil {
		t.Fatalf("set is_primary: %v", err)
	}
}

func TestDevWorkspaceRename_Success(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doDevWorkspace(uid, env.tenantID, url.Values{"name": {"Acme Baru"}}, env.h.DevWorkspaceRename)

	tn, err := env.q.GetTenant(t.Context(), env.tenantID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if tn.Name != "Acme Baru" {
		t.Errorf("nama harus jadi 'Acme Baru', got %q", tn.Name)
	}
	// Slug immutable — tetap nilai seed.
	if tn.Slug != "test" {
		t.Errorf("slug harus tetap immutable, got %q", tn.Slug)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("harus redirect 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Errorf("sukses tak boleh ber-err, got Location=%q", loc)
	}
	// Audit tercatat sebagai workspace.rename (target = workspace).
	logs, _ := env.q.ListAuditLogs(t.Context(), 10)
	var found bool
	for _, l := range logs {
		if l.Action == "workspace.rename" {
			found = true
		}
	}
	if !found {
		t.Error("rename harus tercatat sebagai workspace.rename di audit_logs")
	}
}

func TestDevWorkspaceRename_EmptyName(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doDevWorkspace(uid, env.tenantID, url.Values{"name": {"   "}}, env.h.DevWorkspaceRename)

	tn, _ := env.q.GetTenant(t.Context(), env.tenantID)
	if tn.Name != "Test" {
		t.Errorf("nama kosong tak boleh tersimpan, got %q", tn.Name)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=name") {
		t.Errorf("nama kosong harus redirect ?err=name, got Location=%q", loc)
	}
}

// TestDevWorkspaceRename_CrossTenant: operator platform mengganti nama workspace
// yang BUKAN miliknya (tanpa keanggotaan). Ini inti pemindahan BL-53 — dulu
// hanya owner workspace itu yang bisa; kini platform bisa by-id.
func TestDevWorkspaceRename_CrossTenant(t *testing.T) {
	env, uid := setupTest(t)
	asing, err := env.q.CreateTenant(t.Context(), db.CreateTenantParams{Name: "Asing", Slug: "asing"})
	if err != nil {
		t.Fatalf("seed tenant asing: %v", err)
	}
	env.seedMember(t, "orang2@local", "owner", asing.ID)

	// uid (operator) BUKAN anggota "asing", namun boleh mengganti namanya.
	rec := env.doDevWorkspace(uid, asing.ID, url.Values{"name": {"Asing Baru"}}, env.h.DevWorkspaceRename)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("harus redirect 303, got %d", rec.Code)
	}
	tn, _ := env.q.GetTenant(t.Context(), asing.ID)
	if tn.Name != "Asing Baru" {
		t.Errorf("platform harus bisa rename workspace lain, got %q", tn.Name)
	}
}

func TestDevWorkspaceArchive_Success(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doDevWorkspace(uid, env.tenantID, nil, env.h.DevWorkspaceArchive)

	tn, _ := env.q.GetTenant(t.Context(), env.tenantID)
	if tn.Status != "archived" {
		t.Errorf("status harus archived, got %q", tn.Status)
	}
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Errorf("arsip non-primer harus sukses, got Location=%q", loc)
	}
}

// TestDevWorkspaceArchive_PrimaryDenied: rumah aplikasi tak bisa diarsip —
// dijaga di handler (?err=primary) DAN SQL (AND NOT is_primary).
func TestDevWorkspaceArchive_PrimaryDenied(t *testing.T) {
	env, uid := setupTest(t)
	env.makePrimary(t, env.tenantID)

	rec := env.doDevWorkspace(uid, env.tenantID, nil, env.h.DevWorkspaceArchive)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=primary") {
		t.Errorf("arsip rumah aplikasi harus ?err=primary, got Location=%q", loc)
	}
	tn, _ := env.q.GetTenant(t.Context(), env.tenantID)
	if tn.Status != "active" {
		t.Errorf("rumah aplikasi tak boleh terarsip, got status %q", tn.Status)
	}
}

func TestDevWorkspaceUnarchive_Success(t *testing.T) {
	env, uid := setupTest(t)
	if err := env.q.ArchiveTenant(t.Context(), env.tenantID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	env.doDevWorkspace(uid, env.tenantID, nil, env.h.DevWorkspaceUnarchive)

	tn, _ := env.q.GetTenant(t.Context(), env.tenantID)
	if tn.Status != "active" {
		t.Errorf("unarchive harus kembalikan ke active, got %q", tn.Status)
	}
}

func TestDevWorkspaceDelete_Success(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doDevWorkspace(uid, env.tenantID, nil, env.h.DevWorkspaceDelete)

	tn, _ := env.q.GetTenant(t.Context(), env.tenantID)
	if !tn.DeletedAt.Valid {
		t.Error("soft-delete harus mengisi deleted_at")
	}
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Errorf("hapus non-primer harus sukses, got Location=%q", loc)
	}
}

// TestDevWorkspaceDelete_PrimaryDenied: rumah aplikasi tak bisa dihapus.
func TestDevWorkspaceDelete_PrimaryDenied(t *testing.T) {
	env, uid := setupTest(t)
	env.makePrimary(t, env.tenantID)

	rec := env.doDevWorkspace(uid, env.tenantID, nil, env.h.DevWorkspaceDelete)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=primary") {
		t.Errorf("hapus rumah aplikasi harus ?err=primary, got Location=%q", loc)
	}
	tn, _ := env.q.GetTenant(t.Context(), env.tenantID)
	if tn.DeletedAt.Valid {
		t.Error("rumah aplikasi tak boleh terhapus")
	}
}

// TestDevWorkspaceDetail_NotFound: id tak dikenal → 404, bukan 500.
func TestDevWorkspaceDetail_NotFound(t *testing.T) {
	env, uid := setupTest(t)
	req := withChiParam(httptest.NewRequest(http.MethodGet, "/dev/workspaces/999999", nil), "id", "999999")
	rec := env.doAuthed(uid, req, env.h.DevWorkspaceDetail)
	if rec.Code != http.StatusNotFound {
		t.Errorf("id tak ada harus 404, got %d", rec.Code)
	}
}
