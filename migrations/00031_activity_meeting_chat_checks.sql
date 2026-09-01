-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- CHECK meeting_type & channel: Pertemuan + Chat kini ber-UI
--
-- Migrasi 00011 sengaja MENUNDA CHECK untuk meeting_type & channel karena kedua
-- kind (meeting/chat) belum ber-form — kolomnya selalu NULL. Kini Sales Activity
-- menambah form Pertemuan (meeting_type) & Chat (channel), sehingga nilai enum
-- keduanya divalidasi handler → CHECK dipasang sebagai jaring terakhir, cermin
-- pola call_result_chk/direction_chk (00011). Guard `IS NULL OR IN(...)` aman:
-- semua baris lama NULL (tak pernah di-INSERT), jadi ADD CONSTRAINT tak menolak
-- data historis.
--
-- Idempoten: DROP IF EXISTS lebih dulu (CHECK tak punya "ADD nilai"), nama tetap.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE activities DROP CONSTRAINT IF EXISTS activities_meeting_type_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activities ADD CONSTRAINT activities_meeting_type_chk
    CHECK (meeting_type IS NULL OR meeting_type IN ('Tatap Muka', 'Daring'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activities DROP CONSTRAINT IF EXISTS activities_channel_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activities ADD CONSTRAINT activities_channel_chk
    CHECK (channel IS NULL OR channel IN ('WhatsApp', 'Telegram', 'SMS', 'Lainnya'));
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
ALTER TABLE activities DROP CONSTRAINT IF EXISTS activities_meeting_type_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activities DROP CONSTRAINT IF EXISTS activities_channel_chk;
-- +goose StatementEnd
