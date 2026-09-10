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
    sub.end_date AS renewal_end_date,
    a.created_at
FROM accounts a
LEFT JOIN customer_success cs
       ON cs.account_id = a.id AND cs.tenant_id = a.tenant_id
-- BL-96: kolom "Jatuh Tempo" = jatuh tempo perpanjangan LANGGANAN aktif (bukan
-- days_in_stage). Satu akun bisa punya banyak langganan → ambil end_date PALING
-- DEKAT (ASC) dari langganan Active belum-terhapus. LATERAL LIMIT 1 memakai
-- idx_subs_one_active-adjacent (tenant_id, end_date) WHERE status='Active'.
LEFT JOIN LATERAL (
    SELECT s.end_date
    FROM subscriptions s
    WHERE s.account_id = a.id AND s.tenant_id = a.tenant_id
      AND s.status = 'Active' AND s.deleted_at IS NULL
      AND s.end_date IS NOT NULL
    ORDER BY s.end_date ASC
    LIMIT 1
) sub ON TRUE
WHERE a.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_csm)::boolean
          AND (a.assigned_csm = sqlc.arg(uid) OR a.backup_csm = sqlc.arg(uid)))
      OR (sqlc.arg(is_sales)::boolean
          AND a.account_owner = sqlc.arg(uid))
  )
  -- BL-114: populasi Health Score = desa PELANGGAN (punya langganan), bukan semua
  -- desa. Churn hidup di subscriptions.status (bukan lifecycle_stage). segment
  -- 'active' (default) = punya langganan hidup (Trial/Active/Suspended); 'churned'
  -- = punya langganan tapi TAK ada yang hidup (semua Expired/Cancelled/Churned).
  -- Prospek (tanpa langganan sama sekali) dikecualikan dari KEDUA segmen.
  AND (
      CASE WHEN sqlc.arg(segment)::text = 'churned' THEN
          EXISTS (SELECT 1 FROM subscriptions s2
                  WHERE s2.account_id = a.id AND s2.tenant_id = a.tenant_id
                    AND s2.deleted_at IS NULL)
          AND NOT EXISTS (SELECT 1 FROM subscriptions s3
                  WHERE s3.account_id = a.id AND s3.tenant_id = a.tenant_id
                    AND s3.deleted_at IS NULL
                    AND s3.status IN ('Trial','Active','Suspended'))
      ELSE
          EXISTS (SELECT 1 FROM subscriptions s2
                  WHERE s2.account_id = a.id AND s2.tenant_id = a.tenant_id
                    AND s2.deleted_at IS NULL
                    AND s2.status IN ('Trial','Active','Suspended'))
      END
  )
  AND (sqlc.arg(filter_status) = ''
       OR COALESCE(cs.health_status, '') = sqlc.arg(filter_status))
  AND (a.created_at, a.id) < (sqlc.arg(cursor_created_at)::timestamptz,
                               sqlc.arg(cursor_id)::bigint)
  AND (sqlc.arg(search)::text = ''
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountHealthScoreKPIs :one
-- KPI agregat untuk header + panel dasbor (BL-96): total desa (dalam scope),
-- sehat/berisiko/kritis, rata-rata skor (NULL bila semua skor belum diisi),
-- rata-rata per-komponen (panel Komposisi Skor), dan cacah arah tren (panel
-- Arah Pergerakan). AVG di-cast ::float8 agar sqlc emit *float64 (nullable),
-- bukan pgtype.Numeric. Ownership clause SAMA PERSIS dengan ListHealthScores.
SELECT
    COUNT(*)                                                        AS total,
    COUNT(*) FILTER (WHERE cs.health_status = 'Healthy')           AS healthy,
    COUNT(*) FILTER (WHERE cs.health_status = 'At-Risk')           AS at_risk,
    COUNT(*) FILTER (WHERE cs.health_status = 'Critical')          AS critical,
    COUNT(*) FILTER (WHERE cs.overall_health_score IS NOT NULL)     AS scored,
    -- AVG native NULL bila tak ada baris terskor → COALESCE 0 agar sqlc emit
    -- float64 non-nullable & scan tak gagal (mis. Support scope=none → 0 baris).
    -- Handler menampilkan "—"/placeholder saat scored=0, jadi 0 tak menyesatkan.
    COALESCE(AVG(cs.overall_health_score), 0)::float8               AS avg_score,
    COALESCE(AVG(cs.adoption_score), 0)::float8                     AS avg_adoption,
    COALESCE(AVG(cs.engagement_score), 0)::float8                   AS avg_engagement,
    COALESCE(AVG(cs.support_score), 0)::float8                      AS avg_support,
    COALESCE(AVG(cs.sentiment_score), 0)::float8                    AS avg_sentiment,
    COUNT(*) FILTER (WHERE cs.score_trend = 'Improving')           AS trend_improving,
    COUNT(*) FILTER (WHERE cs.score_trend = 'Stable')              AS trend_stable,
    COUNT(*) FILTER (WHERE cs.score_trend = 'Declining')           AS trend_declining
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
  -- BL-114: KPI dihitung atas populasi yang SAMA dengan ListHealthScores
  -- (desa pelanggan per segmen), bukan seluruh desa.
  AND (
      CASE WHEN sqlc.arg(segment)::text = 'churned' THEN
          EXISTS (SELECT 1 FROM subscriptions s2
                  WHERE s2.account_id = a.id AND s2.tenant_id = a.tenant_id
                    AND s2.deleted_at IS NULL)
          AND NOT EXISTS (SELECT 1 FROM subscriptions s3
                  WHERE s3.account_id = a.id AND s3.tenant_id = a.tenant_id
                    AND s3.deleted_at IS NULL
                    AND s3.status IN ('Trial','Active','Suspended'))
      ELSE
          EXISTS (SELECT 1 FROM subscriptions s2
                  WHERE s2.account_id = a.id AND s2.tenant_id = a.tenant_id
                    AND s2.deleted_at IS NULL
                    AND s2.status IN ('Trial','Active','Suspended'))
      END
  );
