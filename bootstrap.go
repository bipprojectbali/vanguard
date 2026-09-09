// bootstrap.go — helper startup dipisah dari main.go agar tiap file di bawah
// ambang file-health. Isi: bukti isolasi tenant, muat settings & business
// perms, dan konstruksi logger. Dipanggil dari run() (run.go).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	_ "time/tzdata" // embed database tzdata: LoadLocation gagal di container minimal (CGO_ENABLED=0) tanpa ini

	"go_starter/internal/authz"
	"go_starter/internal/config"
	"go_starter/internal/db"
	"go_starter/internal/fls"

	"github.com/jackc/pgx/v5/pgxpool"
)

// verifyTenantIsolation MEMBUKTIKAN isolasi tenant mengikat — dengan bertanya ke
// database, bukan dengan memeriksa env.
//
// Pemeriksaan dilakukan di dalam WithSuper, artinya pada transaksi yang sudah
// menurunkan haknya ke app_rw — persis keadaan setiap query aplikasi. Memeriksa
// pool telanjang akan menjawab pertanyaan yang salah: di sana koneksi memang
// masih owner, dan memang seharusnya (migrasi butuh itu).
//
// Berlaku SAMA di dev maupun production, dan itu perubahan dari sebelumnya.
// Dulu dev sengaja dilonggarkan karena mengikat RLS menuntut role & DSN kedua —
// gesekan yang tak sepadan untuk `make dev`. Dengan SET LOCAL ROLE gesekan itu
// nol, jadi tak ada lagi alasan membiarkan dev berbeda. Justru sebaliknya:
// query yang lupa `WHERE tenant_id` sekarang gagal di laptop, bukan di produksi.
func verifyTenantIsolation(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	var st db.RLSStatus
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		var e error
		st, e = db.CheckRLSTx(ctx, q, "audit_logs")
		return e
	})
	if err != nil {
		return err
	}
	if st.Binds() {
		log.Info("isolasi tenant: RLS mengikat", "role", st.User)
		return nil
	}
	// Tak mengikat = menolak start, tanpa pengecualian. Bukan kekakuan: penyebab
	// yang mungkin tinggal sedikit sejak APP_DATABASE_URL hilang, dan semuanya
	// berarti jaringnya memang tak terpasang.
	return fmt.Errorf("isolasi tenant TIDAK mengikat: %s.\n"+
		"Setiap transaksi aplikasi menurunkan haknya ke app_rw (SET LOCAL ROLE), jadi ini "+
		"berarti role itu tak terpasang benar. Periksa: migrasi sudah jalan sampai selesai? "+
		"`GRANT app_rw TO CURRENT_USER` ada (dibutuhkan bila DATABASE_URL memakai owner "+
		"non-superuser)? FORCE ROW LEVEL SECURITY masih aktif di tabel ber-tenant?",
		st.Reason())
}

// loadSettings membaca seluruh pengaturan platform jadi map siap-cache.
// platform_settings TANPA RLS (pengaturan berlaku lintas-workspace), jadi
// WithSuper aman — dan memang perlu: saat boot belum ada tenant untuk di-scope.
func loadSettings(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	kv := map[string]string{}
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		rows, e := q.ListSettings(ctx)
		if e != nil {
			return e
		}
		for _, s := range rows {
			kv[s.Key] = s.Value
		}
		return nil
	})
	return kv, err
}

// loadBusinessPerms membaca SELURUH izin CRM (semua tenant) untuk membangun
// enforcer bisnis saat startup. WithSuper WAJIB: business_role_permissions ber-RLS,
// jadi di tenant-tx tenant lain tak terbaca → peran mereka deny-all senyap.
// Baris sqlc dipetakan ke authz.BusinessPerm agar paket authz tetap DB-free.
func loadBusinessPerms(ctx context.Context, pool *pgxpool.Pool) ([]authz.BusinessPerm, error) {
	var out []authz.BusinessPerm
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		rows, e := q.ListAllBusinessRolePermissions(ctx)
		if e != nil {
			return e
		}
		out = make([]authz.BusinessPerm, 0, len(rows))
		for _, r := range rows {
			out = append(out, authz.BusinessPerm{
				TenantID: r.TenantID, Role: r.Role, Obj: r.Obj, Act: r.Act,
			})
		}
		return nil
	})
	return out, err
}

// loadFieldSecurity membaca SELURUH kebijakan FLS phone (semua tenant) untuk mengisi
// cache internal/fls saat startup. WithSuper WAJIB dengan alasan sama loadBusinessPerms:
// field_security_policies ber-RLS, jadi di tenant-tx tenant lain tak terbaca. Baris
// sqlc dipetakan ke fls.Row agar paket fls tetap DB-free. Nol baris = semua tenant
// pakai default terkunci (pola code_formats).
func loadFieldSecurity(ctx context.Context, pool *pgxpool.Pool) ([]fls.Row, error) {
	var out []fls.Row
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		rows, e := q.ListAllFieldSecurityPolicies(ctx)
		if e != nil {
			return e
		}
		out = make([]fls.Row, 0, len(rows))
		for _, r := range rows {
			out = append(out, fls.Row{
				TenantID:     r.TenantID,
				BusinessRole: r.BusinessRole,
				CanViewPhone: r.CanViewPhone,
				CanEditPhone: r.CanEditPhone,
			})
		}
		return nil
	})
	return out, err
}

func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
