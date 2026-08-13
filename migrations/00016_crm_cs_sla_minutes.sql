-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 6 CUSTOMER SUCCESS (slice A1) — ganti satuan target SLA dari JAM ke
-- MENIT.
--
-- KENAPA: sla_policies dibuat di 00005 dengan first_response_target_hours/
-- resolution_target_hours (INTEGER, satuan JAM). Saat mencocokkan ke wireframe
-- Penpot "CS — 6.11 SLA Management", baris prioritas Kritis menuntut Target
-- Respon = "15 menit" — nilai di bawah 1 jam, tak bisa direpresentasikan bulat
-- dalam satuan jam. Tabel ini masih KOSONG/belum dipakai fitur apa pun sejak
-- 00005 (Modul 6 baru mulai dibangun di slice ini) → rename aman, tanpa
-- backfill data.
--
-- Kolom tetap INTEGER (nullable, sama seperti sebelumnya) — hanya satuan &
-- nama yang berubah. Nilai contoh per prioritas (didokumentasikan, bukan
-- di-seed di sini): Kritis 15/120 · Tinggi 60/480 · Sedang 240/2880 ·
-- Rendah 1440/7200 (menit).
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE sla_policies RENAME COLUMN first_response_target_hours TO first_response_target_minutes;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE sla_policies RENAME COLUMN resolution_target_hours TO resolution_target_minutes;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
ALTER TABLE sla_policies RENAME COLUMN first_response_target_minutes TO first_response_target_hours;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE sla_policies RENAME COLUMN resolution_target_minutes TO resolution_target_hours;
-- +goose StatementEnd
