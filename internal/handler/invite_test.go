package handler

import (
	"errors"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// invite_test.go — alur undangan: buat, terima, dan penolakan token yang tak
// sah (kedaluwarsa / sudah dipakai / tak ada).

// mkInvite membuat undangan langsung di DB (melewati handler) agar test fokus
// pada logika penerimaan. ttl negatif → sudah kedaluwarsa.
func (e *testEnv) mkInvite(t *testing.T, token, email, role string, ttl time.Duration) db.Invite {
	t.Helper()
	inv, err := e.q.CreateInvite(t.Context(), db.CreateInviteParams{
		TenantID:  e.tenantID,
		Email:     email,
		Role:      role,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(ttl), Valid: true},
		Kind:      "internal",
	})
	if err != nil {
		t.Fatalf("seed invite: %v", err)
	}
	return inv
}

// TestInvite_AcceptMenjadikanAnggota: token sah → user jadi anggota workspace
// pengundang dengan role yang diundang, dan undangan ditandai terpakai.
func TestInvite_AcceptMenjadikanAnggota(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	env.mkInvite(t, "tok-ok", "tamu@local", "admin", time.Hour)

	// User penerima (belum jadi anggota workspace mana pun di tenant ini).
	tamu, err := env.q.CreateUser(ctx, db.CreateUserParams{Email: "tamu@local", PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("seed tamu: %v", err)
	}

	env.withSession(t, tamu.ID, func(sctx sessionCtx) {
		if err := env.h.acceptInvite(sctx.ctx, "tok-ok", tamu.ID); err != nil {
			t.Fatalf("acceptInvite: %v", err)
		}
	})

	m, err := env.q.GetMembership(ctx, db.GetMembershipParams{
		UserID: tamu.ID, TenantID: env.tenantID,
	})
	if err != nil {
		t.Fatalf("penerima harus jadi anggota: %v", err)
	}
	if m.Role != "admin" {
		t.Errorf("role harus sesuai undangan (admin), got %q", m.Role)
	}
	// Undangan ditandai terpakai → tak bisa dipakai lagi.
	env.withSession(t, tamu.ID, func(sctx sessionCtx) {
		if err := env.h.acceptInvite(sctx.ctx, "tok-ok", tamu.ID); !errors.Is(err, errInviteUsed) {
			t.Errorf("undangan sekali pakai: harus errInviteUsed, got %v", err)
		}
	})
}

// TestInvite_TolakKedaluwarsa: token lewat masa berlaku → ditolak, tak ada
// membership terbuat.
func TestInvite_TolakKedaluwarsa(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	env.mkInvite(t, "tok-exp", "lama@local", "member", -time.Hour) // sudah lewat

	u, _ := env.q.CreateUser(ctx, db.CreateUserParams{Email: "lama@local", PassHash: ptr("x")})
	env.withSession(t, u.ID, func(sctx sessionCtx) {
		if err := env.h.acceptInvite(sctx.ctx, "tok-exp", u.ID); !errors.Is(err, errInviteExpired) {
			t.Errorf("undangan kedaluwarsa harus ditolak, got %v", err)
		}
	})
	if _, err := env.q.GetMembership(ctx, db.GetMembershipParams{
		UserID: u.ID, TenantID: env.tenantID,
	}); err == nil {
		t.Error("undangan kedaluwarsa TAK BOLEH membuat membership")
	}
}

// TestInvite_TolakTokenTakDikenal: token acak → errInviteNotFound.
func TestInvite_TolakTokenTakDikenal(t *testing.T) {
	env, uid := setupTest(t)
	env.withSession(t, uid, func(sctx sessionCtx) {
		if err := env.h.acceptInvite(sctx.ctx, "tak-ada", uid); !errors.Is(err, errInviteNotFound) {
			t.Errorf("token tak dikenal harus errInviteNotFound, got %v", err)
		}
	})
}

// TestInvite_ListHanyaPending: undangan yang sudah diterima tak muncul di daftar
// pending (panel anggota).
func TestInvite_ListHanyaPending(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	env.mkInvite(t, "tok-a", "a@local", "member", time.Hour)
	env.mkInvite(t, "tok-b", "b@local", "member", time.Hour)

	if err := env.q.AcceptInvite(ctx, "tok-a"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	pending, err := env.q.ListInvitesByTenant(ctx, env.tenantID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pending) != 1 || pending[0].Token != "tok-b" {
		t.Errorf("hanya undangan pending yang tampil, got %d", len(pending))
	}
}
