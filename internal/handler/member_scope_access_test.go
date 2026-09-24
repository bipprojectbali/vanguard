package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/db"
)

// member_scope_access_test.go — dipisah dari member_scope_test.go (BL-171)
// agar tiap file di bawah ambang tipe Test (400). Isinya dua section yang
// mengetes JALUR BACA/REASSIGN sumbu bisnis: MembersPage (halaman) &
// MemberSetKind (reassignment kind lintas-cakupan). Helper bersama
// (memberScopePolicy, inviteDeleteReq, seedInviteKind) tetap di
// member_scope_test.go. Lihat header file itu untuk konteks BL-171 lengkap.

// --- MembersPage: akses sumbu bisnis -----------------------------------------

// TestMembersPage_BusinessAxisManage_OpensAndSeesBoth: peran kustom "hr" diberi
// crm:members write (via loadBusinessRolesWith, TANPA baris member_scope_policies
// = default kedua jenis terbuka) → halaman terbuka, kedua kind terlihat, boleh
// mengedit kind (manage && scope.Both()).
func TestMembersPage_BusinessAxisManage_OpensAndSeesBoth(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	internal := env.seedMember(t, "internal@local", "member", 0).ID
	external := env.seedMember(t, "external@local", "member", 0).ID
	if err := env.q.UpdateMembershipKind(t.Context(), db.UpdateMembershipKindParams{
		UserID: external, TenantID: env.tenantID, Kind: authz.KindExternal,
	}); err != nil {
		t.Fatalf("set kind eksternal: %v", err)
	}
	_ = internal

	req := rolesReq(http.MethodGet, "/w/test/members", nil, "")
	rec := env.runAccount(uid, "member", "hr", req, env.h.MembersPage)

	if rec.Code != http.StatusOK {
		t.Fatalf("aktor business-axis 'Kelola' harus bisa buka halaman, got %d\n%s", rec.Code, rec.Body.String())
	}
	html := rec.Body.String()
	if !strings.Contains(html, "internal@local") {
		t.Error("anggota internal harus terlihat (cakupan default kedua jenis)")
	}
	if !strings.Contains(html, "external@local") {
		t.Error("anggota eksternal harus terlihat (cakupan default kedua jenis)")
	}
}

// TestMembersPage_BusinessAxisRead_NoManageActions: crm:members read (BUKAN
// write) → halaman terbuka tapi manage=false (form aksi tak dirender).
// members.go/panel Members merender form ubah kind/role hanya bila manage;
// dibuktikan tak langsung dari markup form melainkan dari fakta bahwa aktor
// yang sama ditolak saat benar-benar POST (lihat TestMemberBiz_ReadOnlyForbidden).
//
// Email TAK disamarkan bagi aktor ini: gerbang halaman (canViewMembers) sudah
// mempersempit penglihat ke pengelola ATAU role bisnis ber-akses "User
// Management" (Lihat/Kelola) — begitu seseorang lolos gerbang itu, ia berhak
// melihat direktori penuh, terlepas dari "Lihat" atau "Kelola".
func TestMembersPage_BusinessAxisRead_NoManageActions(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "read"})
	env.seedRole(t, "hr", "HR", "all", false)
	env.seedMember(t, "m@local", "member", 0)

	req := rolesReq(http.MethodGet, "/w/test/members", nil, "")
	rec := env.runAccount(uid, "member", "hr", req, env.h.MembersPage)

	if rec.Code != http.StatusOK {
		t.Fatalf("aktor business-axis 'Lihat' harus bisa buka halaman, got %d", rec.Code)
	}
	html := rec.Body.String()
	if !strings.Contains(html, "m@local") {
		t.Error("aktor 'Lihat' harus tetap melihat email utuh — gerbang halaman sudah cukup ketat")
	}
}

// TestMembersPage_ScopeFiltersKind: peran "hr" dibatasi member_scope_policies
// ke internal-saja → anggota eksternal TAK BOLEH sampai ke browser (filter di
// handler, bukan cuma disembunyikan CSS).
func TestMembersPage_ScopeFiltersKind(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}
	external := env.seedMember(t, "external@local", "member", 0).ID
	if err := env.q.UpdateMembershipKind(t.Context(), db.UpdateMembershipKindParams{
		UserID: external, TenantID: env.tenantID, Kind: authz.KindExternal,
	}); err != nil {
		t.Fatalf("set kind eksternal: %v", err)
	}

	req := rolesReq(http.MethodGet, "/w/test/members", nil, "")
	rec := env.runAccount(uid, "member", "hr", req, env.h.MembersPage)

	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "external@local") {
		t.Error("anggota eksternal TAK BOLEH sampai ke browser aktor bercakupan internal-saja")
	}
}

// TestMembersPage_NoMembersPerm_StillForbidden: peran kustom TANPA crm:members
// sama sekali tetap 403 — regresi 0004/0008: BL-171 MELEBARKAN, tak membuka
// halaman ini utk sembarang peran CRM (mis. "sales" yang punya crm:accounts
// tapi bukan crm:members).
func TestMembersPage_NoMembersPerm_StillForbidden(t *testing.T) {
	env, uid := setupRoles(t)

	req := rolesReq(http.MethodGet, "/w/test/members", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.MembersPage)

	if rec.Code != http.StatusForbidden {
		t.Errorf("peran tanpa crm:members harus 403, got %d", rec.Code)
	}
}

// --- MemberSetKind: reassignment lintas cakupan -----------------------------

// TestMemberSetKind_NarrowScope_CannotReassignAcrossBoundary: aktor bercakupan
// internal-saja mencoba memindahkan anggota internal → external (LINTAS
// cakupan) → forbidden, kind tak berubah.
func TestMemberSetKind_NarrowScope_CannotReassignAcrossBoundary(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}
	member := env.seedMember(t, "m@local", "member", 0).ID // kind=internal default

	form := url.Values{"kind": {"external"}}
	rec := env.runAccount(uid, "member", "hr", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("reassignment lintas cakupan harus err=forbidden, got %q", loc)
	}
	if got := env.memberKind(t, member); got != "internal" {
		t.Errorf("kind tak boleh berubah, got %q", got)
	}
}

// TestMemberSetKind_NarrowScope_CanEditBusinessRoleWithoutTouchingKind: aktor
// bercakupan internal-saja MASIH bisa menyunting business_role target in-scope
// selama kind yang dikirim SAMA dengan kind target (bukan reassignment).
func TestMemberSetKind_NarrowScope_CanEditBusinessRoleWithoutTouchingKind(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}
	member := env.seedMember(t, "m@local", "member", 0).ID // kind=internal default

	form := url.Values{"kind": {"internal"}, "business_role": {"sales"}}
	rec := env.runAccount(uid, "member", "hr", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_assigned") {
		t.Fatalf("edit business_role in-scope tanpa reassignment kind harus ok=crm_assigned, got %q (status %d)", loc, rec.Code)
	}
	if got := env.bizRole(t, member); got != "sales" {
		t.Errorf("business_role = %q, want sales", got)
	}
}

// TestMemberSetKind_OutOfScopeTarget_ForbiddenOutright: target BERADA di luar
// cakupan aktor (kind=external, aktor internal-saja) → ditolak SEBELUM apa pun
// diperiksa/diterapkan, terlepas dari apa yang dikirim form.
func TestMemberSetKind_OutOfScopeTarget_ForbiddenOutright(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}
	member := env.seedMember(t, "m@local", "member", 0).ID
	if err := env.q.UpdateMembershipKind(t.Context(), db.UpdateMembershipKindParams{
		UserID: member, TenantID: env.tenantID, Kind: authz.KindExternal,
	}); err != nil {
		t.Fatalf("set kind eksternal: %v", err)
	}

	form := url.Values{"kind": {"external"}} // kind TAK berubah, tetap harus ditolak
	rec := env.runAccount(uid, "member", "hr", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("target di luar cakupan harus err=forbidden meski kind tak berubah, got %q", loc)
	}
}

// TestMemberSetKind_BroadScope_CanReassignAcrossBoundary: aktor bercakupan
// KEDUA jenis boleh melakukan reassignment lintas kind — jalur positif yang
// dikontraskan dengan test narrow-scope di atas.
func TestMemberSetKind_BroadScope_CanReassignAcrossBoundary(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	member := env.seedMember(t, "m@local", "member", 0).ID // kind=internal default

	form := url.Values{"kind": {"external"}}
	rec := env.runAccount(uid, "member", "hr", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=kind_changed") {
		t.Fatalf("aktor bercakupan kedua jenis harus bisa reassign, got %q (status %d)", loc, rec.Code)
	}
	if got := env.memberKind(t, member); got != "external" {
		t.Errorf("kind = %q, want external", got)
	}
}
