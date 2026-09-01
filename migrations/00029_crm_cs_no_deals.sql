-- +goose Up
-- +goose StatementBegin

-- ═══════════════════════════════════════════════════════════════════════════
-- BL-11 — cabut izin baca Deals dari peran CS (csm) untuk workspace yang SUDAH
-- ADA. Keputusan desain: CS adalah peran pasca-jual; pipeline pra-jual (Deals)
-- bukan wilayahnya. Konteks kontrak pelanggan TAK hilang — CS tetap punya
-- crm:subscriptions read (subscription = cermin pasca-jual dari deal menang:
-- source_deal_id + nilai + plan).
--
-- business_defaults.go (DefaultBusinessRoles) sudah menanggalkan izin ini →
-- berlaku utk workspace BARU. Migrasi ini menyamakan workspace LAMA yang
-- di-seed sebelum BL-11. Enforcer tersegar sendiri saat restart (LoadBusiness
-- membaca DB, business.go). Idempotent: DELETE tanpa error bila baris tak ada.
-- ═══════════════════════════════════════════════════════════════════════════

DELETE FROM business_role_permissions
WHERE role = 'csm' AND obj = 'crm:deals' AND act = 'read';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Kembalikan izin csm→crm:deals→read utk setiap tenant yang masih punya baris
-- csm lain (bukti peran CS ter-seed di tenant itu). ON CONFLICT dijaga index
-- unik (tenant_id, role, obj, act) dari 00007.
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT DISTINCT tenant_id, 'csm', 'crm:deals', 'read'
FROM business_role_permissions
WHERE role = 'csm'
ON CONFLICT DO NOTHING;

-- +goose StatementEnd
