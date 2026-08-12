-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- KIND NOTIFIKASI BARU: member.business_role.changed
--
-- Penugasan peran CRM (sumbu bisnis) di /w/{slug}/members kini memberi tahu
-- anggota yang bersangkutan — cermin member.role.changed di sumbu tenant.
-- notifications.kind dijaga CHECK allowlist (00001): kind di luar daftar ditolak
-- DB, dan karena notify() fail-soft, penolakan itu HILANG SENYAP (aksi utama
-- lolos, kabarnya tak pernah tertulis). Tambahkan kind baru ke allowlist agar
-- notifikasi peran CRM benar-benar tersimpan.
--
-- Drop-lalu-add (bukan ALTER … ADD): CHECK constraint tak punya bentuk "tambah
-- nilai". Idempoten via IF EXISTS pada drop + nama constraint tetap sama.
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
        'workspace.joined'
    ));
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN (
        'member.role.changed',
        'member.removed',
        'workspace.joined'
    ));
-- +goose StatementEnd
