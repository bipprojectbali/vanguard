-- 00052_member_scope_policy.sql — BL-171: cakupan jenis anggota (internal/
-- eksternal) per (tenant, business_role) yang boleh dilihat/dikelola peran itu
-- di halaman Anggota (/members) lewat akses baru sumbu bisnis "crm:members".
--
-- Mirror persis field_security_policies (00044): satu baris per (tenant,
-- business_role), FK komposit ke business_roles(tenant_id, name) ON DELETE
-- CASCADE, RLS FORCE + tenant_isolation, GRANT app_rw. Beda dari FLS: CHECK di
-- sini menolak KEDUA kolom false sekaligus (minimal satu jenis harus terlihat
-- — checkbox kosong semua tak bermakna, beda dari FLS yang boleh all-false).
--
-- Baris ABSEN untuk sebuah (tenant, role) = "belum dikonfigurasi" = default
-- KEDUA jenis terbuka (bukan tertutup seperti FLS) — supaya fitur murni
-- opt-in tanpa backfill: role lama yang baru diberi akses crm:members lewat
-- /roles langsung melihat SEMUA anggota sampai admin sengaja mempersempitnya.

-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS member_scope_policies (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    business_role     TEXT NOT NULL,
    can_view_internal BOOLEAN NOT NULL DEFAULT true,
    can_view_external BOOLEAN NOT NULL DEFAULT true,
    created_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT msp_at_least_one_chk CHECK (can_view_internal OR can_view_external),
    CONSTRAINT msp_role_fk FOREIGN KEY (tenant_id, business_role)
        REFERENCES business_roles (tenant_id, name) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_member_scope_policies_role
    ON member_scope_policies (tenant_id, business_role);

ALTER TABLE member_scope_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE member_scope_policies FORCE  ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON member_scope_policies;
CREATE POLICY tenant_isolation ON member_scope_policies
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

GRANT SELECT, INSERT, UPDATE, DELETE ON member_scope_policies TO app_rw;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS member_scope_policies;
-- +goose StatementEnd
