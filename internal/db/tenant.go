// tenant.go — helper RLS ditulis-tangan (BUKAN generated sqlc; sqlc tak menyentuh
// file ini). Menyediakan dua jalur eksekusi: WithTenant (scoped ke satu tenant) &
// WithSuper (bypass platform). Keduanya membungkus SATU transaksi dengan GUC
// transaction-local agar RLS Postgres (policy tenant_isolation) menggerakkan
// isolasi di level DB — "lupa WHERE tenant_id" tetap 0 baris, bukan kebocoran.
package db

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GUC transaction-local yang dibaca policy RLS tenant_isolation.
const (
	gucTenantID = "app.tenant_id" // scope: hanya baris tenant ini yang terlihat
	gucIsSuper  = "app.is_super"  // "on" = bypass RLS (jalur platform)
)

// runtimeRole = role least-privilege yang dimasuki setiap transaksi aplikasi.
// NOLOGIN: tak pernah dipakai untuk connect, hanya dimasuki lewat SET LOCAL ROLE.
const runtimeRole = "app_rw"

// dropPrivileges menurunkan hak transaksi ini ke app_rw (NOBYPASSRLS).
//
// INILAH yang membuat satu DSN cukup. Dulu butuh dua: DATABASE_URL (owner, untuk
// migrasi yang perlu ALTER/CREATE POLICY) dan APP_DATABASE_URL (app_rw, agar RLS
// mengikat) — sebab owner & superuser SELALU bypass RLS, bahkan dengan FORCE.
// Konsekuensinya satu env lagi yang bisa lupa diisi, satu password lagi yang bisa
// bocor, satu entri lagi di userlist.txt PgBouncer.
//
// SET LOCAL ROLE mencapai hasil yang sama tanpa koneksi kedua. Terverifikasi di
// Postgres 17:
//
//   - di dalam tx, current_user = app_rw dan rolsuper ikut turun jadi false;
//   - hak pulih otomatis saat COMMIT *maupun* ROLLBACK — transaksi berikutnya di
//     koneksi yang sama kembali sebagai owner, jadi tak ada yang bocor ke
//     peminjam pool berikutnya (kebocoran #1 pada pooling transaksi);
//   - app.is_super='on' tetap bypass policy, jadi jalur platform (/dev) utuh;
//   - CREATE TABLE ditolak — DDL tetap milik jalur migrasi;
//   - biayanya ~5 µs per transaksi.
//
// Efek samping yang justru diinginkan: RLS kini mengikat di DEV juga. Query yang
// lupa `WHERE tenant_id` gagal di laptop, bukan di produksi.
//
// Yang HILANG dibanding koneksi app_rw sungguhan: SQL injection yang berhasil
// bisa memanggil RESET ROLE dan kembali jadi owner. Diterima secara sadar —
// seluruh query digenerate sqlc (berparameter), dan tak ada jalur SQL mentah
// dari input user. Kalau kelak ada, timbang ulang keputusan ini.
func dropPrivileges(ctx context.Context, tx pgx.Tx) error {
	// Identifier tak bisa diparameterkan; runtimeRole konstanta, bukan input.
	_, err := tx.Exec(ctx, "SET LOCAL ROLE "+runtimeRole)
	return err
}

// WithTenant menjalankan fn dalam SATU transaksi yang di-scope ke tenantID via RLS.
// set_config(...,true) = TRANSACTION-LOCAL → GUC tak bocor ke peminjam pool
// berikutnya (gotcha pool-leak — kebocoran tenant #1 paling umum). fn error / panic
// → rollback; fn nil → commit.
func WithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID int64, fn func(*Queries) error) error {
	return withTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", gucTenantID, strconv.FormatInt(tenantID, 10)); err != nil {
			return err
		}
		if err := dropPrivileges(ctx, tx); err != nil {
			return err
		}
		return fn(New(tx))
	})
}

// WithSuper menjalankan fn di jalur PLATFORM (bypass RLS): set app.is_super='on'.
// HANYA dari jalur terverifikasi — auth pre-identity (tenant belum diketahui), boot
// reconcile, super_admin(env)/staff terverifikasi. JANGAN PERNAH dipicu role DB
// (anti privilege-escalation): keputusan bypass diambil di Go, bukan dari data.
func WithSuper(ctx context.Context, pool *pgxpool.Pool, fn func(*Queries) error) error {
	return withTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, 'on', true)", gucIsSuper); err != nil {
			return err
		}
		// Hak tetap DITURUNKAN meski ini jalur platform: bypass-nya datang dari GUC
		// app.is_super (keputusan yang diambil di Go), bukan dari privilege role.
		// Dengan begitu jalur platform tetap tak bisa DDL, dan bila policy kelak
		// diubah, ia ikut terikat aturan barunya.
		if err := dropPrivileges(ctx, tx); err != nil {
			return err
		}
		return fn(New(tx))
	})
}

// withTx: buka tx, jalankan fn, commit bila nil / rollback bila error. defer
// Rollback = no-op setelah Commit sukses (pola pgx idiomatik).
func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op bila sudah Commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WithSavepoint menjalankan fn dalam SAVEPOINT bersarang di ATAS transaksi ber-scope
// yang sedang berjalan (q berasal dari h.q(ctx) — WithTenant/WithSuper). Gunanya:
// aksi SEKUNDER yang boleh GAGAL tanpa menyeret transaksi utama. Di arsitektur satu-tx
// (Scope selalu commit), sebuah query yang gagal MERACUNI tx → commit berakhir rollback
// → SELURUH request batal. Savepoint mengurung kegagalan: fn error → ROLLBACK TO
// SAVEPOINT (tx luar tetap sehat, pemanggil boleh lanjut fail-soft); fn nil → RELEASE.
// GUC & SET LOCAL ROLE tetap berlaku (koneksi & tx sama) → RLS tetap mengikat.
//
// q.db bukan pgx.Tx (mis. dipanggil di luar jalur ber-tx) → jalankan fn apa adanya
// tanpa savepoint (tak ada tx untuk diurung).
func WithSavepoint(ctx context.Context, q *Queries, fn func(*Queries) error) error {
	tx, ok := q.db.(pgx.Tx)
	if !ok {
		return fn(q)
	}
	sp, err := tx.Begin(ctx) // pgx: nested tx = SAVEPOINT
	if err != nil {
		return err
	}
	if err := fn(New(sp)); err != nil {
		_ = sp.Rollback(ctx) // ROLLBACK TO SAVEPOINT — tx luar selamat
		return err
	}
	return sp.Commit(ctx) // RELEASE SAVEPOINT
}
