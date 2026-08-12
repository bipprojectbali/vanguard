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

// member_business_role_test.go — sumbu CRM (business_role) di POST
// /w/{slug}/members/{id}/role. Yang dijaga:
//
//   - Penugasan peran CRM oleh pengelola (gerbang canManageMembers sumbu TENANT,
//     BUKAN CanBusiness) → workspace baru yang owner-nya belum punya peran CRM
//     tetap bisa membuka pintu CRM. Anggota biasa ditolak (forbidden).
//   - Opt-in diri sendiri: owner boleh menugaskan business_role ke BARIS SENDIRI
//     (sumbu tenant dilewati untuk diri sendiri, sumbu CRM tidak).
//   - Nama peran dari form divalidasi tenant-aware (GetBusinessRole) — peran asing
//     ditolak err=crm_role, tak menyentuh DB.
//   - Cabut peran (business_role → NULL).
//   - Guard B LUNAK: mencabut Admin CRM terakhir tetap DISIMPAN, tapi memantul
//     dengan peringatan ok=crm_lastadmin (bukan blokir).
//   - Dua sumbu dalam satu POST: role tenant + peran CRM sama-sama diterapkan.
//   - Efek samping: audit member.business_role.changed + notifikasi ke anggota.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji
// terpisah di rls_test.go. Peran CRM bawaan ditanam via setupRoles.

// memberRoleReq membangun POST /w/test/members/{id}/role dengan chi param
// workspace + id dan body form terkode.
func memberRoleReq(id string, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/w/test/members/"+id+"/role",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workspace", "test")
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// bizRole membaca business_role satu anggota ("" bila NULL/tak ada).
func (e *testEnv) bizRole(t *testing.T, userID int64) string {
	t.Helper()
	m, err := e.q.GetMembership(t.Context(), db.GetMembershipParams{
		UserID: userID, TenantID: e.tenantID,
	})
	if err != nil {
		t.Fatalf("get membership %d: %v", userID, err)
	}
	if m.BusinessRole == nil {
		return ""
	}
	return *m.BusinessRole
}

// assignBiz menyetel business_role langsung lewat pool (prasyarat, bypass handler).
func (e *testEnv) assignBiz(t *testing.T, userID int64, role string) {
	t.Helper()
	if err := e.q.UpdateMemberBusinessRole(t.Context(), db.UpdateMemberBusinessRoleParams{
		UserID: userID, TenantID: e.tenantID, BusinessRole: &role,
	}); err != nil {
		t.Fatalf("assign business_role %q ke %d: %v", role, userID, err)
	}
}

// hasNotif = true bila user punya notifikasi dengan kind tertentu.
func (e *testEnv) hasNotif(t *testing.T, userID int64, kind string) bool {
	t.Helper()
	for _, n := range e.firstPage(t, userID) {
		if n.Kind == kind {
			return true
		}
	}
	return false
}

const bizChanged = "member.business_role.changed"

// TestMemberBiz_AssignByManager: owner (pengelola) menugaskan peran CRM "sales" ke
// anggota → ok=crm_assigned, business_role tersimpan, audit + notifikasi tercatat.
func TestMemberBiz_AssignByManager(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"business_role": {"sales"}}
	rec := env.runAccount(uid, "owner", "", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_assigned") {
		t.Fatalf("harus ok=crm_assigned, got %q (status %d)", loc, rec.Code)
	}
	if got := env.bizRole(t, member); got != "sales" {
		t.Errorf("business_role = %q, want sales", got)
	}
	env.assertAudited(t, bizChanged)
	if !env.hasNotif(t, member, bizChanged) {
		t.Error("anggota harus diberi tahu perubahan peran CRM")
	}
}

// TestMemberBiz_SelfOptIn: owner menugaskan peran CRM ke BARIS SENDIRI (opt-in).
// Sumbu tenant dilewati untuk diri sendiri; sumbu CRM tetap berlaku.
func TestMemberBiz_SelfOptIn(t *testing.T) {
	env, uid := setupRoles(t)

	form := url.Values{"business_role": {"admin"}}
	rec := env.runAccount(uid, "owner", "", memberRoleReq(itoa(uid), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_assigned") {
		t.Fatalf("self opt-in harus ok=crm_assigned, got %q (status %d)", loc, rec.Code)
	}
	if got := env.bizRole(t, uid); got != "admin" {
		t.Errorf("business_role diri sendiri = %q, want admin", got)
	}
}

// TestMemberBiz_Forbidden: anggota biasa (tenant role member) ditolak err=forbidden
// dan tak menyentuh business_role siapa pun.
func TestMemberBiz_Forbidden(t *testing.T) {
	env, uid := setupRoles(t)
	target := env.seedMember(t, "t@local", "member", 0).ID

	form := url.Values{"business_role": {"sales"}}
	rec := env.runAccount(uid, "member", "", memberRoleReq(itoa(target), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("anggota biasa harus err=forbidden, got %q", loc)
	}
	if got := env.bizRole(t, target); got != "" {
		t.Errorf("tak berwenang tak boleh menyimpan peran, got %q", got)
	}
}

// TestMemberBiz_UnknownRole: nama peran yang tak ada di workspace ditolak
// err=crm_role (validasi tenant-aware di atas RLS) — tak menyentuh DB.
func TestMemberBiz_UnknownRole(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"business_role": {"ghost"}}
	rec := env.runAccount(uid, "owner", "", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=crm_role") {
		t.Errorf("peran asing harus err=crm_role, got %q", loc)
	}
	if got := env.bizRole(t, member); got != "" {
		t.Errorf("peran asing tak boleh tersimpan, got %q", got)
	}
}

// TestMemberBiz_Unassign: mencabut peran (business_role "") → NULL, ok=crm_assigned.
func TestMemberBiz_Unassign(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID
	env.assignBiz(t, member, "sales")

	form := url.Values{"business_role": {""}}
	rec := env.runAccount(uid, "owner", "", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_assigned") {
		t.Fatalf("cabut peran harus ok=crm_assigned, got %q (status %d)", loc, rec.Code)
	}
	if got := env.bizRole(t, member); got != "" {
		t.Errorf("business_role harus NULL setelah dicabut, got %q", got)
	}
}

// TestMemberBiz_LastAdminSoftWarn: mencabut Admin CRM TERAKHIR tetap disimpan
// (peringatan LUNAK), memantul ok=crm_lastadmin — bukan blokir.
func TestMemberBiz_LastAdminSoftWarn(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "adm@local", "member", 0).ID
	env.assignBiz(t, member, "admin") // satu-satunya pemegang admin CRM

	form := url.Values{"business_role": {"manager"}}
	rec := env.runAccount(uid, "owner", "", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_lastadmin") {
		t.Fatalf("admin CRM terakhir harus ok=crm_lastadmin, got %q (status %d)", loc, rec.Code)
	}
	// Perubahan TETAP diterapkan (peringatan, bukan blokir).
	if got := env.bizRole(t, member); got != "manager" {
		t.Errorf("peringatan lunak tetap menyimpan; business_role = %q, want manager", got)
	}
}

// TestMemberBiz_BothAxes: satu POST membawa role TENANT + peran CRM → keduanya
// diterapkan pada anggota yang sama.
func TestMemberBiz_BothAxes(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"role": {"admin"}, "business_role": {"sales"}}
	rec := env.runAccount(uid, "owner", "", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_assigned") {
		t.Fatalf("dua sumbu harus ok=crm_assigned, got %q (status %d)", loc, rec.Code)
	}
	m, err := env.q.GetMembership(t.Context(), db.GetMembershipParams{
		UserID: member, TenantID: env.tenantID,
	})
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if m.Role != "admin" {
		t.Errorf("sumbu tenant: role = %q, want admin", m.Role)
	}
	if m.BusinessRole == nil || *m.BusinessRole != "sales" {
		t.Errorf("sumbu CRM: business_role = %v, want sales", m.BusinessRole)
	}
}
