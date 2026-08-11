package handler

import (
	"context"
	"errors"
	"fmt"

	"go_starter/internal/appmode"
	"go_starter/internal/db"
	"go_starter/internal/settings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// bootstrap.go — keadaan awal aplikasi (keputusan 0006 & 0007). Dijalankan
// SEKALI saat startup, sebelum server melayani request.

// BootstrapPrimary memastikan ada workspace PRIMER — rumah aplikasi — dan
// mengembalikan mode tenancy yang berlaku.
//
// Keduanya sekaligus karena saling terkait: mode dibaca dari DB, dan workspace
// primer adalah satu-satunya workspace di mode single. Memisahkannya berarti dua
// pembacaan yang bisa saling bertentangan.
//
// Yang dikerjakan:
//
//  1. Workspace primer dibuat bila belum ada. Aplikasi jadi tak pernah berada di
//     keadaan "belum ada workspace", sehingga tak perlu jalur khusus untuk user
//     pertama — dan keadaan paling jarang diuji adalah yang paling sering rusak.
//
//  2. Mode dibaca dari platform_settings. Nol baris = single: setiap aplikasi
//     lahir sebagai satu aplikasi, dan multi-tenant adalah sesuatu yang dinaikkan.
//
// Perhatikan yang TIDAK ada di sini lagi: pemeriksaan "mode single tapi DB berisi
// >1 workspace → tolak start". Keadaan itu tak bisa terjadi — untuk punya banyak
// workspace, mode harus sudah naik ke multi, dan trigger DB tak mengizinkannya
// turun kembali. Aturan yang dulu harus diperiksa kini mustahil dilanggar.
func BootstrapPrimary(ctx context.Context, pool *pgxpool.Pool, appName string) (appmode.Mode, error) {
	var mode appmode.Mode
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		primary, err := q.GetPrimaryTenant(ctx)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("baca workspace primer: %w", err)
			}
			// Belum ada → buat. Namanya dari APP_NAME: migrasi tak tahu apa-apa
			// tentang aplikasi di atasnya, jadi ia bukan tempat menaruh seed.
			primary, err = q.CreatePrimaryTenant(ctx, db.CreatePrimaryTenantParams{
				Name: appName, Slug: appmode.PrimarySlug,
			})
			if err != nil {
				return fmt.Errorf("buat workspace primer: %w", err)
			}
		}
		// Seed peran CRM bawaan ke workspace primer. Idempoten (ON CONFLICT DO
		// NOTHING) → aman diulang tiap boot; menutup workspace primer yang lahir
		// di deployment baru (sebelum migrasi 00007 sempat mem-backfill-nya).
		if err := seedBusinessRoles(ctx, q, primary.ID); err != nil {
			return fmt.Errorf("seed peran workspace primer: %w", err)
		}

		s, err := q.GetSetting(ctx, appmode.SettingKey)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				mode = appmode.Single // belum pernah dinaikkan
				return nil
			}
			return fmt.Errorf("baca mode tenancy: %w", err)
		}
		m, ok := appmode.Parse(s.Value)
		if !ok {
			// Nilai terisi tapi tak dikenal = data rusak, bukan keadaan awal.
			// Menjalankan mode yang tak diminta siapa pun lebih buruk daripada
			// menolak start: yang pertama menyembunyikan data secara senyap.
			return fmt.Errorf("nilai %s di platform_settings tak dikenal: %q (harus %q atau %q)",
				appmode.SettingKey, s.Value, appmode.NameSingle, appmode.NameMulti)
		}
		mode = m
		return nil
	})
	return mode, err
}

// UpgradeToMulti menaikkan aplikasi dari satu-app ke multi-tenant. SEKALI JALAN:
// trigger database menolak penurunan, jadi tak ada pasangan DowngradeToSingle —
// dan memang tak boleh ada.
//
// Berlaku SEKETIKA tanpa restart: seluruh route sudah berbentuk /w/{slug} sejak
// mode single, jadi yang berubah hanya apa yang boleh dilihat & dilakukan
// (menu ganti-workspace, tombol buat-workspace muncul). Workspace primer tetap
// di alamat yang sama — nol tautan mati.
func UpgradeToMulti(ctx context.Context, q *db.Queries, actorID int64) error {
	if err := q.UpsertSetting(ctx, db.UpsertSettingParams{
		Key:       appmode.SettingKey,
		Value:     appmode.NameMulti,
		UpdatedBy: &actorID,
	}); err != nil {
		return fmt.Errorf("naikkan mode tenancy: %w", err)
	}
	// DB dulu, cache & state proses kemudian — kalau terbalik, tulis yang gagal
	// meninggalkan aplikasi mengira dirinya multi sementara DB berkata lain.
	settings.Set(appmode.SettingKey, appmode.NameMulti)
	appmode.Set(appmode.Multi)
	return nil
}
