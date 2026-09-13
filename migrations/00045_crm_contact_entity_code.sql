-- 00045_crm_contact_entity_code.sql — BL-132: kode lookup sistem untuk Kontak
-- (KON-001, KON-002, …), mengikuti pola accounts/leads/deals/dst di 00006.
--
-- Kontak SENGAJA tak berkode sejak 00006 (sub-entitas desa, contacts.account_id
-- NOT NULL — lookupnya lewat desa induk). User minta kontak tetap punya kode
-- sendiri untuk dikutip lepas dari desa induknya. CHECK entity di code_formats/
-- code_sequences dibuat SEMPIT sengaja (00006) — menambah entitas berkode berarti
-- ALTER CHECK di sini, bukan menyelinapkan nilai baru lewat migrasi data.
--
-- Data lama TAK di-backfill (kolom nullable, mirror keputusan 00006 untuk
-- accounts.entity_code): kontak yang sudah ada sebelum migrasi ini tetap tanpa
-- kode; alokasi terjadi mulai dari jalur CREATE aplikasi berikutnya.

-- +goose Up
-- +goose StatementBegin

-- Lebarkan CHECK code_formats/code_sequences agar 'contact' diterima. Nama
-- constraint dipertahankan SAMA dgn 00006 (ALTER, bukan buat baru) — hindari dua
-- constraint tumpang tindih menjaga kolom yang sama.
ALTER TABLE code_formats DROP CONSTRAINT code_formats_entity_chk;
ALTER TABLE code_formats ADD CONSTRAINT code_formats_entity_chk CHECK
    (entity IN ('account','lead','deal','quote','ticket','subscription','contact'));

ALTER TABLE code_sequences DROP CONSTRAINT code_sequences_entity_chk;
ALTER TABLE code_sequences ADD CONSTRAINT code_sequences_entity_chk CHECK
    (entity IN ('account','lead','deal','quote','ticket','subscription','contact'));

-- contacts.entity_code — mirror accounts.entity_code (00006): nullable, unik
-- parsial per-tenant (hanya baris hidup & berkode).
ALTER TABLE contacts
    ADD COLUMN IF NOT EXISTS entity_code TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_entity_code
    ON contacts (tenant_id, entity_code)
    WHERE entity_code IS NOT NULL AND deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_contacts_entity_code;
ALTER TABLE contacts DROP COLUMN IF EXISTS entity_code;

ALTER TABLE code_sequences DROP CONSTRAINT code_sequences_entity_chk;
ALTER TABLE code_sequences ADD CONSTRAINT code_sequences_entity_chk CHECK
    (entity IN ('account','lead','deal','quote','ticket','subscription'));

ALTER TABLE code_formats DROP CONSTRAINT code_formats_entity_chk;
ALTER TABLE code_formats ADD CONSTRAINT code_formats_entity_chk CHECK
    (entity IN ('account','lead','deal','quote','ticket','subscription'));

-- +goose StatementEnd
