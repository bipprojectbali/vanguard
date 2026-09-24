package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// member_scope_test.go — BL-171: module Casbin baru "crm:members" (User
// Management) yang MELEBARKAN akses /members ke role kustom, plus "Cakupan
// Jenis Anggota" (member_scope_policies) yang membatasi jenis anggota (kind)
// mana yang boleh dilihat/dikelola aktor yang masuk lewat sumbu bisnis. Yang
// dijaga di sini (RoleUpdate & CHECK DB); pengujian JALUR AKSES & AKSI dipisah
// ke member_scope_access_test.go & member_scope_actions_test.go agar tiap
// file di bawah ambang tipe Test (400):
//
//   - RoleUpdate: simpan/hapus/auto-both baris member_scope_policies, sentinel
//     mscope_present persis pola fsec_present (BL-145).
//   - CHECK DB msp_at_least_one_chk: baris dua kolom false ditolak.
//   - MembersPage & MemberSetKind (→ member_scope_access_test.go): role kustom
//     ber-crm:members read/write kini bisa membuka halaman, difilter
//     actorKindScope; reassignment kind lintas-cakupan hanya utk aktor
//     bercakupan KEDUA jenis.
//   - MemberSetRole, InviteCreate/InviteDelete, MemberRemove/GuardDelete
//     (→ member_scope_actions_test.go): sumbu TENANT tetap terkunci
//     canManageMembers murni meski aktor business-axis lolos gerbang luar;
//     sumbu CRM (business_role) adalah jalur akses melebar yang sesungguhnya.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// diuji terpisah di rls_test.go. Peran CRM bawaan + enforcer nyata ditanam
// via setupRoles (roles_test.go).

// memberScopePolicy membaca baris member_scope_policies satu peran; found=false
// bila absen (pgx.ErrNoRows) — pasangan pgx.ErrNoRows/fail-closed persis makna
// GetMemberScopePolicy bagi actorKindScope.
func (e *testEnv) memberScopePolicy(t *testing.T, role string) (db.MemberScopePolicy, bool) {
	t.Helper()
	pol, err := e.q.GetMemberScopePolicy(t.Context(), db.GetMemberScopePolicyParams{
		TenantID: e.tenantID, BusinessRole: role,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.MemberScopePolicy{}, false
		}
		t.Fatalf("get member scope policy %q: %v", role, err)
	}
	return pol, true
}

// inviteDeleteReq membangun POST /w/test/members/invite/{id}/delete dengan chi
// param workspace + id — tak ada test InviteDelete lain yang sudah menanam
// helper ini.
func inviteDeleteReq(id string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/w/test/members/invite/"+id+"/delete", strings.NewReader(""))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workspace", "test")
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// seedInviteKind membuat undangan pending langsung di DB (melewati handler)
// dengan kind eksplisit — mirror mkInvite (invite_test.go) yang hardcode
// kind="internal", dibutuhkan di sini untuk menanam undangan eksternal.
func (e *testEnv) seedInviteKind(t *testing.T, email, kind string) db.Invite {
	t.Helper()
	inv, err := e.q.CreateInvite(t.Context(), db.CreateInviteParams{
		TenantID:  e.tenantID,
		Email:     email,
		Role:      authz.RoleNameMember,
		Token:     email + "-tok",
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		Kind:      kind,
	})
	if err != nil {
		t.Fatalf("seed invite kind=%s: %v", kind, err)
	}
	return inv
}

// --- RoleUpdate: simpan Cakupan Jenis Anggota -------------------------------

// TestRoleUpdate_MemberScope_SavesPartialScope: level.crm:members=write +
// mscope_present=1 + hanya internal tercentang → baris tersimpan
// {internal:true, external:false}, BUKAN auto-both (setidaknya satu tercentang).
func TestRoleUpdate_MemberScope_SavesPartialScope(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "hr", "HR", "all", false)

	form := roleFormValues("HR", "all")
	form.Set("level.crm:members", "write")
	form.Set("mscope_present", "1")
	form.Set("mscope_internal", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/hr", form, "hr")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	pol, found := env.memberScopePolicy(t, "hr")
	if !found {
		t.Fatal("baris member_scope_policies harus tersimpan")
	}
	if !pol.CanViewInternal || pol.CanViewExternal {
		t.Errorf("cakupan = {internal:%v external:%v}, want {true false}", pol.CanViewInternal, pol.CanViewExternal)
	}
}

// TestRoleUpdate_MemberScope_AutoBothWhenNoneChecked: level aktif tapi form tak
// kirim satu pun checkbox cakupan → coerce ke KEDUA jenis terbuka (default
// opt-in, bukan tolak submit — JS sudah mencegah ini di client, sini lapis server).
func TestRoleUpdate_MemberScope_AutoBothWhenNoneChecked(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "hr", "HR", "all", false)

	form := roleFormValues("HR", "all")
	form.Set("level.crm:members", "write")
	form.Set("mscope_present", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/hr", form, "hr")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	pol, found := env.memberScopePolicy(t, "hr")
	if !found {
		t.Fatal("baris member_scope_policies harus tersimpan (auto-both)")
	}
	if !pol.CanViewInternal || !pol.CanViewExternal {
		t.Errorf("cakupan harus auto-coerce ke {true true}, got {%v %v}", pol.CanViewInternal, pol.CanViewExternal)
	}
}

// TestRoleUpdate_MemberScope_DeletedWhenLevelNone: baris existing dihapus saat
// level.crm:members kembali ke "none" — "nonaktif" direpresentasikan lewat
// ABSENnya baris (CHECK DB menolak false/false), bukan dua kolom false.
func TestRoleUpdate_MemberScope_DeletedWhenLevelNone(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed baris cakupan: %v", err)
	}

	form := roleFormValues("HR", "all")
	form.Set("mscope_present", "1") // level.crm:members TAK dikirim = none
	req := rolesReq(http.MethodPost, "/w/test/roles/hr", form, "hr")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	if _, found := env.memberScopePolicy(t, "hr"); found {
		t.Error("baris cakupan harus terhapus saat level.crm:members = none")
	}
}

// TestRoleUpdate_MemberScope_AbsentSentinelLeavesUntouched: form TANPA
// mscope_present (pemanggil lama/test yang tak tahu-menahu soal cakupan ini) —
// matriks level.crm:members tetap tersimpan, tapi TAK ADA baris
// member_scope_policies ditulis/dihapus sama sekali. Mirror persis
// TestRoleUpdate_MergedFsec_AbsentSentinelLeavesUntouched.
func TestRoleUpdate_MemberScope_AbsentSentinelLeavesUntouched(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "hr", "HR", "all", false)

	form := roleFormValues("HR", "all")
	form.Set("level.crm:members", "write")
	// mscope_present sengaja TAK di-set.
	req := rolesReq(http.MethodPost, "/w/test/roles/hr", form, "hr")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	if !env.hasPerm(t, "hr", "crm:members", "write") {
		t.Error("matriks level.crm:members harus tetap tersimpan walau sentinel absen")
	}
	if _, found := env.memberScopePolicy(t, "hr"); found {
		t.Error("sentinel absen tak boleh menulis baris member_scope_policies apa pun")
	}
}

// --- CHECK DB msp_at_least_one_chk ------------------------------------------

// TestMemberScopePolicy_ChecksAtLeastOneKind: insert langsung dgn kedua kolom
// false ditolak CHECK DB — pertahanan lapis terakhir di belakang coercion
// handler (RoleUpdate selalu meng-coerce sebelum sampai sini).
func TestMemberScopePolicy_ChecksAtLeastOneKind(t *testing.T) {
	env, _ := setupRoles(t)
	env.seedRole(t, "hr", "HR", "all", false)

	err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr",
		CanViewInternal: false, CanViewExternal: false,
	})
	if err == nil {
		t.Fatal("insert dengan kedua kolom false harus ditolak CHECK msp_at_least_one_chk")
	}
}
