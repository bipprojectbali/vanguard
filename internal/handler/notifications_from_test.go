package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// notifications_from_test.go — BL-103: /notifications adalah rute GLOBAL (lintas-
// workspace), jadi shell-nya diturunkan dari ROLE. Untuk akun PLATFORM
// (super_admin env) itu berarti panel dev — sekalipun lonceng ditekan dari dalam
// sebuah workspace. Perbaikan: lonceng membawa ?from={slug}; bila valid, shell
// mengikuti workspace itu, bukan dev. Yang diuji di sini adalah PEMBUNGKUS shell
// (nav/brand), bukan isi umpan (itu di notifications_page_test.go).
//
// Penanda pembeda dipilih yang TAK ambigu: nav dev punya "/dev/logs" &
// "/dev/workspaces" (tak pernah muncul di quicklinks — yang hanya /dev/users),
// nav workspace punya grup "Subscriptions" (tak ada di dev, changelog, atau
// chrome shell lain).

const (
	devNavMarker       = "/dev/logs"      // hanya devNav
	workspaceNavMarker = "Subscriptions"  // grup nav workspace (selalu dirender)
)

// initAuthz memasang engine Casbin global — dibutuhkan navFor/workspaceNavCtx yang
// memanggil authz.Can. Aman dipanggil berulang (set global).
func initAuthz(t *testing.T) {
	t.Helper()
	e, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		t.Fatalf("authz.New: %v", err)
	}
	authz.Init(e)
}

// getNotifPagePlatform membuka /notifications sebagai akun PLATFORM (super_admin
// env, isRoot). Meniru getNotifPage tapi dengan identitas platform.
func getNotifPagePlatform(t *testing.T, env *testEnv, uid int64, query string) string {
	t.Helper()
	u := "/notifications"
	if query != "" {
		u += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, u, nil)
	rec := env.doAuthed(uid, req, func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "iam.amaliadwi@gmail.com",
			authz.RoleNameSuperAdmin, true, env.tenantID, "Test", "test", "")
		env.h.NotificationsPage(w, r)
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("halaman notifikasi status %d", rec.Code)
	}
	return rec.Body.String()
}

// TestNotifPage_PlatformFromWorkspace_ShellWorkspace: INTI perbaikan. Akun
// platform yang menekan lonceng dari dalam workspace (?from={slug}) harus melihat
// shell WORKSPACE itu — bukan panel dev.
func TestNotifPage_PlatformFromWorkspace_ShellWorkspace(t *testing.T) {
	env, uid := setupTest(t)
	initAuthz(t)

	body := getNotifPagePlatform(t, env, uid, "from=test")

	if !strings.Contains(body, workspaceNavMarker) {
		t.Error("BL-103: dengan ?from valid, akun platform harus dapat nav workspace (grup Subscriptions)")
	}
	if strings.Contains(body, devNavMarker) {
		t.Error("BL-103: dengan ?from valid, shell tak boleh lagi nav dev (/dev/logs)")
	}
}

// TestNotifPage_PlatformTanpaFrom_ShellDev: mengunci perilaku default. Tanpa
// ?from, /notifications tetap shell dev untuk akun platform (tak ada regresi bagi
// yang membuka lonceng dari luar workspace, mis. /dev).
func TestNotifPage_PlatformTanpaFrom_ShellDev(t *testing.T) {
	env, uid := setupTest(t)
	initAuthz(t)

	body := getNotifPagePlatform(t, env, uid, "")

	if !strings.Contains(body, devNavMarker) {
		t.Error("BL-103: tanpa ?from, akun platform harus tetap dapat nav dev (/dev/logs)")
	}
	if strings.Contains(body, workspaceNavMarker) {
		t.Error("BL-103: tanpa ?from, shell tak boleh berubah jadi nav workspace")
	}
}

// TestNotifPage_FromAsingTakBocor: anggota biasa memberi ?from milik workspace
// yang BUKAN miliknya. resolveTenantBySlug fail-closed → nama workspace asing tak
// boleh bocor, shell jatuh ke perilaku lama (workspace aktifnya sendiri).
func TestNotifPage_FromAsingTakBocor(t *testing.T) {
	env, uid := setupTest(t)
	initAuthz(t)

	// Workspace lain yang uid TIDAK ikuti. Nama sengaja khas agar mudah dideteksi
	// bila bocor; slug beda dari "test".
	asing, err := env.q.CreateTenant(t.Context(),
		db.CreateTenantParams{Name: "Workspace Rahasia Asing", Slug: "asing"})
	if err != nil {
		t.Fatalf("seed tenant asing: %v", err)
	}

	body := getNotifPage(t, env, uid, "from="+asing.Slug)

	if strings.Contains(body, asing.Name) {
		t.Errorf("BL-103: nama workspace asing %q bocor lewat ?from tak tervalidasi", asing.Name)
	}
}

// TestNotifPage_PagerPertahankanFrom: navigasi di dalam halaman notifikasi tak
// boleh membuang ?from — kalau hilang, halaman kedua memantul balik ke shell dev.
func TestNotifPage_PagerPertahankanFrom(t *testing.T) {
	env, uid := setupTest(t)
	initAuthz(t)

	now := time.Now()
	for i := range notifPageSize + 1 {
		seedNotifAt(t, env, uid, "member.role.changed", now.Add(-time.Duration(i)*time.Minute))
	}

	body := getNotifPagePlatform(t, env, uid, "from=test")
	// href di-render gomponents meng-escape "&" → "&amp;"; terima kedua bentuk.
	if !strings.Contains(body, "from=test&amp;after=") && !strings.Contains(body, "from=test&after=") {
		t.Error("BL-103: tautan halaman berikutnya harus mempertahankan ?from=test")
	}
}
