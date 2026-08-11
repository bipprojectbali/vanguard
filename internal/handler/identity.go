package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// RefreshIdentity me-load identitas user SEGAR dari DB tiap request (untuk user
// login) dan menyegarkan session. Ini membuat otorisasi real-time & self-healing:
//
//   - Role/status berubah di DB → langsung berlaku (tak tunggu re-login).
//   - User di-block/disable → logout paksa seketika.
//   - User di-soft-delete → session dibersihkan.
//   - Session lama tanpa role (shape berubah saat dev) → terisi ulang dari DB.
//
// Super-admin env (root) tetap kebal gate status. No-op untuk anonim.
// Pasang pada route yang butuh identitas akurat (Home + grup terproteksi),
// bukan pada static/health (hindari DB hit tak perlu).
func (h *Handler) RefreshIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		uid := session.UserID(ctx)
		if uid == 0 {
			next.ServeHTTP(w, r) // anonim — tak ada yang perlu disegarkan
			return
		}
		u, err := h.q(ctx).GetUser(ctx, uid)
		if err != nil {
			// User tak ada (dihapus) — GetUser memfilter deleted_at. Session basi.
			_ = session.Clear(ctx)
			next.ServeHTTP(w, r)
			return
		}

		isRoot := isSuperAdminEmail(u.Email)
		// Gate status real-time — root env kebal (tak bisa dikunci).
		if !isRoot && (u.Status == "blocked" || u.Status == "disabled") {
			_ = session.Clear(ctx)
			http.Redirect(w, r, "/login?err=inactive", http.StatusSeeOther)
			return
		}

		// Workspace aktif sudah DIVALIDASI middleware Scope (anggota terverifikasi).
		tenantID := session.TenantID(ctx)
		role, businessRole, dataScope := h.resolveRole(ctx, h.q(ctx), u, tenantID)
		avatar := ""
		if u.AvatarUrl != nil {
			avatar = *u.AvatarUrl
		}
		// Nama workspace segar dari DB (real-time: ganti nama langsung terlihat di
		// brand sidebar). tenants TANPA RLS-policy → terbaca di tx ber-scope. Fail-
		// soft: error → pakai nama tercache (jangan kosongkan brand karena glitch).
		// Slug ikut disegarkan: setiap URL workspace dibentuk darinya (0004), jadi
		// session tanpa slug (mis. dibuat sebelum 0004) akan sembuh sendiri di sini.
		tenantName, tenantSlug := session.TenantName(ctx), session.TenantSlug(ctx)
		if t, err := h.q(ctx).GetTenant(ctx, tenantID); err == nil {
			tenantName, tenantSlug = t.Name, t.Slug
		}
		// Tulis session hanya bila ada yang berubah (hindari commit tiap request).
		if session.Role(ctx) != role || session.IsRoot(ctx) != isRoot ||
			session.Email(ctx) != u.Email || session.AvatarURL(ctx) != avatar ||
			session.TenantName(ctx) != tenantName || session.TenantSlug(ctx) != tenantSlug {
			session.SetIdentity(ctx, u.ID, u.Email, role, isRoot, tenantID, tenantName, tenantSlug, avatar)
		}
		// business_role terpisah dari SetIdentity (sumbu bisnis tegak lurus; ia
		// per-workspace & CRM-only, tak menumpang identitas inti). Disegarkan
		// sendiri agar pindah/pencabutan peran CRM langsung berlaku tanpa re-login.
		// data_scope (F3) ikut role → disegarkan bersama; menyunting cakupan role
		// di panel berefek pada request berikutnya tanpa re-login.
		if session.BusinessRole(ctx) != businessRole {
			session.SetBusinessRole(ctx, businessRole)
		}
		if session.BusinessDataScope(ctx) != dataScope {
			session.SetBusinessDataScope(ctx, dataScope)
		}
		next.ServeHTTP(w, r)
	})
}

// resolveRole menentukan role EFEKTIF dari DB tiap request (real-time),
// mengembalikan DUA sumbu tegak-lurus (docs/crm/sistem-dan-role.md §3):
//
//	role tenant/platform:
//	  super_admin — email di SUPER_ADMIN_EMAILS (env-only, immutable, menang atas apa pun)
//	  staff       — email terdaftar di platform_staff (support platform, mutable)
//	  <membership> — role TENANT (owner/admin/member) di WORKSPACE AKTIF
//	businessRole  — peran CRM (admin/manager/sales/csm/support) di workspace aktif,
//	                dari kolom memberships.business_role; "" bila belum diberi.
//	dataScope     — cakupan data (F3) role itu ('all'/'own'/'none'), dari kolom
//	                business_roles.data_scope; "" bila tak ada role → ScopeNone.
//
// Role tenant kini PER-WORKSPACE (tabel memberships), bukan properti user: orang
// yang sama bisa owner di satu workspace & member di workspace lain. Platform role
// di-overlay di sini, tak pernah disimpan di DB sebagai role tenant.
//
// business_role SENGAJA tak diturunkan dari role platform: super_admin/staff yang
// bukan CSM tetap "" di sumbu bisnis — wewenang platform ≠ memegang desa. Ia
// datang HANYA dari membership; tanpa membership (mis. platform bukan anggota) → "".
//
// q HARUS bisa membaca platform_staff & memberships — keduanya TANPA RLS, jadi
// terbaca di WithTenant maupun WithSuper tx. Fail-soft: error DB → role terendah
// (member) + business_role kosong, jangan naikkan otoritas karena glitch.
func (h *Handler) resolveRole(ctx context.Context, q *db.Queries, u db.User, tenantID int64) (role, businessRole, dataScope string) {
	m, err := q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: tenantID})
	if err == nil && m.BusinessRole != nil {
		businessRole = *m.BusinessRole
		// Cakupan data (F3) role aktif — dibaca dari kolom, bukan ditebak dari nama:
		// role custom bisa bercakupan apa saja. Fail-soft: error/tak ada → "" →
		// db.AccountsScopeFor memetakannya ke ScopeNone (nol baris), bukan bocor.
		if ds, e := q.GetBusinessRoleDataScope(ctx, db.GetBusinessRoleDataScopeParams{
			TenantID: tenantID, Name: businessRole,
		}); e == nil {
			dataScope = ds
		}
	}
	if isSuperAdminEmail(u.Email) {
		return authz.RoleNameSuperAdmin, businessRole, dataScope // env override menang atas semua
	}
	if ok, err := q.IsPlatformStaff(ctx, u.Email); err != nil {
		h.Log.Error("resolveRole: staff lookup", "err", err)
	} else if ok {
		return authz.RoleNameStaff, businessRole, dataScope
	}
	if err != nil {
		// Tak ada membership (mis. baru dicabut) → otoritas terendah, BUKAN naik.
		h.Log.Warn("resolveRole: membership tak ditemukan", "user_id", u.ID, "tenant_id", tenantID)
		return authz.RoleNameMember, "", ""
	}
	return m.Role, businessRole, dataScope
}
