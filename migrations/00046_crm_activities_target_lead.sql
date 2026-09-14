-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- CHECK target_type: tambah 'lead' (BL-160)
--
-- Migrasi 00011 membatasi target_type ke ('account','contact','deal','ticket',
-- 'subscription') — Lead sengaja tak disertakan saat itu (belum ber-UI). BL-160
-- (sumber: user 12 Sep) meminta Lead juga bisa dicatat Activity-nya (pola sama
-- Deal/Contact, reuse activitiesTimelineFor). Tambah 'lead' ke CHECK sebagai
-- jaring terakhir; validasi utama tetap di handler (validActivityTargetTypes +
-- targetInScope, sales_activities_form.go/sales_activities_helpers.go).
--
-- Idempoten: DROP IF EXISTS lebih dulu (CHECK tak punya "ADD nilai"), nama tetap.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE activities DROP CONSTRAINT IF EXISTS activities_target_type_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activities ADD CONSTRAINT activities_target_type_chk
    CHECK (target_type IN ('account','contact','deal','ticket','subscription','lead'));
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
ALTER TABLE activities DROP CONSTRAINT IF EXISTS activities_target_type_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activities ADD CONSTRAINT activities_target_type_chk
    CHECK (target_type IN ('account','contact','deal','ticket','subscription'));
-- +goose StatementEnd
