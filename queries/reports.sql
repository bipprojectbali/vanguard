-- reports.sql — preset report read-only Modul 8 (tasks.md M8-1). "Report bukan
-- objek data" (skema.md §8): nol tabel baru, query agregasi murni atas tabel yang
-- sudah ada. Ownership (F3) pakai flag SAMA dengan modul asal tabel — sumber SATU,
-- bukan duplikat logic scope (pola sama dengan dashboard.sql).

-- name: ReportPipelineByStage :many
-- Sales Report (wireframe 8.1): SEMUA stage TERMASUK Closed Won/Lost — beda
-- sengaja dari DashboardPipelineByStage (yang exclude keduanya untuk chart
-- funnel). Report butuh gambaran penuh pipeline+hasil, bukan cuma yang terbuka.
SELECT
    stage,
    COUNT(*)::bigint                  AS deal_count,
    COALESCE(SUM(amount), 0)::numeric AS stage_value
FROM deals
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
GROUP BY stage
ORDER BY CASE stage
    WHEN 'Prospecting'  THEN 1
    WHEN 'Qualification' THEN 2
    WHEN 'Demo'          THEN 3
    WHEN 'Proposal'      THEN 4
    WHEN 'Negotiation'   THEN 5
    WHEN 'Closed Won'    THEN 6
    WHEN 'Closed Lost'   THEN 7
    ELSE 8
END;
