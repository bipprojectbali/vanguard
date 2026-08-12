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
-- Idempotent (DROP ... IF EXISTS lalu ADD) agar aman di-rerun.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN (
        'member.role.changed',
        'member.removed',
        'workspace.joined',
        'renewal.upsell.pending',
        'renewal.approved',
        'renewal.rejected'
    ));
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('member.role.changed','member.removed','workspace.joined'));
-- +goose StatementEnd
