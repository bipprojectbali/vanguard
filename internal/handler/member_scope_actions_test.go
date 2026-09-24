package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/db"
)

// member_scope_actions_test.go — dipisah dari member_scope_test.go (BL-171)
// agar tiap file di bawah ambang tipe Test (400). Isinya tiga section yang
// mengetes JALUR AKSI sumbu bisnis: MemberSetRole (sumbu tenant tetap
// terkunci), InviteCreate/InviteDelete (dibatasi actorKindScope), dan
// MemberRemove/GuardDelete (batasan yang diketahui & sengaja dipertahankan).
// Helper bersama (memberScopePolicy, inviteDeleteReq, seedInviteKind) tetap di
// member_scope_test.go. Lihat header file itu untuk konteks BL-171 lengkap.

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
