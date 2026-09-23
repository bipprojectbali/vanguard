package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// invite_accept_email_test.go — acceptInvitesByEmail (BL-170): jalur UTAMA
// auto-join saat login/register/OAuth. Menguji langsung fungsi unexported
// (bukan lewat HTTP penuh) karena tiga titik panggil (auth.go, auth_login.go,
// oauth_google.go) hanya MEMANGGIL fungsi ini — logikanya sendiri layak diuji
// terisolasi. Jalur token lama (acceptInvite) diuji di invite_test.go.

// mkInviteTenant seed undangan langsung di DB utk tenant + business_role +
// kind spesifik (mkInvite di invite_test.go hardcode tenant env & kind=internal).
func (e *testEnv) mkInviteTenant(t *testing.T, tenantID int64, token, email, role string, businessRole *string, kind string) db.Invite {
	t.Helper()
	inv, err := e.q.CreateInvite(t.Context(), db.CreateInviteParams{
		TenantID: tenantID, Email: email, Role: role, Token: token,
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		BusinessRole: businessRole, Kind: kind,
	})
	if err != nil {
		t.Fatalf("seed invite tenant %d: %v", tenantID, err)
	}
	return inv
}

// TestAcceptInvitesByEmail_MultiTenant: dua undangan pending di DUA tenant
// berbeda utk email yang sama → SEMUA diterima dalam satu panggilan, masing-
// masing dgn role/business_role/kind sendiri, dan ditandai accepted.
func TestAcceptInvitesByEmail_MultiTenant(t *testing.T) {
	env, _ := setupRoles(t)
	ctx := t.Context()

	tenant2, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Kedua", Slug: "kedua"})
	if err != nil {
		t.Fatalf("seed tenant kedua: %v", err)
	}
	sales := "sales"
	env.mkInviteTenant(t, env.tenantID, "tok-a", "ganda@local", "member", &sales, "internal")
	env.mkInviteTenant(t, tenant2.ID, "tok-b", "ganda@local", "admin", nil, "internal")

	u := env.seedUserOnly(t, "ganda@local")
	env.withSession(t, u.ID, func(sctx sessionCtx) {
		env.h.acceptInvitesByEmail(sctx.ctx, "ganda@local", u.ID)
	})

	m1, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: env.tenantID})
	if err != nil {
		t.Fatalf("harus jadi anggota tenant 1: %v", err)
	}
	if m1.Role != "member" || m1.BusinessRole == nil || *m1.BusinessRole != "sales" {
		t.Errorf("tenant 1: role/business_role salah, got role=%q br=%v", m1.Role, m1.BusinessRole)
	}

	m2, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: tenant2.ID})
	if err != nil {
		t.Fatalf("harus jadi anggota tenant 2: %v", err)
	}
	if m2.Role != "admin" {
		t.Errorf("tenant 2: role salah, got %q", m2.Role)
	}

	pending, err := env.q.ListPendingInvitesByEmail(ctx, "ganda@local")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("kedua undangan harus sudah accepted, masih pending %d", len(pending))
	}
}

// TestAcceptInvitesByEmail_ConflictUpgrade: user SUDAH anggota (role member)
// sebelum undangan diproses (mis. join organik) → undangan role lebih tinggi
// (admin) menaikkan role existing, business_role+kind tetap diterapkan.
func TestAcceptInvitesByEmail_ConflictUpgrade(t *testing.T) {
	env, _ := setupRoles(t)
	ctx := t.Context()
	u := env.seedMember(t, "naik@local", "member", 0)

	manager := "manager"
	env.mkInviteTenant(t, env.tenantID, "tok-up", "naik@local", "admin", &manager, "internal")
	env.withSession(t, u.ID, func(sctx sessionCtx) {
		env.h.acceptInvitesByEmail(sctx.ctx, "naik@local", u.ID)
	})

	m, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: env.tenantID})
	if err != nil {
		t.Fatalf("membership: %v", err)
	}
	if m.Role != "admin" {
		t.Errorf("role harus naik jadi admin, got %q", m.Role)
	}
	if m.BusinessRole == nil || *m.BusinessRole != "manager" {
		t.Errorf("business_role harus diterapkan dari undangan, got %v", m.BusinessRole)
	}
}

// TestAcceptInvitesByEmail_NoDowngrade: user SUDAH admin sebelum undangan
// role member diproses → role TAK diturunkan, tapi business_role+kind dari
// undangan tetap diterapkan (bukan silent no-op total).
func TestAcceptInvitesByEmail_NoDowngrade(t *testing.T) {
	env, _ := setupRoles(t)
	ctx := t.Context()
	u := env.seedMember(t, "turun@local", "admin", 0)

	sales := "sales"
	env.mkInviteTenant(t, env.tenantID, "tok-down", "turun@local", "member", &sales, "internal")
	env.withSession(t, u.ID, func(sctx sessionCtx) {
		env.h.acceptInvitesByEmail(sctx.ctx, "turun@local", u.ID)
	})

	m, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: env.tenantID})
	if err != nil {
		t.Fatalf("membership: %v", err)
	}
	if m.Role != "admin" {
		t.Errorf("role admin TAK BOLEH turun jadi member, got %q", m.Role)
	}
	if m.BusinessRole == nil || *m.BusinessRole != "sales" {
		t.Errorf("business_role tetap harus diterapkan meski role tak berubah, got %v", m.BusinessRole)
	}
}

// TestAcceptInvitesByEmail_TanpaUndanganNoOp: tak ada undangan pending utk
// email → tak error, tak membuat membership apa pun.
func TestAcceptInvitesByEmail_TanpaUndanganNoOp(t *testing.T) {
	env, _ := setupRoles(t)
	ctx := t.Context()
	u := env.seedUserOnly(t, "sepi@local")

	env.h.acceptInvitesByEmail(ctx, "sepi@local", u.ID)

	if _, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: env.tenantID}); err == nil {
		t.Error("tanpa undangan pending tak boleh membuat membership")
	}
}
