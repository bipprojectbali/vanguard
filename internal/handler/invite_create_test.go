package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"

	"github.com/go-chi/chi/v5"
)

// invite_create_test.go — InviteCreate (BL-170): guard anti re-undang anggota
// existing, upsert-by-replace undangan pending, dan validasi cascading
// kind/business_role. Jalur AKSEPTASI (acceptInvitesByEmail) diuji terpisah di
// invite_accept_email_test.go.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// diuji terpisah di rls_test.go. Peran CRM bawaan ditanam via setupRoles
// (sales=internal tersedia bawaan; peran eksternal ditanam manual per test).

// inviteCreateReq membangun POST /w/test/members/invite dengan chi param
// workspace + body form terkode.
func inviteCreateReq(form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/w/test/members/invite",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workspace", "test")
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// pendingInvite mengembalikan undangan pending tenant test untuk email
// tertentu, atau nil bila tak ada.
func (e *testEnv) pendingInvite(t *testing.T, email string) *db.Invite {
	t.Helper()
	rows, err := e.q.ListInvitesByTenant(t.Context(), e.tenantID)
	if err != nil {
		t.Fatalf("list invites: %v", err)
	}
	for _, inv := range rows {
		if inv.Email == email {
			cp := inv
			return &cp
		}
	}
	return nil
}

// TestInviteCreate_Success: owner mengundang email baru dengan Peran CRM +
// Jenis Anggota → 303 ok, undangan tersimpan (role tenant SELALU "member",
// business_role & kind sesuai form), audit TANPA PII (email tak tercatat).
func TestInviteCreate_Success(t *testing.T) {
	env, uid := setupRoles(t)

	form := url.Values{"email": {"calon@contoh.com"}, "kind": {"internal"}, "business_role": {"sales"}}
	rec := env.runAccount(uid, "owner", "", inviteCreateReq(form), env.h.InviteCreate)

	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("harus sukses, got %q (status %d)", loc, rec.Code)
	}
	inv := env.pendingInvite(t, "calon@contoh.com")
	if inv == nil {
		t.Fatal("undangan harus tersimpan")
	}
	if inv.Role != "member" {
		t.Errorf("role tenant undangan harus selalu member, got %q", inv.Role)
	}
	if inv.Kind != "internal" {
		t.Errorf("kind = %q, want internal", inv.Kind)
	}
	if inv.BusinessRole == nil || *inv.BusinessRole != "sales" {
		t.Errorf("business_role = %v, want sales", inv.BusinessRole)
	}
	env.assertAudited(t, "invite.create")
}

// TestInviteCreate_RejectExistingMember: email yang sudah jadi anggota tenant
// ini ditolak err=invite_member — tak menyimpan undangan (guard b).
func TestInviteCreate_RejectExistingMember(t *testing.T) {
	env, uid := setupRoles(t)
	seedMember(t, env, "sudah@contoh.com", "member")

	form := url.Values{"email": {"sudah@contoh.com"}, "kind": {"internal"}}
	rec := env.runAccount(uid, "owner", "", inviteCreateReq(form), env.h.InviteCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=invite_member") {
		t.Errorf("harus err=invite_member, got %q (status %d)", loc, rec.Code)
	}
	if env.pendingInvite(t, "sudah@contoh.com") != nil {
		t.Error("anggota existing tak boleh dapat undangan baru")
	}
}

// TestInviteCreate_UpsertReplace: re-undang email yang masih punya undangan
// pending MENGGANTI (bukan menumpuk) — field terbaru yang tersimpan.
func TestInviteCreate_UpsertReplace(t *testing.T) {
	env, uid := setupRoles(t)

	form1 := url.Values{"email": {"ulang@contoh.com"}, "kind": {"internal"}, "business_role": {"sales"}}
	env.runAccount(uid, "owner", "", inviteCreateReq(form1), env.h.InviteCreate)
	first := env.pendingInvite(t, "ulang@contoh.com")
	if first == nil {
		t.Fatal("undangan pertama harus tersimpan")
	}

	form2 := url.Values{"email": {"ulang@contoh.com"}, "kind": {"internal"}, "business_role": {"manager"}}
	env.runAccount(uid, "owner", "", inviteCreateReq(form2), env.h.InviteCreate)

	rows, err := env.q.ListInvitesByTenant(t.Context(), env.tenantID)
	if err != nil {
		t.Fatalf("list invites: %v", err)
	}
	n := 0
	var latest *db.Invite
	for _, inv := range rows {
		if inv.Email == "ulang@contoh.com" {
			n++
			cp := inv
			latest = &cp
		}
	}
	if n != 1 {
		t.Fatalf("harus tepat SATU undangan pending utk email ini, got %d", n)
	}
	if latest.BusinessRole == nil || *latest.BusinessRole != "manager" {
		t.Errorf("undangan harus terganti field terbaru (manager), got %v", latest.BusinessRole)
	}
	if latest.Token == first.Token {
		t.Error("undangan pengganti harus token baru (bukan baris lama yang diedit)")
	}
}

// TestInviteCreate_KindRequired: kind kosong/tak dikenal ditolak err=kind,
// tak menyimpan undangan.
func TestInviteCreate_KindRequired(t *testing.T) {
	cases := []string{"", "bogus"}
	for _, k := range cases {
		t.Run("kind="+k, func(t *testing.T) {
			env, uid := setupRoles(t)
			form := url.Values{"email": {"x@contoh.com"}, "kind": {k}}
			rec := env.runAccount(uid, "owner", "", inviteCreateReq(form), env.h.InviteCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=kind") {
				t.Errorf("harus err=kind, got %q (status %d)", loc, rec.Code)
			}
			if env.pendingInvite(t, "x@contoh.com") != nil {
				t.Error("kind tak sah tak boleh menyimpan undangan")
			}
		})
	}
}

// TestInviteCreate_BusinessRoleMismatchKind: business_role yang kind-nya tak
// cocok dengan kind form ditolak err=crm_role — pertahanan berlapis di atas
// cascading select (form bisa dimanipulasi langsung).
func TestInviteCreate_BusinessRoleMismatchKind(t *testing.T) {
	env, uid := setupRoles(t)
	// sales = bawaan, kind internal. Pilih kind=external tapi business_role=sales.
	form := url.Values{"email": {"x@contoh.com"}, "kind": {"external"}, "business_role": {"sales"}}
	rec := env.runAccount(uid, "owner", "", inviteCreateReq(form), env.h.InviteCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=crm_role") {
		t.Errorf("harus err=crm_role, got %q (status %d)", loc, rec.Code)
	}
	if env.pendingInvite(t, "x@contoh.com") != nil {
		t.Error("business_role tak cocok kind tak boleh menyimpan undangan")
	}
}

// TestInviteCreate_BusinessRoleUnknown: nama peran CRM asing (tak ada di
// tenant) ditolak err=crm_role.
func TestInviteCreate_BusinessRoleUnknown(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"email": {"x@contoh.com"}, "kind": {"internal"}, "business_role": {"ghost"}}
	rec := env.runAccount(uid, "owner", "", inviteCreateReq(form), env.h.InviteCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=crm_role") {
		t.Errorf("harus err=crm_role, got %q (status %d)", loc, rec.Code)
	}
}

// TestInviteCreate_Forbidden: anggota biasa tak boleh mengundang.
func TestInviteCreate_Forbidden(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"email": {"x@contoh.com"}, "kind": {"internal"}}
	rec := env.runAccount(uid, "member", "", inviteCreateReq(form), env.h.InviteCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("anggota biasa harus err=forbidden, got %q", loc)
	}
	if env.pendingInvite(t, "x@contoh.com") != nil {
		t.Error("tak berwenang tak boleh menyimpan undangan")
	}
}
