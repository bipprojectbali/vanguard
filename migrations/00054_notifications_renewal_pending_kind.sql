-- 00054_notifications_renewal_pending_kind.sql — tambah kind 'renewal.pending'
-- ke allowlist notifications_kind_check (celah dari BL-172).
--
-- KENAPA: BL-172 menyatukan notifikasi renewal Upsell & Downgrade jadi SATU
-- kind generik "renewal.pending" (context-aware via payload Direction, lihat
-- notifyManagersRenewalPending di subscriptions_action.go) — menggantikan kind
-- lama "renewal.upsell.pending" (00015) sebagai kind yang benar-benar DIKIRIM.
-- Tapi migrasi allowlist-nya tak pernah dibuat: CHECK (00047) masih hanya
-- mengizinkan 'renewal.upsell.pending', jadi setiap notify(..., "renewal.pending",
-- ...) DITOLAK DB — dan karena notify() sengaja fail-soft (notifications_notify.go),
-- penolakan itu HILANG SENYAP tanpa membatalkan renewal (jebakan yang sama
-- didokumentasikan di 00013/00015). Manager tak pernah menerima notifikasi
-- renewal Upsell/Downgrade sejak BL-172 digabung.
--
-- 'renewal.upsell.pending' TETAP dipertahankan di allowlist (SUPERSET WAJIB,
-- pola sama 00015/00047) — notifications_service.go masih menanganinya utk
-- baris historis lama, dan menghapusnya dari CHECK akan menolak ALTER bila ada
-- baris produksi lama bernilai itu.
--
-- Union dihitung dari 00051 (allowlist TERAKHIR, sudah termasuk
-- 'member.kind.changed' BL-170) — BUKAN dari 00047. Salah hitung union dari
-- 00047 di draf awal migrasi ini sempat DIAM-DIAM menghapus 'member.kind.changed'
-- dari allowlist (regresi terdeteksi via TestMemberKind_ChangedByManager sebelum
-- migrasi ini di-push/diterapkan produksi).

-- +goose Up
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
        'subscription.renewal.reminder',
        'renewal.pending'
    ));
-- +goose StatementEnd

-- +goose Down

-- Down memulihkan ke keadaan SEBELUM migrasi ini = allowlist persis 00051
-- (tanpa 'renewal.pending').
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
