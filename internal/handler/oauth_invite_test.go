package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/oauth"
	"go_starter/internal/session"
)

// oauth_invite_test.go — BL-54: undangan yang menunggu di session harus
// auto-accept saat penerima masuk lewat Google (jalur auth PRODUKSI). Sebelum
// fix, acceptPendingInvite hanya dipanggil di jalur password (dev-only) → di
// produksi undangan menggantung pending menuntut klik "Terima" manual.

// doCallbackWithInvite menjalankan GoogleCallback dengan token undangan tersimpan
// di session (seperti user yang membuka /invite/{token} sebelum login), lalu
// masuk via Google. Meniru doCallback + PutPendingInvite dalam satu session.
func (e *testEnv) doCallbackWithInvite(t *testing.T, email, token string) *httptest.ResponseRecorder {
	t.Helper()
	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{Sub: "g-" + email, Email: email, EmailVerified: true}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback/google?code=abc&state=s1", nil)
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.PutOAuthFlow(r.Context(), "s1", "the-nonce", "the-verifier")
		session.PutPendingInvite(r.Context(), token) // dibuka lewat tautan undangan
		e.h.GoogleCallback(w, r)
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

// TestGoogleCallback_AutoAcceptPendingInvite: penerima undangan yang masuk lewat
// Google langsung tergabung ke workspace pengundang — tanpa klik "Terima" manual.
func TestGoogleCallback_AutoAcceptPendingInvite(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	env.mkInvite(t, "tok-oauth", "undangan@gmail.com", "admin", time.Hour)

	rec := env.doCallbackWithInvite(t, "undangan@gmail.com", "tok-oauth")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login Google + auto-accept harus 303, got %d: %s", rec.Code, rec.Body.String())
	}
	// SetActiveTenant(pengundang) → redirect ke workspace pengundang, bukan
	// workspace baru milik penerima.
	if loc := rec.Header().Get("Location"); loc != "/w/test" {
		t.Errorf("auto-accept harus mengarahkan ke workspace pengundang /w/test, got %q", loc)
	}

	// Penerima jadi anggota tenant pengundang dengan role undangan.
	u, err := env.q.GetUserByEmail(ctx, "undangan@gmail.com")
	if err != nil {
		t.Fatalf("user Google tidak terbuat: %v", err)
	}
	m, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: env.tenantID})
	if err != nil {
		t.Fatalf("penerima harus jadi anggota workspace pengundang: %v", err)
	}
	if m.Role != "admin" {
		t.Errorf("role harus sesuai undangan (admin), got %q", m.Role)
	}
	// Undangan ditandai terpakai → tak lagi menggantung pending di Notifikasi.
	pending, err := env.q.ListPendingInvitesByEmail(ctx, "undangan@gmail.com")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("undangan harus terpakai (accepted_at terisi), masih %d pending", len(pending))
	}
}

// TestGoogleCallback_AutoAcceptFailSoft: undangan kedaluwarsa TIDAK menggagalkan
// login Google — penerima tetap masuk ke workspace-nya sendiri, tanpa membership
// di workspace pengundang.
func TestGoogleCallback_AutoAcceptFailSoft(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	env.mkInvite(t, "tok-exp", "gagal@gmail.com", "admin", -time.Hour) // sudah lewat

	rec := env.doCallbackWithInvite(t, "gagal@gmail.com", "tok-exp")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("undangan kedaluwarsa tak boleh menggagalkan login, got %d: %s", rec.Code, rec.Body.String())
	}
	// Penerima masuk ke workspace SENDIRI (slug dari local-part email), bukan pengundang.
	if loc := rec.Header().Get("Location"); loc != "/w/gagal" {
		t.Errorf("fail-soft: penerima tetap masuk workspace sendiri /w/gagal, got %q", loc)
	}
	// TIDAK ada membership di workspace pengundang.
	u, err := env.q.GetUserByEmail(ctx, "gagal@gmail.com")
	if err != nil {
		t.Fatalf("user Google tidak terbuat: %v", err)
	}
	if _, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: env.tenantID}); err == nil {
		t.Error("undangan kedaluwarsa TAK BOLEH membuat membership di workspace pengundang")
	}
}
