-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 5 SUBSCRIPTIONS (slice 3c) — persetujuan renewal Upsell.
--
-- KENAPA: renewal yang menaikkan nilai (Upsell: MRR baru > previous_value) tak
-- langsung aktif — ia butuh persetujuan Manager. Baris renewal lahir berstatus
-- 'PendingApproval' dan HIDUP berdampingan dengan baris lama yang masih 'Active'
-- (idx_subs_one_active HANYA menjaga status='Active', jadi satu Pending + satu
-- Active tak bertabrakan). Setelah Manager memutuskan: approve → baris lama
-- di-Expired lalu baris baru jadi 'Active'; reject → baris baru jadi 'Cancelled'.
-- Kolom approval_* mencatat SIAPA & KAPAN memutuskan (jejak audit di baris).
--
-- Aditif & nullable → tanpa backfill: baris lama & jalur non-approval (straight
-- renewal, create-from-deal) memakai approval_status NULL. Idempotent (guard
-- IF NOT EXISTS / DROP IF EXISTS) agar aman di-rerun.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS approval_status TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS approved_by BIGINT REFERENCES users(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;
-- +goose StatementEnd

-- Domain approval_status (nullable → NULL ATAU salah satu nilai). Guard drop+add
-- agar rerun aman.
-- +goose StatementBegin
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subs_approval_status_chk;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE subscriptions ADD CONSTRAINT subs_approval_status_chk CHECK (
    approval_status IS NULL OR approval_status IN ('Pending', 'Approved', 'Rejected'));
-- +goose StatementEnd

-- Perluas subs_status_chk dengan 'PendingApproval' (baris renewal Upsell menunggu
-- keputusan Manager). Drop+add penuh karena ADD tak bisa memodifikasi CHECK ada.
-- +goose StatementBegin
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subs_status_chk;
-- +goose StatementEnd
-- 'Suspended' = nilai CADANGAN, BELUM di-wire (BL-22): tak ada aksi app yang
-- menghasilkannya, tak di-seed, tak ditawarkan di dropdown filter. Enum sengaja
-- tetap menerima nilai ini (pintu masa depan). Untuk tunggakan pakai
-- payment_status='Overdue'; untuk penghentian pakai churn_type='Involuntary'.
-- +goose StatementBegin
ALTER TABLE subscriptions ADD CONSTRAINT subs_status_chk CHECK (
    status IN ('Trial', 'Active', 'Suspended', 'Expired', 'Cancelled', 'Churned', 'PendingApproval'));
-- +goose StatementEnd


-- +goose Down

-- Kembalikan subs_status_chk TANPA 'PendingApproval' (baris Pending harus sudah
-- tuntas sebelum rollback; bila masih ada, ADD akan gagal — itu disengaja agar
-- data tak jadi ilegal secara diam-diam).
-- +goose StatementBegin
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subs_status_chk;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE subscriptions ADD CONSTRAINT subs_status_chk CHECK (
    status IN ('Trial', 'Active', 'Suspended', 'Expired', 'Cancelled', 'Churned'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subs_approval_status_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE subscriptions DROP COLUMN IF EXISTS approved_at;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE subscriptions DROP COLUMN IF EXISTS approved_by;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE subscriptions DROP COLUMN IF EXISTS approval_status;
-- +goose StatementEnd
