package handler

import (
	"context"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
)

// signup_tenant.go — penempatan pendaftar baru ke workspace. Dipakai register
// password DAN OAuth, keduanya di dalam tx yang sudah dibuka pemanggil.
//
// Ada karena kedua jalur itu dulu punya salinan logika yang sama: buat tenant,
// buat membership owner. Di mode single (0006 §6) aturannya berbeda TOTAL, dan
// dua salinan berarti satu di antaranya pasti terlewat — dengan akibat yang
// tidak kentara: setiap orang yang mendaftar lewat jalur yang terlewat menjadi
// OWNER aplikasi.

// placeNewUser menempatkan user yang baru mendaftar dan mengembalikan tenant
// tempat ia bernaung.
//
//	multi  → workspace BARU, pendaftar jadi owner (ia memang pemiliknya).
//	single → GABUNG ke aplikasi tunggal sebagai member — TIDAK membuat apa pun,
//	         dan TIDAK jadi owner.
//
// Kenapa member di mode single: aplikasi tunggal dimiliki operator, bukan
// pendaftar. Kalau pendaftar jadi owner, siapa pun yang menekan tombol daftar
// memperoleh kendali penuh. Pengelola ditentukan SUPER_ADMIN_EMAILS (env) —
// konsisten dengan 0002: otoritas tertinggi datang dari env, tak pernah dari
// pendaftaran. Menaikkan seseorang jadi admin tetap lewat panel.
//
// wantName dipakai HANYA di mode multi (nama workspace yang diminta pendaftar);
// di mode single ia diabaikan — aplikasinya sudah ada dan bernama.
func placeNewUser(ctx context.Context, q *db.Queries, userID int64, wantName string) (db.Tenant, error) {
	if appmode.IsSingle() {
		t, err := q.GetPrimaryTenant(ctx)
		if err != nil {
			// Workspace primer dibuat saat startup (BootstrapPrimary), jadi absennya
			// di sini = bug wiring, bukan keadaan yang perlu ditangani diam-diam.
			return db.Tenant{}, err
		}
		if _, err := q.CreateMembership(ctx, db.CreateMembershipParams{
			UserID: userID, TenantID: t.ID, Role: authz.RoleNameMember,
		}); err != nil {
			return db.Tenant{}, err
		}
		return t, nil
	}

	slug, err := uniqueSlug(ctx, q, slugify(wantName))
	if err != nil {
		return db.Tenant{}, err
	}
	t, err := q.CreateTenant(ctx, db.CreateTenantParams{Name: wantName, Slug: slug})
	if err != nil {
		return db.Tenant{}, err
	}
	if _, err := q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: userID, TenantID: t.ID, Role: authz.RoleNameOwner,
	}); err != nil {
		return db.Tenant{}, err
	}
	// Peran CRM bawaan, di tx yang sama → atomik: workspace tak pernah lahir tanpa
	// perannya. (Cabang single tak menyeed: workspace primer sudah di-seed boot.)
	if err := seedBusinessRoles(ctx, q, t.ID); err != nil {
		return db.Tenant{}, err
	}
	return t, nil
}
