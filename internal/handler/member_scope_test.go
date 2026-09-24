package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
// dijaga:
//
//   - RoleUpdate: simpan/hapus/auto-both baris member_scope_policies, sentinel
//     mscope_present persis pola fsec_present (BL-145).
//   - CHECK DB msp_at_least_one_chk: baris dua kolom false ditolak.
//   - MembersPage: role kustom ber-crm:members read/write kini bisa membuka
//     halaman, difilter actorKindScope; role tanpa crm:members TETAP 403
//     (regresi 0004/0008 tak berubah).
//   - MemberSetKind: reassignment kind lintas-cakupan hanya utk aktor
//     bercakupan KEDUA jenis; target di luar cakupan ditolak sama sekali.
//   - MemberSetRole: sumbu TENANT tetap terkunci canManageMembers murni meski
//     aktor business-axis lolos gerbang luar; sumbu CRM (business_role) adalah
//     jalur akses melebar yang sesungguhnya.
//   - InviteCreate/InviteDelete: dibatasi actorKindScope aktor business-axis.
//   - MemberRemove/GuardDelete: aktor business-axis murni (tenant role
//     "member") tetap diblokir GuardDelete thd target setara — batasan yang
//     diketahui & sengaja dipertahankan (plan BL-171 keputusan #1).
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

// --- MemberSetRole: sumbu tenant TETAP terkunci -----------------------------

// TestMemberSetRole_BusinessAxisOnly_RoleFieldWholeRequestForbidden: aktor
// business-axis murni (bukan canManageMembers) mengirim field "role" (sumbu
// TENANT) → SELURUH request ditolak forbidden SEBELUM sumbu CRM diproses sama
// sekali — bukan "tenant di-skip, business_role tetap jalan". Ini guard
// anti-eskalasi (plan BL-171 keputusan desain #1): tak ada earlier `return`
// parsial, jadi business_role yang disertakan dalam POST yang sama JUGA harus
// gagal diterapkan.
func TestMemberSetRole_BusinessAxisOnly_RoleFieldWholeRequestForbidden(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"role": {"admin"}, "business_role": {"sales"}}
	rec := env.runAccount(uid, "member", "hr", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("kirim field 'role' oleh aktor business-axis murni harus err=forbidden, got %q", loc)
	}
	m, err := env.q.GetMembership(t.Context(), db.GetMembershipParams{UserID: member, TenantID: env.tenantID})
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if m.Role != "member" {
		t.Errorf("sumbu tenant tak boleh berubah, got role=%q", m.Role)
	}
	if m.BusinessRole != nil {
		t.Errorf("business_role JUGA tak boleh diterapkan — request ditolak SELURUHNYA, got %v", *m.BusinessRole)
	}
}

// TestMemberSetRole_BusinessAxisOnly_BusinessRoleOnlySucceeds: aktor
// business-axis murni mengirim HANYA business_role (tanpa field "role") →
// berhasil normal. Ini jalur akses MELEBAR yang sesungguhnya (BL-171).
func TestMemberSetRole_BusinessAxisOnly_BusinessRoleOnlySucceeds(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"business_role": {"sales"}}
	rec := env.runAccount(uid, "member", "hr", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_assigned") {
		t.Fatalf("business_role-saja oleh aktor business-axis harus ok=crm_assigned, got %q (status %d)", loc, rec.Code)
	}
	if got := env.bizRole(t, member); got != "sales" {
		t.Errorf("business_role = %q, want sales", got)
	}
}

// TestMemberSetRole_NoManagePerm_Forbidden: peran CRM tanpa crm:members
// (mis. "sales" murni) tetap ditolak seluruhnya — regresi gerbang luar.
func TestMemberSetRole_NoManagePerm_Forbidden(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"business_role": {"sales"}}
	rec := env.runAccount(uid, "member", "sales", memberRoleReq(itoa(member), form), env.h.MemberSetRole)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("peran tanpa crm:members harus err=forbidden, got %q", loc)
	}
	if got := env.bizRole(t, member); got != "" {
		t.Errorf("tak berwenang tak boleh menyimpan business_role, got %q", got)
	}
}

// --- InviteCreate/InviteDelete: dibatasi actorKindScope ---------------------

// TestInviteCreate_BusinessAxis_OutOfScopeKind_Rejected: aktor bercakupan
// internal-saja mencoba mengundang kind=external → ditolak forbidden, tak
// membuat undangan.
func TestInviteCreate_BusinessAxis_OutOfScopeKind_Rejected(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}

	form := url.Values{"email": {"calon@luar.com"}, "kind": {"external"}}
	req := inviteCreateReq(form)
	rec := env.runAccount(uid, "member", "hr", req, env.h.InviteCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("undang di luar cakupan harus err=forbidden, got %q", loc)
	}
	if exists, _ := env.q.MemberExistsByEmail(t.Context(), db.MemberExistsByEmailParams{
		TenantID: env.tenantID, Lower: "calon@luar.com",
	}); exists {
		t.Error("tak boleh ada efek samping — undangan ditolak sebelum tersimpan")
	}
}

// TestInviteCreate_BusinessAxis_InScope_Succeeds: aktor bercakupan
// internal-saja mengundang kind=internal (dalam cakupan) → berhasil.
func TestInviteCreate_BusinessAxis_InScope_Succeeds(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}

	form := url.Values{"email": {"calon@dalam.com"}, "kind": {"internal"}}
	req := inviteCreateReq(form)
	rec := env.runAccount(uid, "member", "hr", req, env.h.InviteCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=invited") {
		t.Fatalf("undang dalam cakupan harus ok=invited, got %q (status %d)", loc, rec.Code)
	}
}

// TestInviteDelete_BusinessAxis_OutOfScopeKind_Rejected: aktor bercakupan
// internal-saja mencoba batalkan undangan kind=external → ditolak, undangan
// tetap ada.
func TestInviteDelete_BusinessAxis_OutOfScopeKind_Rejected(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}
	inv := env.seedInviteKind(t, "luar@contoh.com", authz.KindExternal)

	rec := env.runAccount(uid, "member", "hr", inviteDeleteReq(itoa(inv.ID)), env.h.InviteDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("hapus undangan di luar cakupan harus err=forbidden, got %q", loc)
	}
	if _, err := env.q.GetInviteByToken(t.Context(), inv.Token); err != nil {
		t.Error("undangan tak boleh terhapus")
	}
}

// TestInviteDelete_BusinessAxis_InScope_Succeeds: aktor bercakupan
// internal-saja membatalkan undangan kind=internal (dalam cakupan) → berhasil.
func TestInviteDelete_BusinessAxis_InScope_Succeeds(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	if err := env.q.UpsertMemberScopePolicy(t.Context(), db.UpsertMemberScopePolicyParams{
		TenantID: env.tenantID, BusinessRole: "hr", CanViewInternal: true, CanViewExternal: false,
	}); err != nil {
		t.Fatalf("seed cakupan internal-saja: %v", err)
	}
	inv := env.seedInviteKind(t, "dalam@contoh.com", authz.KindInternal)

	rec := env.runAccount(uid, "member", "hr", inviteDeleteReq(itoa(inv.ID)), env.h.InviteDelete)

	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Errorf("hapus undangan dalam cakupan tak boleh error, got %q (status %d)", loc, rec.Code)
	}
	if _, err := env.q.GetInviteByToken(t.Context(), inv.Token); err == nil {
		t.Error("undangan harus terhapus")
	}
}

// --- MemberRemove/GuardDelete: batasan yang diketahui & sengaja -------------

// TestMemberRemove_BusinessAxisOnly_BlockedByGuardDeleteEqualRole: aktor
// business-axis murni (tenant role "member") mencoba mengeluarkan anggota lain
// yang JUGA tenant role "member" — actorKindScope mengizinkan (kind sama-sama
// masuk cakupan), tapi authz.GuardDelete (lapisan sumbu ROLE TENANT, tak
// disentuh BL-171) tetap menolak: target.Role >= effectiveRole(actor) saat
// keduanya "member". Ini BUKAN regresi — plan BL-171 eksplisit mempertahankan
// GuardDelete sebagai penjaga sesungguhnya di sini (BL-171 melebarkan akses
// LIHAT/KELOLA data CRM, bukan hierarki wewenang tenant).
func TestMemberRemove_BusinessAxisOnly_BlockedByGuardDeleteEqualRole(t *testing.T) {
	env, uid := setupRoles(t)
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "hr", Obj: "crm:members", Act: "write"})
	env.seedRole(t, "hr", "HR", "all", false)
	target := env.seedMember(t, "t@local", "member", 0).ID

	req := memberRoleReq(itoa(target), url.Values{}) // reuse chi param builder (workspace+id)
	req.Method = http.MethodPost
	req.URL.Path = "/w/test/members/" + itoa(target) + "/remove"
	rec := env.runAccount(uid, "member", "hr", req, env.h.MemberRemove)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("GuardDelete harus tetap menolak target setara tenant-role, got %q", loc)
	}
	if _, err := env.q.GetMembership(t.Context(), db.GetMembershipParams{UserID: target, TenantID: env.tenantID}); err != nil {
		t.Error("keanggotaan target tak boleh terhapus")
	}
}
