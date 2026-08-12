-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 5 SUBSCRIPTIONS (slice 3c) — perluas domain `kind` notifikasi.
--
-- KENAPA: renewal Upsell mengirim kabar ke Manager (menunggu persetujuan) dan ke
-- pemilik langganan (disetujui / ditolak). CHECK asli `notifications_kind_check`
-- (00001) hanya mengizinkan kind keanggotaan; kind renewal ditolaknya, dan karena
-- `notify` sengaja fail-soft, kabarnya HILANG tanpa jejak error. CHECK diperluas
-- agar tiga kind renewal sah — bukan dibuang, supaya kind salah-ketik tetap ketahuan.
--
-- SUPERSET WAJIB: CHECK ini DROP-lalu-ADD, jadi menimpa allowlist sebelumnya. Ia
-- berjalan SETELAH 00013_notif_business_role_kind (main) yang menambah
-- 'member.business_role.changed'. Kalau kind itu tak ikut disertakan di sini, ia
-- terhapus dari allowlist → notifikasi peran CRM hilang senyap (jebakan fail-soft
-- yang sama). Maka daftar di bawah = union SEMUA kind sah sampai titik ini.
--
-- Idempotent (DROP ... IF EXISTS lalu ADD) agar aman di-rerun.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN (
        'member.role.changed',
        'member.business_role.changed',
        'member.removed',
        'workspace.joined',
        'renewal.upsell.pending',
        'renewal.approved',
        'renewal.rejected'
    ));
-- +goose StatementEnd

-- +goose Down

-- Down memulihkan ke keadaan SEBELUM migrasi ini = allowlist pasca
-- 00013_notif_business_role_kind (tanpa tiga kind renewal, tapi tetap dgn
-- 'member.business_role.changed').
-- +goose StatementBegin
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN (
        'member.role.changed',
        'member.business_role.changed',
        'member.removed',
        'workspace.joined'
    ));
-- +goose StatementEnd
