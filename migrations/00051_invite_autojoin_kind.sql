-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- BL-170: UNDANG ANGGOTA → AUTO-JOIN BY EMAIL + JENIS ANGGOTA
--
-- Akseptasi undangan pindah dari klik-link ke auto-match email saat
-- login/register (jalur utama baru); link/token TETAP hidup di backend
-- (UI-nya saja yang disembunyikan). Tambahan sumbu baru "kind"
-- (internal/eksternal) di tiga tabel:
--
--   business_roles.kind — peran CRM ini utk anggota internal atau eksternal
--                          (filter cascading select di form Undang).
--   invites.kind         — jenis yang DIPILIH admin saat mengundang, plus
--                          invites.business_role (peran CRM yang sudah
--                          ditentukan di muka, diterapkan otomatis saat invite
--                          diterima).
--   memberships.kind      — jenis anggota AKTUAL, independen dari Peran CRM,
--                          bisa diubah kapan pun di panel Anggota.
--
-- Semua kolom baru ber-DEFAULT 'internal' → baris lama otomatis konsisten
-- tanpa UPDATE eksplisit (BL-105/organic join path tetap kind='internal',
-- nol perubahan kode di sana).
--
-- invites.business_role TANPA FK — presedent memberships.business_role sejak
-- 00007: peran kustom per-tenant, tak bisa divalidasi CHECK statis, divalidasi
-- app-layer (Go). Menghapus sebuah peran tak boleh mencabut invite yang
-- menunjuknya; app-layer yang menentukan apa terjadi pada invite pending.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE business_roles ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'internal';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE business_roles DROP CONSTRAINT IF EXISTS business_roles_kind_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE business_roles ADD CONSTRAINT business_roles_kind_chk
    CHECK (kind IN ('internal','external'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE invites ADD COLUMN IF NOT EXISTS business_role TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE invites ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'internal';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE invites DROP CONSTRAINT IF EXISTS invites_kind_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE invites ADD CONSTRAINT invites_kind_chk
    CHECK (kind IN ('internal','external'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE memberships ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'internal';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE memberships DROP CONSTRAINT IF EXISTS memberships_kind_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE memberships ADD CONSTRAINT memberships_kind_chk
    CHECK (kind IN ('internal','external'));
-- +goose StatementEnd

-- notifications_kind_check (00013/00015/00047) diperluas lagi: kind baru
-- 'member.kind.changed' utk penanda perubahan Jenis Anggota (cermin
-- member.business_role.changed). SUPERSET WAJIB (union sampai 00047 + baris
-- baru ini) — pola sama migrasi kind-notifikasi sebelumnya.
-- +goose StatementBegin
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_kind_check;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN (
        'member.role.changed',
        'member.business_role.changed',
        'member.kind.changed',
        'member.removed',
        'workspace.joined',
        'renewal.upsell.pending',
        'renewal.approved',
        'renewal.rejected',
        'subscription.renewal.reminder'
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
        'member.business_role.changed',
        'member.removed',
        'workspace.joined',
        'renewal.upsell.pending',
        'renewal.approved',
        'renewal.rejected',
        'subscription.renewal.reminder'
    ));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE memberships DROP CONSTRAINT IF EXISTS memberships_kind_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE memberships DROP COLUMN IF EXISTS kind;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE invites DROP CONSTRAINT IF EXISTS invites_kind_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE invites DROP COLUMN IF EXISTS kind;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE invites DROP COLUMN IF EXISTS business_role;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE business_roles DROP CONSTRAINT IF EXISTS business_roles_kind_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE business_roles DROP COLUMN IF EXISTS kind;
-- +goose StatementEnd
