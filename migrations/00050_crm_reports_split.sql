-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- BL-169 — pecah objek Casbin "crm:reports" (1 gerbang) jadi 4 objek sesuai
-- 4 halaman Reports nyata di sidebar: crm:reports_sales, crm:reports_cs,
-- crm:reports_support, crm:reports_subscriptions.
--
-- KENAPA: sidebar & routing sudah 4 halaman terpisah (Sales/CS/Support/
-- Subscription Reports, masing-masing handler & route sendiri), tapi matriks
-- /roles cuma punya satu baris "Reports" (crm:reports) — admin tak bisa
-- memberi Sales akses ke Sales Report saja tanpa ikut membuka Support Report.
--
-- MIGRASI DATA: backfill (bukan pindah 1:1) — tiap baris (tenant, role,
-- crm:reports, act) disalin jadi 4 baris (satu per objek baru), lalu baris
-- lama dihapus. Ini keputusan sadar (bukan default): grant crm:reports lama
-- berarti "lihat SEMUA preset report", jadi tenant yang sudah memberi akses
-- Reports ke suatu role TAK kehilangan cakupan apa pun saat migrasi — mereka
-- cuma mendapat 4 baris granular yang setara dengan 1 baris lama gabungan.
-- Admin bebas mempersempit lewat editor /roles SETELAH migrasi bila mau.
--
-- business_defaults.go (DefaultBusinessRoles, crmModules) sudah dipindah →
-- berlaku utk workspace BARU. Migrasi ini menyamakan workspace LAMA — bukan
-- cuma default Manager/Sales/CSM/Support, tapi SEMUA baris obj=crm:reports
-- apa pun rolenya (termasuk role custom yang sudah disunting tenant).
--
-- ON CONFLICT DO NOTHING menjaga index unik (tenant_id, role, obj, act, 00007)
-- bila suatu tenant kebetulan sudah punya baris di salah satu objek baru
-- (tak mungkin hari ini, objek ini baru) — aman bila migrasi dijalankan ulang.
-- Enforcer tersegar sendiri saat restart (LoadBusiness membaca DB, business.go).
-- Idempoten.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT tenant_id, role, 'crm:reports_sales', act
FROM business_role_permissions
WHERE obj = 'crm:reports'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT tenant_id, role, 'crm:reports_cs', act
FROM business_role_permissions
WHERE obj = 'crm:reports'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT tenant_id, role, 'crm:reports_support', act
FROM business_role_permissions
WHERE obj = 'crm:reports'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT tenant_id, role, 'crm:reports_subscriptions', act
FROM business_role_permissions
WHERE obj = 'crm:reports'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM business_role_permissions
WHERE obj = 'crm:reports';
-- +goose StatementEnd

-- +goose Down

-- Down memulihkan baris (tenant, role, crm:reports, act) dari salah satu dari
-- 4 objek baru — dipilih crm:reports_sales karena hasil backfill Up membuat
-- keempatnya identik (baris yang sama disalin ke semuanya); tak bisa
-- membedakan baris yang murni pindahan dari baris yang mungkin disunting
-- BERBEDA per-objek SETELAH migrasi ini (kasus langka, sama seperti
-- keterbatasan Down 00049) — diterima sebagai batas wajar migrasi data.
-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT tenant_id, role, 'crm:reports', act
FROM business_role_permissions
WHERE obj = 'crm:reports_sales'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM business_role_permissions
WHERE obj IN (
	'crm:reports_sales', 'crm:reports_cs',
	'crm:reports_support', 'crm:reports_subscriptions'
);
-- +goose StatementEnd
