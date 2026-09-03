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

-- name: ReportHealthByStatus :many
-- Customer Success Report (wireframe 8.2): breakdown Health/Adoption per
-- status. NPS/CSAT (skema.md §8, sumber kedua 8.2) TIDAK termasuk — tabel
-- survei belum ada di skema mana pun (lihat doc comment reports_cs.go
-- handler); scope 8.2 di sini sengaja dipersempit ke Health/Adoption saja.
-- F3 ownership PERSIS ListHealthScores/CountHealthScoreKPIs (health_score.sql)
-- agar tak divergen dari halaman /health-scores.
SELECT
    COALESCE(cs.health_status, 'Belum Dinilai')     AS health_status,
    COUNT(*)::bigint                                AS account_count,
    ROUND(AVG(cs.overall_health_score)::numeric, 1) AS avg_health_score
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
GROUP BY COALESCE(cs.health_status, 'Belum Dinilai')
ORDER BY CASE COALESCE(cs.health_status, 'Belum Dinilai')
    WHEN 'Healthy'  THEN 1
    WHEN 'At-Risk'  THEN 2
    WHEN 'Critical' THEN 3
    ELSE 4
END;

-- name: ReportTicketsByStatus :many
-- Support Report (wireframe 8.3): breakdown tiket per status + jumlah
-- terlanggar SLA + rata-rata jam resolusi (hanya tiket 'selesai'). F3
-- ownership PERSIS ListTickets/CountTicketKPIs (tickets.sql) — TERMASUK
-- override Support (TicketsListFilterFor: data_scope='none' + canWrite →
-- ScopeAll). avg_resolution_hours NULL bila belum ada tiket selesai di grup.
SELECT
    t.status          AS status,
    COUNT(*)::bigint  AS ticket_count,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND t.sla_deadline_at < now()
                       AND t.status <> 'selesai')::bigint AS breached_count,
    ROUND((AVG(EXTRACT(EPOCH FROM (t.resolved_at - t.created_at)) / 3600.0)
           FILTER (WHERE t.status = 'selesai' AND t.resolved_at IS NOT NULL))::numeric, 1)
                      AS avg_resolution_hours
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
)
GROUP BY t.status
ORDER BY CASE t.status
    WHEN 'baru'     THEN 1
    WHEN 'diproses' THEN 2
    WHEN 'menunggu' THEN 3
    WHEN 'selesai'  THEN 4
    ELSE 5
END;
