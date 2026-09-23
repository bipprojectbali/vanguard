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

// member_kind_test.go — applyMemberKind / POST /w/{slug}/members/{id}/kind
// (BL-170): sumbu KETIGA Jenis Anggota (internal/eksternal), form TERPISAH
// dari role tenant + Peran CRM (member_business_role_test.go). Yang dijaga:
//
//   - Pengelola mengubah kind anggota → ok=kind_changed, persist, audit +
//     notifikasi member.kind.changed.
//   - No-op saat kind tak berubah (ok="", tak diaudit) — bukan "sukses palsu".
//   - Auto-reset business_role ke NULL bila peran lama tak cocok kind baru
//     (ok=kind_changed_reset) — mencegah kombinasi kind/peran yang korup.
//   - business_role DIPERTAHANKAN bila masih cocok kind baru (perlu peran
//     kind=external ditanam manual — seedRole hardcode internal).
//   - Forbidden utk anggota biasa; kind tak sah ditolak err=kind.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// diuji terpisah di rls_test.go.

// memberKindReq membangun POST /w/test/members/{id}/kind dengan chi param
// workspace + id dan body form terkode.
func memberKindReq(id string, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/w/test/members/"+id+"/kind",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workspace", "test")
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// memberKind membaca kind satu anggota.
func (e *testEnv) memberKind(t *testing.T, userID int64) string {
	t.Helper()
	m, err := e.q.GetMembership(t.Context(), db.GetMembershipParams{
		UserID: userID, TenantID: e.tenantID,
	})
	if err != nil {
		t.Fatalf("get membership %d: %v", userID, err)
	}
	return m.Kind
}

const kindChanged = "member.kind.changed"

// wasAudited melaporkan (tanpa gagalkan test) apakah action tertentu tercatat
// — pasangan NEGATIF assertAudited, utk membuktikan no-op TAK meninggalkan jejak.
func (e *testEnv) wasAudited(t *testing.T, action string) bool {
	t.Helper()
	logs, err := e.q.ListAuditLogs(t.Context(), 50)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	for _, l := range logs {
		if l.Action == action {
			return true
		}
	}
	return false
}

// TestMemberKind_ChangedByManager: owner mengubah kind member internal→external
// → ok=kind_changed, persist, audit + notifikasi tercatat.
func TestMemberKind_ChangedByManager(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"kind": {"external"}}
	rec := env.runAccount(uid, "owner", "", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=kind_changed") ||
		strings.Contains(loc, "kind_changed_reset") {
		t.Fatalf("harus ok=kind_changed (bukan reset), got %q (status %d)", loc, rec.Code)
	}
	if got := env.memberKind(t, member); got != "external" {
		t.Errorf("kind = %q, want external", got)
	}
	env.assertAudited(t, kindChanged)
	if !env.hasNotif(t, member, kindChanged) {
		t.Error("anggota harus diberi tahu perubahan Jenis Anggota")
	}
}

// TestMemberKind_NoOpWhenUnchanged: kind form == kind existing → ok kosong,
// tak diaudit (bukan sukses palsu).
func TestMemberKind_NoOpWhenUnchanged(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID // default kind=internal

	form := url.Values{"kind": {"internal"}}
	rec := env.runAccount(uid, "owner", "", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	loc := rec.Header().Get("Location")
	if strings.Contains(loc, "ok=kind_changed") {
		t.Errorf("kind tak berubah tak boleh melapor sukses, got %q", loc)
	}
	if env.wasAudited(t, kindChanged) {
		t.Error("no-op tak boleh menghasilkan entri audit")
	}
}

// TestMemberKind_AutoResetInvalidBusinessRole: business_role existing (internal)
// tak lagi cocok saat kind diubah ke external → business_role auto-reset NULL,
// ok=kind_changed_reset (bukan korupsi kombinasi kind/peran).
func TestMemberKind_AutoResetInvalidBusinessRole(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID
	env.assignBiz(t, member, "sales") // sales = bawaan, kind internal

	form := url.Values{"kind": {"external"}}
	rec := env.runAccount(uid, "owner", "", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=kind_changed_reset") {
		t.Fatalf("harus ok=kind_changed_reset, got %q (status %d)", loc, rec.Code)
	}
	if got := env.memberKind(t, member); got != "external" {
		t.Errorf("kind = %q, want external", got)
	}
	if got := env.bizRole(t, member); got != "" {
		t.Errorf("business_role tak lagi cocok kind baru harus di-reset NULL, got %q", got)
	}
}

// TestMemberKind_RetainsValidBusinessRole: business_role existing yang kind-nya
// SUDAH cocok kind TUJUAN (walau kind anggota itu sendiri baru berubah ke sana)
// tetap DIPERTAHANKAN, tak ikut ter-reset. Peran "mitra" ditanam kind=external
// (seedRole helper hardcode internal, jadi ditanam manual via CreateBusinessRole).
func TestMemberKind_RetainsValidBusinessRole(t *testing.T) {
	env, uid := setupRoles(t)
	ctx := t.Context()
	if _, err := env.q.CreateBusinessRole(ctx, db.CreateBusinessRoleParams{
		TenantID: env.tenantID, Name: "mitra", DisplayName: "Mitra",
		DataScope: "own", Kind: "external",
	}); err != nil {
		t.Fatalf("seed peran eksternal: %v", err)
	}
	// Anggota kind=internal (default) tapi business_role-nya sudah "mitra"
	// (kind=external) — state yang normalnya dicegah applyBusinessRole,
	// ditanam langsung di sini utk membuktikan applyMemberKind menilai
	// kecocokan business_role TERHADAP KIND TUJUAN, bukan kind lama.
	member := env.seedMember(t, "m@local", "member", 0).ID
	env.assignBiz(t, member, "mitra")

	form := url.Values{"kind": {"external"}}
	rec := env.runAccount(uid, "owner", "", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=kind_changed") ||
		strings.Contains(loc, "kind_changed_reset") {
		t.Fatalf("harus ok=kind_changed TANPA reset, got %q (status %d)", loc, rec.Code)
	}
	if got := env.bizRole(t, member); got != "mitra" {
		t.Errorf("business_role yang masih cocok kind baru tak boleh ter-reset, got %q", got)
	}
}

// TestMemberKind_Forbidden: anggota biasa tak boleh mengubah kind siapa pun.
func TestMemberKind_Forbidden(t *testing.T) {
	env, uid := setupRoles(t)
	target := env.seedMember(t, "t@local", "member", 0).ID

	form := url.Values{"kind": {"external"}}
	rec := env.runAccount(uid, "member", "", memberKindReq(itoa(target), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("anggota biasa harus err=forbidden, got %q", loc)
	}
	if got := env.memberKind(t, target); got != "internal" {
		t.Errorf("tak berwenang tak boleh mengubah kind, got %q", got)
	}
}

// TestMemberKind_TogetherWithBusinessRole: SATU POST membawa kind+business_role
// sekaligus (form gabungan baris /members, SATU tombol Simpan) — ok=kind_role_changed,
// persist kedua sumbu.
func TestMemberKind_TogetherWithBusinessRole(t *testing.T) {
	env, uid := setupRoles(t)
	if _, err := env.q.CreateBusinessRole(t.Context(), db.CreateBusinessRoleParams{
		TenantID: env.tenantID, Name: "mitra", DisplayName: "Mitra",
		DataScope: "own", Kind: "external",
	}); err != nil {
		t.Fatalf("seed peran eksternal: %v", err)
	}
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"kind": {"external"}, "business_role": {"mitra"}}
	rec := env.runAccount(uid, "owner", "", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=kind_role_changed") {
		t.Fatalf("harus ok=kind_role_changed, got %q (status %d)", loc, rec.Code)
	}
	if got := env.memberKind(t, member); got != "external" {
		t.Errorf("kind = %q, want external", got)
	}
	if got := env.bizRole(t, member); got != "mitra" {
		t.Errorf("business_role = %q, want mitra", got)
	}
}

// TestMemberKind_BusinessRoleOnlyChanged: kind sama (no-op), hanya business_role
// berubah → ok=crm_assigned (bukan kind_changed).
func TestMemberKind_BusinessRoleOnlyChanged(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID // kind=internal default

	form := url.Values{"kind": {"internal"}, "business_role": {"sales"}}
	rec := env.runAccount(uid, "owner", "", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=crm_assigned") {
		t.Fatalf("harus ok=crm_assigned, got %q (status %d)", loc, rec.Code)
	}
	if got := env.bizRole(t, member); got != "sales" {
		t.Errorf("business_role = %q, want sales", got)
	}
}

// TestMemberKind_InvalidRejected: nilai kind tak dikenal ditolak err=kind,
// tak menyentuh DB.
func TestMemberKind_InvalidRejected(t *testing.T) {
	env, uid := setupRoles(t)
	member := env.seedMember(t, "m@local", "member", 0).ID

	form := url.Values{"kind": {"bogus"}}
	rec := env.runAccount(uid, "owner", "", memberKindReq(itoa(member), form), env.h.MemberSetKind)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=kind") {
		t.Errorf("kind tak sah harus err=kind, got %q", loc)
	}
	if got := env.memberKind(t, member); got != "internal" {
		t.Errorf("kind tak sah tak boleh tersimpan, got %q", got)
	}
}
