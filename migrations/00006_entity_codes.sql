-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- KODE-UNIK ENTITAS (lookup) — mis. DESA-001, DEAL-042, LEAD-007.
--
-- Beda dari `accounts.village_code` (Kode Kemendagri, identitas RESMI desa dari
-- pemerintah): ini kode INTERNAL CRM untuk lookup & percakapan sehari-hari
-- ("tolong cek DESA-014"). Satu diberikan sistem, satu berasal dari luar.
--
-- Tiga keputusan bentuk:
--
-- 1) FORMAT per-workspace, bukan global. Tiap tenant boleh punya prefix/padding
--    sendiri, dan yang lebih penting: COUNTER-nya terpisah, jadi tiap workspace
--    mulai dari 001. Konsisten dengan isolasi RLS & keunikan village_code
--    (per-tenant). Satu tabel `code_formats` (tenant × entity).
--
-- 2) COUNTER di DB, dialokasikan ATOMIK (bukan di Go). `code_sequences` menyimpan
--    next_val per (tenant, entity); alokasi = UPDATE ... RETURNING dalam satu
--    pernyataan yang MENGUNCI baris. Dua INSERT account bersamaan tak mungkin
--    dapat nomor sama — kalau counter dihitung di aplikasi (SELECT max+1), race
--    itu justru yang paling sering lolos sampai dua baris bertabrakan di produksi.
--
-- 3) KODE disimpan di baris entitasnya (`accounts.entity_code`), bukan dirakit
--    ulang tiap baca. Kode = identitas yang dikutip manusia; ia harus STABIL
--    walau format diubah kelak. Merakit dari (prefix sekarang + seq) berarti
--    mengganti prefix mengubah kode SEMUA baris lama — DESA-014 di email kemarin
--    tiba-tiba menunjuk baris lain. Format hanya dipakai saat MEMBUAT kode baru.
-- ═══════════════════════════════════════════════════════════════════════════


-- ── code_formats — format kode per (tenant, entity) ────────────────────────
-- Baris ABSEN = pakai default bawaan kode (internal/codes.DefaultFormat). Jadi
-- workspace baru langsung berkode tanpa seed — sama pola dengan platform_settings
-- yang boleh kosong. Operator hanya mengisi baris saat ingin MENGUBAH default.
--
-- entity = 'account'|'lead'|'deal'|'quote'|'ticket'|'subscription' (CHECK). Nilai
-- di luar itu ditolak DB: entitas baru harus sengaja ditambahkan di dua tempat
-- (CHECK di sini + konstanta di internal/codes), bukan menyelinap lewat typo.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS code_formats (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id  BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    entity     TEXT NOT NULL,
    prefix     TEXT NOT NULL,
    separator  TEXT NOT NULL DEFAULT '-',
    padding    INTEGER NOT NULL DEFAULT 3,   -- lebar minimum angka (001); 0 = tanpa padding
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT code_formats_entity_chk CHECK
        (entity IN ('account','lead','deal','quote','ticket','subscription')),
    -- Padding dibatasi masuk akal: negatif tak berarti, dan >12 menjadikan kode
    -- panjang tak terbaca (lagipula BIGINT seq tak akan sepanjang itu).
    CONSTRAINT code_formats_padding_chk CHECK (padding BETWEEN 0 AND 12),
    -- Separator boleh kosong ('DESA001') tapi tak boleh panjang liar.
    CONSTRAINT code_formats_separator_chk CHECK (length(separator) <= 3),
    -- Prefix wajib ada (kode tanpa prefix = cuma angka, tak bisa dibedakan
    -- antar-entitas saat di-lookup) dan tak boleh panjang liar.
    CONSTRAINT code_formats_prefix_chk CHECK (length(prefix) BETWEEN 1 AND 16)
);
-- +goose StatementEnd

-- Satu format per (tenant, entity). Upsert bergantung pada unik ini.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_code_formats_entity
    ON code_formats (tenant_id, entity);
-- +goose StatementEnd


-- ── code_sequences — counter atomik per (tenant, entity) ───────────────────
-- next_val = nomor yang akan DIPAKAI berikutnya (mulai 1). Alokasi memakai
-- UPDATE ... RETURNING (lihat queries/entity_codes.sql) yang mengunci baris,
-- jadi aman di bawah insert bersamaan. Terpisah dari code_formats: format jarang
-- berubah & dibaca operator; sequence sering di-update di jalur pembuatan —
-- memisahkannya menghindari kontensi tulis pada baris format.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS code_sequences (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    entity    TEXT NOT NULL,
    next_val  BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT code_sequences_pk PRIMARY KEY (tenant_id, entity),
    CONSTRAINT code_sequences_entity_chk CHECK
        (entity IN ('account','lead','deal','quote','ticket','subscription')),
    CONSTRAINT code_sequences_nextval_chk CHECK (next_val >= 1)
);
-- +goose StatementEnd


-- ── accounts.entity_code — kode lookup tersimpan ───────────────────────────
-- Nullable: baris lama (sebelum migrasi ini) belum berkode, dan pemberian kode
-- terjadi saat CREATE lewat aplikasi. Tak di-backfill di sini — backfill butuh
-- alokasi seq berurutan yang lebih rapi dilakukan sekali dari aplikasi bila
-- diinginkan, bukan di DDL. Unik per tenant (hanya baris hidup & berkode):
-- kode adalah penanda yang dikutip manusia, duplikatnya menghancurkan gunanya.
-- +goose StatementBegin
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS entity_code TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_entity_code
    ON accounts (tenant_id, entity_code)
    WHERE entity_code IS NOT NULL AND deleted_at IS NULL;
-- +goose StatementEnd


-- ── RLS — dua tabel baru (tak diwariskan) ──────────────────────────────────
-- code_formats & code_sequences tenant-scoped → policy tenant_isolation IDENTIK
-- template. code_sequences dialokasikan di dalam WithTenant (tx ber-scope app_rw),
-- jadi RLS-nya WAJIB aktif — tanpa policy, app_rw ditolak akses (deny-default RLS)
-- dan pembuatan kode gagal di tiap create. GRANT app_rw otomatis (ALTER DEFAULT
-- PRIVILEGES 00001).
-- +goose StatementBegin
ALTER TABLE code_formats   ENABLE ROW LEVEL SECURITY;
ALTER TABLE code_formats   FORCE  ROW LEVEL SECURITY;
ALTER TABLE code_sequences ENABLE ROW LEVEL SECURITY;
ALTER TABLE code_sequences FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON code_formats
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON code_sequences
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_accounts_entity_code;
ALTER TABLE accounts DROP COLUMN IF EXISTS entity_code;
DROP TABLE IF EXISTS code_sequences;
DROP TABLE IF EXISTS code_formats;
-- +goose StatementEnd
