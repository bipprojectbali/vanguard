-- health_score.sql — workspace-level Health Score listing (Modul 6 slice C1).
-- Data sumber: customer_success (1:1 dengan accounts). Tidak ada tabel baru.
-- F3 ownership: scope_all (admin/manager) / is_csm / is_sales — pola SAMA
-- dengan ListAccounts agar tidak divergen. Support (ScopeNone) → semua flag
-- false → 0 baris (fail-closed, bukan error).

-- name: ListHealthScores :many
-- Workspace-level listing akun + data health score.
-- Keyset (created_at DESC, id DESC), filter_status '' = semua.
-- uid dioper walau scope_all (diabaikan di klausa).
SELECT
    a.id,
    a.village_name           AS account_name,
    cs.overall_health_score,
    cs.health_status,
    cs.adoption_score,
    cs.engagement_score,
    cs.support_score,
    cs.sentiment_score,
    cs.score_trend,
    cs.lifecycle_stage,
    cs.stage_entry_date,
    a.created_at
FROM accounts a
LEFT JOIN customer_success cs
       ON cs.account_id = a.id AND cs.tenant_id = a.tenant_id
WHERE a.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_csm)::boolean
          AND (a.assigned_csm = sqlc.arg(uid) OR a.backup_csm = sqlc.arg(uid)))
      OR (sqlc.arg(is_sales)::boolean
          AND a.account_owner = sqlc.arg(uid))
  )
  AND (sqlc.arg(filter_status) = ''
       OR COALESCE(cs.health_status, '') = sqlc.arg(filter_status))
  AND (a.created_at, a.id) < (sqlc.arg(cursor_created_at)::timestamptz,
                               sqlc.arg(cursor_id)::bigint)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountHealthScoreKPIs :one
-- KPI agregat untuk header: total desa (dalam scope), sehat/berisiko/kritis,
-- dan rata-rata skor (NULL bila semua skor belum diisi).
-- Ownership clause SAMA PERSIS dengan ListHealthScores agar konsisten.
SELECT
    COUNT(*)                                                        AS total,
    COUNT(*) FILTER (WHERE cs.health_status = 'Healthy')           AS healthy,
    COUNT(*) FILTER (WHERE cs.health_status = 'At-Risk')           AS at_risk,
    COUNT(*) FILTER (WHERE cs.health_status = 'Critical')          AS critical,
    COUNT(*) FILTER (WHERE cs.overall_health_score IS NOT NULL)     AS scored
FROM accounts a
LEFT JOIN customer_success cs
       ON cs.account_id = a.id AND cs.tenant_id = a.tenant_id
WHERE a.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_csm)::boolean
          AND (a.assigned_csm = sqlc.arg(uid) OR a.backup_csm = sqlc.arg(uid)))
      OR (sqlc.arg(is_sales)::boolean
          AND a.account_owner = sqlc.arg(uid))
  );
