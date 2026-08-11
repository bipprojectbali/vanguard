-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- RBAC BISNIS yang bisa DIEDIT per-workspace (sumbu F2 + cakupan data F3).
--
-- Sampai 00005, business_role (admin/manager/sales/csm/support) FIXED: policy
-- di CSV embed, enforcer in-memory, CHECK constraint, dan konstanta Go — empat
-- salinan nama yang sama. Untuk template yang di-clone tiap workspace, itu kaku:
-- satu desa tak bisa mendefinisikan peran sendiri (mis. "Kepala Cabang" yang
-- hanya melihat desa binaannya).
--
-- Migrasi ini memindahkan policy ke DUA tabel tenant-scoped supaya bisa diubah
-- saat aplikasi berjalan (reload per-tenant, tanpa restart):
--
--   business_roles              — satu baris per peran per workspace. data_scope
--                                 (all/own/none) = cakupan desa F3, kini EDITABLE
--                                 per-peran alih-alih ditebak dari nama.
--   business_role_permissions   — baris (role,obj,act), 1:1 dengan p-rule Casbin.
--                                 Di-load ke enforcer sebagai `t<tenant>:<role>`.
--
-- Kenapa dua tabel, bukan satu kolom jsonb: tiap baris memetakan langsung ke
-- AddPolicies Casbin, mempertahankan semantik glob `crm:*` & matcher yang sudah
-- ada — matriks admin jadi SELECT biasa, bukan parsing jsonb di dalam loader.
--
-- Kenapa BUKAN composite FK dari memberships.business_role: menghapus sebuah
-- peran yang masih dipegang anggota harus MENG-UNASSIGN mereka (business_role
-- jadi NULL), bukan cascade-menghapus keanggotaannya. Validasi nama pindah ke
-- app-layer (ValidBusinessRole tenant-aware). CHECK 5-nama lama di-DROP.
-- ═══════════════════════════════════════════════════════════════════════════


-- ── business_roles — definisi peran CRM per-workspace ──────────────────────
-- UNIQUE(tenant_id, name): nama peran unik dalam satu workspace, tapi dua
-- workspace boleh sama-sama punya "sales" (isolasi tetap lewat tenant_id + RLS).
-- Ia juga TARGET FK dari business_role_permissions, jadi butuh index unik atasnya.
--
-- data_scope = cakupan baris desa (F3). Default 'none' = fail-closed: peran baru
-- yang lupa diberi cakupan melihat NOL desa, bukan semua. is_system = peran bawaan
-- (admin) yang tak bisa diubah/dihapus — kalau bisa, admin dapat men-strip crm:*
-- dari dirinya sendiri lalu terkunci keluar dari manajemen peran.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS business_roles (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id    BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    display_name TEXT NOT NULL,
    data_scope   TEXT NOT NULL DEFAULT 'none',
    is_system    BOOLEAN NOT NULL DEFAULT false,
    created_by   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT business_roles_scope_chk CHECK (data_scope IN ('all','own','none')),
    CONSTRAINT business_roles_tenant_name_key UNIQUE (tenant_id, name)
);
-- +goose StatementEnd


-- ── business_role_permissions — matriks (role,obj,act) ─────────────────────
-- Satu baris = satu p-rule Casbin. FK komposit ke (tenant_id, name): menghapus
-- peran mencabut seluruh izinnya (CASCADE), dan izin tak bisa menunjuk peran
-- yang tak ada di workspace-nya. UNIQUE mencegah baris ganda.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS business_role_permissions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    obj        TEXT NOT NULL,
    act        TEXT NOT NULL,
    CONSTRAINT business_role_perms_uniq UNIQUE (tenant_id, role, obj, act),
    CONSTRAINT business_role_perms_role_fk
        FOREIGN KEY (tenant_id, role) REFERENCES business_roles (tenant_id, name)
        ON DELETE CASCADE
);
-- +goose StatementEnd

-- Loader startup memindai per-tenant lalu memfilter; index (tenant_id, role)
-- melayani baik reload-per-tenant maupun startup load-all.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_business_role_perms ON business_role_permissions (tenant_id, role);
-- +goose StatementEnd


-- ── DROP CHECK 5-nama lama ─────────────────────────────────────────────────
-- memberships_business_role_chk (00005) mengunci business_role ke 5 nama harfiah.
-- Dengan peran custom per-workspace, validasi nama tak bisa lagi statis — pindah
-- ke ValidBusinessRole tenant-aware (lookup business_roles). Nilai existing sudah
-- di antara 5 nama itu, jadi men-DROP CHECK tak menyentuh data apa pun.
-- +goose StatementBegin
ALTER TABLE memberships DROP CONSTRAINT IF EXISTS memberships_business_role_chk;
-- +goose StatementEnd


-- ── RLS — per tabel (template dua-GUC identik 00005) ───────────────────────
-- +goose StatementBegin
ALTER TABLE business_roles            ENABLE ROW LEVEL SECURITY;
ALTER TABLE business_roles            FORCE  ROW LEVEL SECURITY;
ALTER TABLE business_role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE business_role_permissions FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
-- Policy dibungkus DO/IF NOT EXISTS: CREATE POLICY tak punya IF NOT EXISTS, jadi
-- migrasi aman diulang (gaya 00005 di-guard pg_policy).
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policy WHERE polname = 'tenant_isolation'
                   AND polrelid = 'business_roles'::regclass) THEN
        CREATE POLICY tenant_isolation ON business_roles
            USING (COALESCE(current_setting('app.is_super', true),'off')='on'
                   OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
            WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
                   OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policy WHERE polname = 'tenant_isolation'
                   AND polrelid = 'business_role_permissions'::regclass) THEN
        CREATE POLICY tenant_isolation ON business_role_permissions
            USING (COALESCE(current_setting('app.is_super', true),'off')='on'
                   OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
            WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
                   OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
    END IF;
END $$;
-- +goose StatementEnd


-- ── Backfill idempotent — 5 peran default untuk SETIAP tenant existing ──────
-- Peran bawaan + matriks izin diturunkan dari business_policy.csv (kini
-- DefaultBusinessRoles() di Go). Di sini di-materialkan untuk workspace yang
-- SUDAH ADA sebelum fitur ini; workspace baru di-seed handler saat dibuat.
--
-- data_scope: admin/manager='all', sales/csm='own', support='none' — cerminan
-- AccountsScopeFor lama, kini persisted & editable. ON CONFLICT DO NOTHING: aman
-- diulang, dan tak menimpa peran yang mungkin sudah diubah operator.
--
-- Baris dijalankan sebagai owner migrasi (bypass RLS via FORCE? tidak — owner
-- non-super tetap terkena FORCE). Migrasi jalan sebagai superuser/owner DB:
-- INSERT lolos karena app.is_super tak relevan bagi superuser, dan bagi owner
-- non-super policy tenant_isolation di atas mengizinkan lewat cabang is_super.
-- Agar pasti lolos di kedua kasus, set GUC super untuk transaksi backfill ini.
-- +goose StatementBegin
SET LOCAL app.is_super = 'on';
-- +goose StatementEnd

-- +goose StatementBegin
-- Peran + cakupan. VALUES = (name, display, scope, is_system).
INSERT INTO business_roles (tenant_id, name, display_name, data_scope, is_system)
SELECT t.id, d.name, d.display_name, d.data_scope, d.is_system
FROM tenants t
CROSS JOIN (VALUES
    ('admin',   'Administrator', 'all',  true),
    ('manager', 'Manager',       'all',  false),
    ('sales',   'Sales',         'own',  false),
    ('csm',     'Customer Success Manager', 'own',  false),
    ('support', 'Support',       'none', false)
) AS d(name, display_name, data_scope, is_system)
ON CONFLICT (tenant_id, name) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
-- Matriks izin. VALUES = (role, obj, act) — salinan langsung business_policy.csv.
-- admin diwakili glob crm:* (write+approve); menambah objek CRM baru tak menuntut
-- baris admin baru.
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT t.id, p.role, p.obj, p.act
FROM tenants t
CROSS JOIN (VALUES
    -- ADMIN — glob, semua modul (write mencakup read; approve berdiri sendiri)
    ('admin','crm:*','write'), ('admin','crm:*','approve'),
    -- MANAGER
    ('manager','crm:dashboard','read'),
    ('manager','crm:accounts','write'), ('manager','crm:contacts','write'),
    ('manager','crm:leads','write'),
    ('manager','crm:deals','write'), ('manager','crm:deals','approve'),
    ('manager','crm:quotes','write'), ('manager','crm:quotes','approve'),
    ('manager','crm:sales_activity','write'),
    ('manager','crm:subscriptions','write'), ('manager','crm:renewals','write'),
    ('manager','crm:plans','read'), ('manager','crm:churn','write'),
    ('manager','crm:health','write'), ('manager','crm:journey','write'),
    ('manager','crm:success_plans','write'), ('manager','crm:adoption','write'),
    ('manager','crm:engagements','write'),
    ('manager','crm:renewal_mgmt','write'), ('manager','crm:renewal_mgmt','approve'),
    ('manager','crm:playbooks','write'), ('manager','crm:voc','write'),
    ('manager','crm:tickets','write'), ('manager','crm:kb','write'),
    ('manager','crm:sla','write'), ('manager','crm:activities','write'),
    ('manager','crm:reports','read'),
    -- SALES
    ('sales','crm:dashboard','read'),
    ('sales','crm:accounts','write'), ('sales','crm:contacts','write'),
    ('sales','crm:leads','write'), ('sales','crm:deals','write'),
    ('sales','crm:quotes','write'), ('sales','crm:sales_activity','write'),
    ('sales','crm:subscriptions','read'), ('sales','crm:renewals','read'),
    ('sales','crm:plans','read'), ('sales','crm:churn','read'),
    ('sales','crm:health','read'), ('sales','crm:renewal_mgmt','read'),
    ('sales','crm:tickets','read'), ('sales','crm:kb','read'),
    ('sales','crm:activities','write'), ('sales','crm:reports','read'),
    -- CSM
    ('csm','crm:dashboard','read'),
    ('csm','crm:accounts','write'), ('csm','crm:contacts','write'),
    ('csm','crm:deals','read'), ('csm','crm:subscriptions','read'),
    ('csm','crm:renewals','read'), ('csm','crm:plans','read'),
    ('csm','crm:churn','write'), ('csm','crm:health','write'),
    ('csm','crm:journey','write'), ('csm','crm:success_plans','write'),
    ('csm','crm:adoption','write'), ('csm','crm:engagements','write'),
    ('csm','crm:renewal_mgmt','write'), ('csm','crm:playbooks','write'),
    ('csm','crm:voc','write'), ('csm','crm:tickets','read'),
    ('csm','crm:kb','read'), ('csm','crm:sla','read'),
    ('csm','crm:activities','write'), ('csm','crm:reports','read'),
    -- SUPPORT
    ('support','crm:dashboard','read'),
    ('support','crm:accounts','read'), ('support','crm:contacts','read'),
    ('support','crm:health','read'), ('support','crm:tickets','write'),
    ('support','crm:kb','write'), ('support','crm:sla','read'),
    ('support','crm:activities','write'), ('support','crm:reports','read')
) AS p(role, obj, act)
ON CONFLICT (tenant_id, role, obj, act) DO NOTHING;
-- +goose StatementEnd

-- admin butuh objek panel manajemen peran itu sendiri: crm:roles (write).
-- Dipisah dari glob crm:* agar terbaca eksplisit — crm:roles TERCAKUP crm:*,
-- baris ini menegaskannya untuk pembaca matriks, ON CONFLICT menahannya idempoten.
-- +goose StatementBegin
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
SELECT t.id, 'admin', 'crm:roles', 'write' FROM tenants t
ON CONFLICT (tenant_id, role, obj, act) DO NOTHING;
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS business_role_permissions;
DROP TABLE IF EXISTS business_roles;
-- CHECK lama tak dipulihkan: nilai business_role tetap valid tanpa constraint,
-- dan mengembalikannya menuntut 5-nama harfiah yang justru migrasi ini buang.
-- +goose StatementEnd
