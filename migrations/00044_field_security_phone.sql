-- 00044_field_security_phone.sql — BL-107: FLS HP/WhatsApp konfigurabel per-tenant.
--
-- Sampai sekarang sumbu F4 (Field-Level Security) untuk nomor HP & WhatsApp kontak
-- DI-HARDCODE di internal/handler/fls.go: lihat penuh = Sales+Admin, sunting =
-- Sales. Untuk template yang di-clone tiap workspace itu kaku — satu desa tak bisa
-- memutuskan sendiri, mis. Manager-nya juga boleh melihat nomor. Migrasi ini
-- memindahkan kebijakan itu ke tabel tenant-scoped yang bisa diubah saat aplikasi
-- berjalan (reload cache per-tenant, tanpa restart), meniru business_role_permissions.
--
-- SATU kebijakan "phone" mencakup mobile_phone DAN whatsapp bersama: keduanya
-- di-gate identik di tiap site lewat helper fls.go yang sama, jadi tak dipisah.
-- Berlaku untuk modul Kontak, Lead, & form Konversi Lead (BUKAN Account — BL-106
-- sudah melepas FLS dari sana sepenuhnya).
--
-- Baris ABSEN untuk sebuah tenant = "belum dikonfigurasi" = pakai DEFAULT bawaan
-- kode (lihat penuh Sales+Admin, sunting Sales) — sama pola code_formats. Nol
-- perubahan perilaku sampai admin sengaja mengisi. Saat admin menyimpan, POST
-- menulis baris untuk SETIAP peran (termasuk all-false) → "admin uncheck semua"
-- tersimpan nyata & beda dari default. ADR 0012.

-- +goose Up
-- +goose StatementBegin

-- ── field_security_policies — kebijakan FLS phone per (tenant, business_role) ──
-- Satu baris per peran (bukan jsonb): tiap baris memetakan langsung ke SELECT yang
-- dikonsumsi halaman Settings & loader cache — alasan sama business_role_permissions
-- (00007). FK komposit ke (tenant_id, name): menghapus peran mencabut baris FLS-nya
-- (CASCADE), dan baris tak bisa menunjuk peran yang tak ada di workspace-nya.
--
-- CHECK edit⇒view: nomor yang tak boleh dilihat mustahil disunting. Menjaga realita
-- sekarang (Admin=lihat-saja, Sales=lihat+sunting) & menolak config nirmakna
-- "sunting tanpa lihat" tersimpan.
CREATE TABLE IF NOT EXISTS field_security_policies (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id      BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    business_role  TEXT NOT NULL,
    can_view_phone BOOLEAN NOT NULL DEFAULT false,
    can_edit_phone BOOLEAN NOT NULL DEFAULT false,
    created_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fsp_edit_implies_view_chk CHECK (NOT can_edit_phone OR can_view_phone),
    CONSTRAINT fsp_role_fk FOREIGN KEY (tenant_id, business_role)
        REFERENCES business_roles (tenant_id, name) ON DELETE CASCADE
);

-- Satu baris per (tenant, role). Upsert bergantung pada unik ini; juga penopang
-- SELECT per-tenant (halaman + reload) & startup load-all.
CREATE UNIQUE INDEX IF NOT EXISTS idx_field_security_policies_role
    ON field_security_policies (tenant_id, business_role);

ALTER TABLE field_security_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE field_security_policies FORCE  ROW LEVEL SECURITY;

-- Pola dua GUC (is_super + tenant_id) identik tabel CRM lain. is_super memungkinkan
-- WithSuper bypass untuk startup load-all (RLS menyembunyikan tenant lain di dalam
-- tx ber-scope, jadi konteks super wajib untuk memuat semua tenant sekaligus).
DROP POLICY IF EXISTS tenant_isolation ON field_security_policies;
CREATE POLICY tenant_isolation ON field_security_policies
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

GRANT SELECT, INSERT, UPDATE, DELETE ON field_security_policies TO app_rw;

-- Tanpa backfill: absennya baris = default terkunci. Workspace lama & baru sama-sama
-- mempertahankan perilaku hardcode lama sampai admin membuka /field-security & menyimpan.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS field_security_policies;
-- +goose StatementEnd
