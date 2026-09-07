-- cs_journey.sql — Query Customer Journey / Lifecycle (CRM Modul 6, 6.2 —
-- BL-77). Dashboard portofolio: posisi tiap desa di sepanjang fase
-- Onboarding → Adoption → Retention → Renewal → Advocacy. TANPA schema baru —
-- semua kolom sudah ada di customer_success (migrasi 00019); assigned_csm
-- di-JOIN dari accounts (bukan diduplikasi).
--
-- RLS mengisolasi workspace; F3 ownership ditegakkan lewat flag boolean
-- (scope_all, is_own) — SATU sumber kebenaran dengan CSJourneyListFilterFor
-- (ownership_cs_journey.go), pola sama cs_impl_tasks.sql.
--
-- days_in_stage per baris SENGAJA TIDAK dihitung di SQL: gotcha #14 (ekspresi
-- tanggal di SELECT list bikin sqlc emit interface{}). ListCSJourneyAccounts
-- memilih stage_entry_date MENTAH; handler menghitung lama-di-fase di Go.
-- Ekspresi tanggal dalam agregat (AVG/MAX/FILTER) & ORDER BY/WHERE tetap boleh.

-- name: CountCSJourneyKPIs :one
-- 4 KPI header: Onboarding (jumlah + rata hari di fase), Adoption (jumlah +
-- fase terlama = MAX hari), Retention (jumlah), Menuju Renewal (jumlah
-- lifecycle_stage='Renewal'). "Menuju Renewal" SENGAJA didefinisikan MURNI
-- dari lifecycle_stage — tidak menyentuh subscriptions.end_date / cs_renewals
-- agar tak ada definisi ganda (BL-77). Cakupan scope identik ListCSJourneyAccounts.
SELECT
    COUNT(*) FILTER (WHERE cs.lifecycle_stage = 'Onboarding')                                       AS onboarding_count,
    COALESCE(AVG(CURRENT_DATE - cs.stage_entry_date)
        FILTER (WHERE cs.lifecycle_stage = 'Onboarding' AND cs.stage_entry_date IS NOT NULL), 0)::float8 AS onboarding_avg_days,
    COUNT(*) FILTER (WHERE cs.lifecycle_stage = 'Adoption')                                         AS adoption_count,
    COALESCE(MAX(CURRENT_DATE - cs.stage_entry_date)
        FILTER (WHERE cs.lifecycle_stage = 'Adoption' AND cs.stage_entry_date IS NOT NULL), 0)::int  AS adoption_max_days,
    COUNT(*) FILTER (WHERE cs.lifecycle_stage = 'Retention')                                        AS retention_count,
    COUNT(*) FILTER (WHERE cs.lifecycle_stage = 'Renewal')                                          AS renewal_count
FROM customer_success cs
JOIN accounts a ON cs.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
);

-- name: ListCSJourneyPhases :many
-- Agregat per fase untuk funnel "Fase Perjalanan Desa": jumlah desa, rata hari
-- di fase, dan jumlah MACET (stage_entry_date lebih tua dari ambang stalled_days).
-- Hanya mengembalikan fase yang punya baris; handler melengkapi ke 5 fase
-- kanonik (urutan lifecycleStageOptions) dengan nol untuk fase kosong.
SELECT
    cs.lifecycle_stage AS stage,
    COUNT(*) AS village_count,
    COALESCE(AVG(CURRENT_DATE - cs.stage_entry_date)
        FILTER (WHERE cs.stage_entry_date IS NOT NULL), 0)::float8 AS avg_days,
    COUNT(*) FILTER (
        WHERE cs.stage_entry_date IS NOT NULL
        AND (CURRENT_DATE - cs.stage_entry_date) > sqlc.arg(stalled_days)::int
    ) AS stalled_count
FROM customer_success cs
JOIN accounts a ON cs.account_id = a.id AND a.deleted_at IS NULL
WHERE cs.lifecycle_stage IS NOT NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
GROUP BY cs.lifecycle_stage;

-- name: ListCSJourneyOnboarding :many
-- Panel "Onboarding Aktif": desa dengan onboarding_status='In Progress'.
-- Diurut target go-live terdekat lebih dulu (NULL di belakang). Peringatan
-- target lampau dihitung di Go (target_go_live_date < CURRENT_DATE). Dibatasi
-- lim (guardrail no full-scan).
SELECT
    cs.account_id,
    a.village_name AS account_name,
    cs.onboarding_progress,
    cs.onboarding_status,
    cs.target_go_live_date,
    u.name AS csm_name
FROM customer_success cs
JOIN accounts a ON cs.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON a.assigned_csm = u.id
WHERE cs.onboarding_status = 'In Progress'
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
ORDER BY cs.target_go_live_date ASC NULLS LAST, cs.account_id ASC
LIMIT sqlc.arg(lim)::int;

-- name: ListCSJourneyAccounts :many
-- Tabel utama "Desa Binaan — Posisi Lifecycle": satu baris per desa,
-- diurut LAMA-DI-FASE terpanjang lebih dulu (stage_entry_date ASC — masuk
-- fase paling lama = paling perlu perhatian). Keyset ASC lewat kunci koalesi
-- COALESCE(stage_entry_date, created_at) yang SELALU finit (tak pernah NULL,
-- selalu dalam rentang int64-nanos formatCursor) + account_id sebagai
-- pemecah-seri (UNIQUE per baris di customer_success).
--
-- filter_stage '' → semua fase; non-'' → cocokkan lifecycle_stage persis.
SELECT
    cs.account_id,
    a.village_name AS account_name,
    cs.lifecycle_stage,
    cs.stage_entry_date,
    cs.overall_health_score,
    cs.health_status,
    cs.onboarding_progress,
    cs.onboarding_status,
    cs.created_at,
    u.name AS csm_name
FROM customer_success cs
JOIN accounts a ON cs.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON a.assigned_csm = u.id
WHERE (COALESCE(cs.stage_entry_date::timestamp AT TIME ZONE 'UTC', cs.created_at), cs.account_id)
      > (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_stage) = '' OR cs.lifecycle_stage = sqlc.arg(filter_stage))
ORDER BY COALESCE(cs.stage_entry_date::timestamp AT TIME ZONE 'UTC', cs.created_at) ASC, cs.account_id ASC
LIMIT sqlc.arg(page_size);
