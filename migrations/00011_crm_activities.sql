-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 7 ACTIVITIES (fondasi) + slice Sales 4.4 — tabel POLIMORFIK `activities`.
--
-- Migrasi INKREMENTAL (00011). SATU tabel menampung aktivitas Sales Activity Log
-- (4.4, kind task/meeting/call/chat/email/note) dan Activities global (Modul 7),
-- dibedakan lewat `activity_context` (sales/general) — agar timeline lintas-modul
-- (semua aktivitas satu account/deal/contact) cukup satu query, dan agar
-- penambahan jenis baru tak menuntut tabel + join baru.
--
-- KOREKSI (BL-40, 2026-09-04): rencana awal di sini menyatukan JUGA Customer
-- Success Engagement (6.5) ke tabel ini via `activity_context='cs'` — "bukan
-- tabel terpisah". Itu TIDAK terwujud: migrasi 00022 justru membangun CS
-- Engagement sebagai tabel SENDIRI `engagements` (entity dgn semantik beda —
-- FK account sungguhan `ON DELETE RESTRICT`, kosakata `engagement_type`/
-- `frequency`/`channel`, siklus hidup `planned/done/skipped/rescheduled`,
-- `scheduled_at NOT NULL` + `next_due_date`). Akibatnya `activity_context='cs'`
-- di tabel ini = SLOT MATI (nol query mengisinya). Timeline lintas-modul yang
-- menggabung Sales + CS kini pakai UNION dua tabel, bukan satu query. Rasional
-- lengkap keputusan dua-tabel: docs/decisions/0010-activities-vs-engagements-dua-tabel.md.
--
-- Kenapa `target_id` BUKAN FK (pola audit_logs): target polimorfik — satu baris
-- bisa menunjuk deal, account, atau contact — jadi tak ada satu tabel untuk
-- di-REFERENCES. Lebih penting: aktivitas adalah JEJAK; ia harus bertahan walau
-- target-nya di-hard-delete (mencatat "menelepon kontak X" tetap bermakna setelah
-- kontak itu hilang). FK ON DELETE CASCADE justru akan menghapus jejaknya.
-- Integritas target ditegakkan di layer aplikasi (loadOwned* memverifikasi target
-- ada & dalam cakupan aktor SEBELUM insert), bukan constraint DB.
--
-- Dibangun PENUH sekali (semua kolom per-kind) walau UI Sales v1 hanya memakai 3
-- kind (task/call/note): M7 & CS tinggal menambah UI di atas kolom yang sudah ada
-- tanpa migrasi ulang. `activity_attendees` (§7b, junction peserta meeting) SENGAJA
-- DITUNDA — hanya relevan saat UI meeting dibangun (Modul 7), pola sama seperti
-- 00009 menunda FK subscriptions sampai Modul 5.
--
-- CHECK hanya untuk kolom yang divalidasi handler v1 (kind, target_type,
-- activity_context, status, priority, direction, call_result). Kolom meeting/
-- email/CS = TEXT polos nullable: M7/CS menambah CHECK saat membangun UI-nya —
-- meng-constrain sekarang berisiko menolak nilai sah yang belum kita ketahui.
--
-- RLS = isolasi WORKSPACE (tenant_id) saja, pola verbatim dari 00010. Isolasi
-- ANTAR-DESA (F3, owner_id) ditegakkan di layer query (ActivitiesListFilter),
-- BUKAN RLS. GRANT app_rw otomatis via ALTER DEFAULT PRIVILEGES (00004).
-- ═══════════════════════════════════════════════════════════════════════════


-- ── activities — aktivitas polimorfik (§7a) ────────────────────────────────
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS activities (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Diskriminator + identitas umum
    kind              TEXT NOT NULL,          -- task/meeting/call/chat/email/note
    subject           TEXT NOT NULL,          -- judul ringkas (semua kind)
    -- Target polimorfik: type memilih "tabel", id menunjuk baris. id BUKAN FK
    -- (lihat header) — tahan hard-delete target, integritas dijaga aplikasi.
    target_type       TEXT NOT NULL,          -- account/contact/deal/ticket/subscription
    target_id         BIGINT NOT NULL,
    -- Kepemilikan F3 (satu sumbu, seperti deal_owner) → users. Pemisah view antar
    -- modul: sales / cs / general.
    owner_id          BIGINT REFERENCES users(id) ON DELETE SET NULL,
    activity_context  TEXT,                   -- sales/general ('cs' = SLOT MATI, lihat header/ADR 0010)
    status            TEXT,                   -- daur hidup (lihat CHECK)
    notes             TEXT,

    -- Task (follow-up berjadwal)
    due_date          DATE,
    priority          TEXT,                   -- Low/Normal/High
    reminder_at       TIMESTAMPTZ,

    -- Meeting (belum ber-UI v1 — TEXT polos, CHECK ditunda ke Modul 7)
    start_at          TIMESTAMPTZ,
    end_at            TIMESTAMPTZ,
    all_day           BOOLEAN,
    location          TEXT,
    meeting_type      TEXT,

    -- Call / Chat (log komunikasi)
    contact_id        BIGINT REFERENCES contacts(id) ON DELETE SET NULL,
    direction         TEXT,                   -- Inbound/Outbound
    activity_at       TIMESTAMPTZ,
    duration_min      INTEGER,
    call_result       TEXT,                   -- Connected/No Answer/…

    -- Email (belum ber-UI v1)
    email_from        TEXT,
    email_to          TEXT,
    email_status      TEXT,
    body              TEXT,

    -- Customer Success 6.5 (belum ber-UI v1)
    engagement_type   TEXT,
    frequency         TEXT,
    channel           TEXT,
    scheduled_date    TIMESTAMPTZ,
    next_due_date     DATE,

    -- Soft-delete + audit
    deleted_at        TIMESTAMPTZ,
    created_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT activities_kind_chk CHECK
        (kind IN ('task','meeting','call','chat','email','note')),
    CONSTRAINT activities_target_type_chk CHECK
        (target_type IN ('account','contact','deal','ticket','subscription')),
    CONSTRAINT activities_context_chk CHECK
        (activity_context IS NULL OR activity_context IN ('sales','cs','general')),
    CONSTRAINT activities_status_chk CHECK
        (status IS NULL OR status IN
            ('Not Started','In Progress','Completed','Deferred',
             'Planned','Held','Cancelled','No-Show')),
    CONSTRAINT activities_priority_chk CHECK
        (priority IS NULL OR priority IN ('Low','Normal','High')),
    CONSTRAINT activities_direction_chk CHECK
        (direction IS NULL OR direction IN ('Inbound','Outbound')),
    CONSTRAINT activities_call_result_chk CHECK
        (call_result IS NULL OR call_result IN
            ('Connected','No Answer','Busy','Voicemail','Follow-up','No Respond'))
);
-- +goose StatementEnd

-- Index target (timeline satu entitas: semua aktivitas deal/account/contact X).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_activities_target
    ON activities (tenant_id, target_type, target_id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Index owner (list F3 + keyset created_at DESC) — penopang ListActivities.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_activities_owner
    ON activities (tenant_id, owner_id, created_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Index kind (filter per jenis aktivitas).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_activities_kind
    ON activities (tenant_id, kind, created_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Index context (pemisah view Sales 4.4 vs CS 6.5 vs global M7).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_activities_context
    ON activities (tenant_id, activity_context, created_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd


-- ── RLS — isolasi workspace (tak diwariskan) ───────────────────────────────
-- ENABLE + FORCE: FORCE = pemilik tabel pun terkena policy. Policy dua-GUC IDENTIK
-- template 00005/00009/00010: app.is_super='on' (jalur platform) ATAU tenant_id
-- cocok; GUC absen → NOL baris (fail-closed).
-- +goose StatementBegin
ALTER TABLE activities ENABLE ROW LEVEL SECURITY;
ALTER TABLE activities FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON activities
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS activities;
-- +goose StatementEnd
