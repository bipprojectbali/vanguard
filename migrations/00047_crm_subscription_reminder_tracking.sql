-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- BL-158: notifikasi terjadwal masa perpanjangan subscription (H-30/H-14/H-7/
-- hari-H) — kolom penanda anti-dobel + kind notifikasi baru.
--
-- KENAPA: task maintenance baru (subscription_renewal_reminders) berjalan
-- harian dan mengevaluasi ULANG sisa-hari tiap siklus; tanpa penanda, siklus
-- kedua di hari yang sama (atau restart proses) akan mengirim notifikasi
-- dobel untuk ambang yang sama. last_reminder_days_sent menyimpan ambang
-- TERAKHIR yang sudah terkirim per subscription (keputusan user: kolom baru,
-- bukan EXISTS-check ke tabel notifications — lebih murah & eksplisit).
-- NULL = belum pernah dikirim reminder sama sekali.
--
-- notifications_kind_check (00015) juga diperluas: kind baru
-- 'subscription.renewal.reminder' ditambahkan. SUPERSET WAJIB (pola sama
-- 00015): DROP-lalu-ADD menimpa allowlist sebelumnya, jadi daftar di bawah =
-- union SEMUA kind sah sampai titik ini (union dgn 00015, tak ada kind baru
-- lain masuk sejak itu).
--
-- CATATAN OUT-OF-SCOPE (ditemukan saat riset, TIDAK diperbaiki di sini):
-- 'subscription.created.from_deal' dipakai di sales_deals_won_notify.go dan
-- ditangani notifText(), tapi TIDAK PERNAH masuk CHECK manapun sejak 00015 —
-- notifikasi itu kemungkinan silent-fail (fail-soft notify() menelan error
-- CHECK violation tanpa menggagalkan aksi utama). Dibiarkan agar tak
-- memperluas scope BL-158 secara diam-diam; dilaporkan terpisah ke user.
--
-- Idempoten: ADD COLUMN IF NOT EXISTS, CHECK DROP IF EXISTS lalu ADD.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS last_reminder_days_sent INTEGER;
-- +goose StatementEnd

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

-- +goose Down

-- Down memulihkan ke keadaan SEBELUM migrasi ini = allowlist persis 00015
-- (tanpa 'subscription.renewal.reminder').
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

-- +goose StatementBegin
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_reminder_days_sent;
-- +goose StatementEnd
