package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// scope_resolve.go — resolusi tenant dari slug/aktif (adopt/tenantBySlug/
// resolve*) & cek peran platform. Dipisah dari middleware Scope di scope.go
// agar file di bawah ambang tipe Route/Handler (150). Satu paket handler.

// adoptTenantBySlug menjadikan workspace ber-slug tsb sebagai konteks aktif TANPA
// memeriksa keanggotaan — khusus role platform, yang memang lintas-workspace
// (super_admin/staff membantu workspace mana pun). false bila slug tak ada.
//
// Terpisah dari resolveTenantBySlug justru agar bedanya kelihatan: yang ini
// SENGAJA tanpa cek membership, dan keputusan itu diambil dari ROLE — tak pernah
// dari data DB (anti privilege-escalation, sama seperti WithSuper).
func (h *Handler) adoptTenantBySlug(ctx context.Context, slug string) (context.Context, bool) {
	t, ok := h.tenantBySlug(ctx, slug)
	if !ok {
		return ctx, false
	}
	if t.ID != session.TenantID(ctx) {
		session.SetActiveTenant(ctx, t.ID, t.Name, t.Slug)
	}
	// Sifat workspace diteruskan seperti pada cabang tenant. Tanpa ini, platform
	// yang membuka workspace primer melihat zona bahaya lengkap dengan tombolnya —
	// aksinya memang akan ditolak (handler + SQL), tapi menawarkan tombol yang
	// pasti gagal adalah cacat tersendiri.
	return withTenantPrimary(ctx, t.IsPrimary), true
}

// tenantBySlug membaca tenant dari slug TANPA memeriksa keanggotaan. Hanya untuk
// jalur PLATFORM (lintas-workspace) — pemanggil wajib sudah memastikan rolenya
// dari session, tak pernah dari data DB (anti privilege-escalation).
func (h *Handler) tenantBySlug(ctx context.Context, slug string) (db.Tenant, bool) {
	var (
		tenant db.Tenant
		found  bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		t, e := q.GetTenantBySlug(ctx, slug)
		if e != nil {
			return nil // slug tak ada → 404, bukan error
		}
		tenant, found = t, true
		return nil
	})
	if err != nil {
		h.Log.Error("scope: baca slug", "slug", slug, "err", err)
		return db.Tenant{}, false
	}
	return tenant, found
}

// resolveTenantBySlug menerjemahkan slug di URL ke tenant_id, HANYA bila user
// benar-benar anggotanya. false untuk slug tak dikenal MAUPUN slug milik orang
// lain — dua keadaan itu sengaja tak dibedakan agar pemanggil (404) tidak
// membocorkan workspace mana yang ada.
//
// Session ikut diperbarui agar jalur tanpa slug berikutnya (/notifications,
// landing) mengikuti workspace yang sedang dibuka — session sebagai PETUNJUK,
// bukan sumber kebenaran (0004).
//
// Dibaca lewat WithSuper: memberships & tenants sengaja tanpa RLS, dan query ini
// justru yang MENENTUKAN tenant — tak bisa bergantung GUC yang belum di-set.
// Keamanan dari filter user_id = uid sesi.
func (h *Handler) resolveTenantBySlug(ctx context.Context, uid int64, slug string) (db.Tenant, bool) {
	var (
		tenant db.Tenant
		name   string
		found  bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		t, e := q.GetTenantBySlug(ctx, slug)
		if e != nil {
			return nil // slug tak ada — bukan error, cukup tak ditemukan
		}
		if _, e := q.GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: t.ID}); e != nil {
			return nil // bukan anggota — disamakan dgn tak ada (anti kebocoran keberadaan)
		}
		tenant, name, found = t, t.Name, true
		return nil
	})
	if err != nil {
		h.Log.Error("scope: resolve slug", "user_id", uid, "err", err)
		return db.Tenant{}, false
	}
	if found && tenant.ID != session.TenantID(ctx) {
		session.SetActiveTenant(ctx, tenant.ID, name, slug)
	}
	return tenant, found
}

// resolveActiveTenant mengembalikan workspace aktif yang SUDAH TERVALIDASI milik
// user. Alur: pakai tenant di session bila user memang anggotanya; kalau tidak
// (session basi / dipaksa / baru login) → jatuh ke workspace pertama user dan
// simpan sebagai aktif. false bila user tak punya workspace sama sekali.
//
// Dibaca lewat WithSuper karena memberships SENGAJA tanpa RLS: query ini justru
// yang MENENTUKAN tenant — tak bisa bergantung GUC yang belum di-set (chicken-and-
// egg). Keamanan dari filter user_id = uid sesi, bukan RLS.
func (h *Handler) resolveActiveTenant(ctx context.Context, uid int64) (int64, bool) {
	want := session.TenantID(ctx)
	var (
		okTenant int64
		okName   string
		okSlug   string
		found    bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		if want != 0 && session.TenantSlug(ctx) != "" {
			// Slug ikut disyaratkan: session lama (sebelum 0004) menyimpan tenantID
			// tanpa slug. Tanpa cek ini ia lolos sebagai "valid" lalu setiap wsPath
			// menghasilkan /workspace/new — self-heal lewat jalur fallback di bawah.
			if _, e := q.GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: want}); e == nil {
				okTenant, found = want, true
				return nil // session valid — tak perlu query lagi
			}
		}
		ms, e := q.ListMembershipsByUser(ctx, uid)
		if e != nil {
			return e
		}
		if len(ms) > 0 {
			okTenant, okName, okSlug, found = ms[0].TenantID, ms[0].Name, ms[0].Slug, true
		}
		return nil
	})
	if err != nil {
		h.Log.Error("scope: resolve tenant", "user_id", uid, "err", err)
		return 0, false
	}
	if found && okSlug != "" {
		session.SetActiveTenant(ctx, okTenant, okName, okSlug) // fallback → simpan
	}
	return okTenant, found
}

// isPlatformRole melaporkan apakah role = operator platform (bypass RLS).
// super_admin (env) & staff (platform_staff) keduanya lintas-tenant.
func isPlatformRole(role string) bool {
	return role == authz.RoleNameSuperAdmin || role == authz.RoleNameStaff
}
