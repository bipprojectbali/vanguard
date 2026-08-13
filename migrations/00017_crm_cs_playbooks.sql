-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 6 CUSTOMER SUCCESS (slice A2) — tabel `playbooks` (6.7). Katalog
-- prosedur respons per skenario (Health Drop/Low Adoption/Renewal
-- Approaching/New Onboarding), dipakai CSM saat menjalankan intervensi.
--
-- Pola SAMA PERSIS dgn sla_policies (00005/00016): master milik WORKSPACE,
-- TANPA F3 ownership (RLS satu-satunya pengurung), TANPA soft-delete
-- (is_active=false = draf/nonaktif, bukan terhapus — playbook lama tetap
-- terbaca meski tak dipakai lagi). Fresh CREATE TABLE di migrasi inkremental
-- (bukan sunting 00005), pola verbatim 00012 (subscriptions). GRANT app_rw
-- otomatis via ALTER DEFAULT PRIVILEGES (00004) — tak ada GRANT manual.
--
-- `steps` TEXT bebas (bukan tabel langkah terpisah/step count) — cermin
-- docs/crm/skema.md §6g. Eksekusi/riwayat pemakaian playbook (KPI "Sedang
-- Berjalan"/"Tingkat Sukses" di wireframe) SENGAJA di luar slice ini — butuh
-- tabel log eksekusi yang belum ada, sama alasan dgn kepatuhan SLA di A1.
--
-- trigger_scenario & recommended_owner TANPA CHECK di DB (kolom teks nullable)
-- — divalidasi di handler, pola verbatim sla_policies.applies_to_priority/
-- business_hours (00005): dropdown menawarkan set tetap, backend validasi
-- SATU tempat (parseSLAPolicyForm/parsePlaybookForm), bukan campuran DB+app.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS playbooks (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id          BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    playbook_name      TEXT NOT NULL,
    trigger_scenario   TEXT,
    description        TEXT,
    steps              TEXT,
    recommended_owner  TEXT,
    is_active          BOOLEAN NOT NULL DEFAULT true,
    created_by         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_playbooks_active ON playbooks (tenant_id, is_active);
-- +goose StatementEnd


-- ── RLS — isolasi WORKSPACE (pola verbatim 00005/00012) ─────────────────────
-- ENABLE + FORCE (pemilik tabel pun terkena policy). Policy dua-GUC: GUC absen
-- → NOL baris (fail-closed), sama dgn seluruh tabel Modul 6.
-- +goose StatementBegin
ALTER TABLE playbooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE playbooks FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON playbooks
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS playbooks;
-- +goose StatementEnd
