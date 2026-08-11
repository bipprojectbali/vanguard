package handler

import (
	"context"
	"errors"
	"fmt"

	"go_starter/internal/authz"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5"
)

// business_seed.go — menanam 5 peran CRM bawaan ke workspace yang BARU dibuat.
//
// Kenapa di handler, bukan migrasi: migrasi 00007 mem-backfill workspace yang
// SUDAH ADA saat migrasi jalan; workspace yang dibuat SETELAHNYA (register, buat
// workspace, bootstrap primer) tak tersentuh migrasi itu. Seed ini menutup celah
// tersebut — dipanggil di tx yang SAMA dengan pembuatan tenant agar atomik: tak
// ada workspace tanpa peran (yang akan membuat SEMUA modul CRM-nya deny-default).
//
// SATU sumber kebenaran: authz.DefaultBusinessRoles() — identik dengan backfill
// migrasi (dikunci TestDefaultRolesMatchLegacyCSV). Idempoten via ON CONFLICT DO
// NOTHING, jadi bootstrap primer yang jalan tiap boot aman diulang.
//
// Enforcer TAK di-reload di sini: workspace baru = tenant baru; subject `t<id>:*`
// belum pernah ada di enforcer, dan CanBusiness untuk tenant itu baru dipanggil
// setelah instance melayani request-nya. Instance lain menyusul saat boot
// (loadBusinessPerms). Yang butuh reload seketika hanyalah SUNTINGAN peran
// (ReloadBusinessTenant di panel /roles), bukan seed awal.
func seedBusinessRoles(ctx context.Context, q *db.Queries, tenantID int64) error {
	for _, role := range authz.DefaultBusinessRoles() {
		// CreateBusinessRole = :one dgn ON CONFLICT DO NOTHING RETURNING: pada
		// seed berulang (bootstrap primer tiap boot) baris sudah ada → zero rows
		// → pgx.ErrNoRows. Itu SUKSES idempoten, bukan gagal: peran sudah tertanam,
		// tinggal pastikan izinnya (juga ON CONFLICT DO NOTHING di bawah).
		if _, err := q.CreateBusinessRole(ctx, db.CreateBusinessRoleParams{
			TenantID:    tenantID,
			Name:        role.Name,
			DisplayName: role.DisplayName,
			Description: role.Description,
			DataScope:   role.DataScope,
			IsSystem:    role.IsSystem,
			CreatedBy:   nil, // seed sistem — bukan tindakan seorang user
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("seed peran %q: %w", role.Name, err)
		}
		for _, p := range role.Perms {
			if err := q.InsertBusinessRolePermission(ctx, db.InsertBusinessRolePermissionParams{
				TenantID: tenantID,
				Role:     role.Name,
				Obj:      p.Obj,
				Act:      p.Act,
			}); err != nil {
				return fmt.Errorf("seed izin %q %s/%s: %w", role.Name, p.Obj, p.Act, err)
			}
		}
	}
	return nil
}
