-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- BL-145 subtask 2 — pindahkan kapabilitas approve renewal dari objek Casbin
-- "crm:renewal_mgmt" ke "crm:renewals" untuk workspace yang SUDAH ADA.
--
-- KENAPA: tombol Setujui/Reject renewal upsell (SubscriptionRenewApprove/
-- Reject) HANYA pernah muncul di halaman Subscription detail (dunia
-- Renewals/Subscriptions) — NOL kemunculan di halaman Renewal Management
-- manapun (verifikasi UI nyata, bukan cuma nama objek Casbin lama, lihat
-- memory bl-145-roles-redesign.md). Kolom "Setujui" nempel di baris "Renewal
-- Management" pada matriks /roles jadi menyesatkan admin yang mengonfigurasi
-- peran — mencentangnya TAK membuka apa pun di halaman itu.
--
-- business_defaults.go (DefaultBusinessRoles, ModuleDef) sudah dipindah →
-- berlaku utk workspace BARU. Migrasi ini menyamakan workspace LAMA: pindahkan
-- SEMUA baris (tenant, role, crm:renewal_mgmt, approve) — bukan cuma Manager
-- default — krn tenant bisa saja sudah menyunting peran custom lewat UI dan
-- menambah approve di renewal_mgmt utk role lain. crm:renewal_mgmt TETAP hidup
-- untuk read/write (gate halaman Renewal Management CS itu sendiri) — hanya
-- act approve yang pindah rumah objek.
--
-- ON CONFLICT DO NOTHING menjaga index unik (tenant_id, role, obj, act, 00007)
-- bila suatu tenant kebetulan sudah punya baris (crm:renewals, approve) —
-- tak mungkin hari ini (act itu belum pernah dipakai di objek ini), tapi aman
-- bila migrasi dijalankan ulang. Enforcer tersegar sendiri saat restart
-- (LoadBusiness membaca DB, business.go). Idempoten.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT tenant_id, role, 'crm:renewals', 'approve'
FROM business_role_permissions
WHERE obj = 'crm:renewal_mgmt' AND act = 'approve'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM business_role_permissions
WHERE obj = 'crm:renewal_mgmt' AND act = 'approve';
-- +goose StatementEnd

-- +goose Down

-- Down memulihkan baris (tenant, role, crm:renewal_mgmt, approve) dari
-- (tenant, role, crm:renewals, approve) — kembali ke keadaan sebelum BL-145
-- subtask 2. Tak bisa membedakan baris "approve" yang murni pindahan dari
-- baris "approve" yang mungkin ditambah manual SETELAH migrasi ini (kasus
-- langka, sama seperti keterbatasan Down 00029) — diterima sebagai batas wajar
-- migrasi data.
-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT tenant_id, role, 'crm:renewal_mgmt', 'approve'
FROM business_role_permissions
WHERE obj = 'crm:renewals' AND act = 'approve'
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM business_role_permissions
WHERE obj = 'crm:renewals' AND act = 'approve';
-- +goose StatementEnd
