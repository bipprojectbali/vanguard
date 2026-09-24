package handler

import (
	"context"
	"errors"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// member_access.go — gerbang akses /members SUMBU BISNIS (BL-171): MELEBARKAN
// akses, TIDAK MENGGANTI canManageMembers (invite_service.go) — jalur tenant-
// axis (owner/admin/platform) tetap utuh dan TAK disentuh. Role kustom yang
// diberi Lihat/Kelola pada module "crm:members" (business_modules.go) kini
// juga bisa membuka /members, dibatasi actorKindScope (Cakupan Jenis Anggota,
// member_scope_policies, BL-171) yang menentukan anggota BERJENIS APA yang
// terlihat/terkelola baginya. Ini merevisi ADR-0008 ("daftar anggota hanya
// pengelola") — lihat plan BL-171 keputusan desain #1: perubahan role TENANT
// (member/admin/owner) TETAP HANYA lewat canManageMembers; actor business-axis
// tak pernah bisa mengeskalasi otoritas tenant lewat izin CRM yang melebar.

type memberAccessLevel int

const (
	memberAccessNone memberAccessLevel = iota
	memberAccessView
	memberAccessManage
)

// crmMemberAccess melaporkan level akses SUMBU BISNIS aktor pada module
// "crm:members" (Kelola > Lihat > Tak ada). Independen dari canManageMembers
// (sumbu tenant) — keduanya dikombinasikan pemanggil.
func crmMemberAccess(ctx context.Context) memberAccessLevel {
	if authz.CanBusiness(ctx, "crm:members", "write") {
		return memberAccessManage
	}
	if authz.CanBusiness(ctx, "crm:members", "read") {
		return memberAccessView
	}
	return memberAccessNone
}

// canViewMembers melaporkan apakah aktor boleh membuka /members SAMA SEKALI —
// gerbang GET, DILEBARKAN (bukan diganti) dari canManageMembers: owner/admin/
// platform tetap lolos seperti sebelumnya; kini role kustom ber-crm:members
// Lihat/Kelola ikut lolos.
func canViewMembers(ctx context.Context) bool {
	return canManageMembers(ctx) || crmMemberAccess(ctx) != memberAccessNone
}

// kindScope = Jenis Anggota (internal/eksternal) yang boleh dilihat/dikelola
// aktor di /members — hasil actorKindScope.
type kindScope struct {
	Internal bool
	External bool
}

// Both melaporkan cakupan PENUH — dipakai menggerbangi aksi yang butuh
// melihat kedua jenis sekaligus (mis. memindah anggota antar jenis, plan
// BL-171 keputusan desain #3: kolom Kind hanya bisa diubah aktor bercakupan
// kedua jenis).
func (s kindScope) Both() bool { return s.Internal && s.External }

// Allows melaporkan apakah kind (nilai "internal"/"external", authz.KindInternal/
// KindExternal) termasuk cakupan aktor.
func (s kindScope) Allows(kind string) bool {
	if kind == authz.KindExternal {
		return s.External
	}
	return s.Internal
}

// actorKindScope menentukan cakupan Jenis Anggota aktor SAAT INI.
//
// canManageMembers (sumbu tenant) → cakupan PENUH selalu, tak bergantung
// member_scope_policies sama sekali (keputusan desain #1: tenant-axis tak
// pernah dipersempit oleh setting CRM — owner/admin/platform TETAP melihat
// semua, persis perilaku sebelum BL-171).
//
// Sumbu bisnis (business_role aktor) → dibaca dari member_scope_policies;
// baris tak ada (pgx.ErrNoRows, role baru saja diberi akses crm:members atau
// memang belum pernah dikonfigurasi) → default KEDUA jenis terbuka, sama
// seperti memberScopeView (roles_page.go). Error lain (koneksi dsb.) →
// fail-CLOSED ({false,false}) — beda dari memberScopeView yang fail-open:
// di /roles kegagalan baca hanya mempengaruhi TAMPILAN form (server tetap
// penjaga sesungguhnya saat submit), sedangkan di sini kegagalan baca
// LANGSUNG menentukan data mana yang boleh keluar — fail-closed adalah
// pilihan yang benar.
func actorKindScope(ctx context.Context, q *db.Queries) kindScope {
	if canManageMembers(ctx) {
		return kindScope{Internal: true, External: true}
	}
	role := session.BusinessRole(ctx)
	if role == "" {
		return kindScope{}
	}
	pol, err := q.GetMemberScopePolicy(ctx, db.GetMemberScopePolicyParams{
		TenantID: session.TenantID(ctx), BusinessRole: role,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return kindScope{Internal: true, External: true}
		}
		return kindScope{} // fail-closed
	}
	return kindScope{Internal: pol.CanViewInternal, External: pol.CanViewExternal}
}
