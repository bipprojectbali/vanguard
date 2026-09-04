-- reports_support.sql — agregasi Support Report (Modul 8, wireframe 8.3, BL-46).
-- Diperluas dari 1 panel (breakdown status) ke 5 panel; query yang DIBANGUN
-- hanya untuk data yang SUDAH ADA (CSAT, kategori tiket, kanal, respons
-- pertama, FCR, KB rating/deflection DILEWATKAN — tak ada kolom/tabel).
--
-- F3 ownership PERSIS ListTickets/CountTicketKPIs (tickets.sql) — TERMASUK
-- override Support (TicketsListFilterFor: data_scope='none' + canWrite →
-- ScopeAll). Filter dinyatakan via JOIN accounts + predikat scope_all/is_own/uid.
-- Bucket bulan: to_char(date_trunc('month', …), 'YYYY-MM') → urut leksikografis =
-- kronologis (pola sama ReportSalesForecast). Timezone: batas bulan UTC (cukup
-- untuk laporan; gotcha #14 — hindari AT TIME ZONE di SELECT list sqlc).

-- name: ReportTicketVolumeByMonth :many
-- Panel 1 (Ticket Volume): per bulan → Tiket Masuk (created bulan itu) & Selesai
-- (resolved bulan itu). Backlog kumulatif (masuk−selesai berjalan) DIHITUNG di
-- handler dari urutan bulan ini (SQL cukup dua deret). UNION ALL dua sumber
-- tanggal lalu GROUP agar bulan yang hanya punya salah satu tetap muncul.
SELECT
    period,
    SUM(masuk)::bigint   AS masuk,
    SUM(selesai)::bigint AS selesai
FROM (
    SELECT to_char(date_trunc('month', t.created_at), 'YYYY-MM') AS period,
           1 AS masuk, 0 AS selesai
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
    UNION ALL
    SELECT to_char(date_trunc('month', t.resolved_at), 'YYYY-MM') AS period,
           0 AS masuk, 1 AS selesai
    FROM tickets t
    JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
    WHERE t.status = 'selesai' AND t.resolved_at IS NOT NULL
      AND (
        sqlc.arg(scope_all)::boolean
        OR (sqlc.arg(is_own)::boolean AND (
            a.account_owner = sqlc.arg(uid)
            OR a.assigned_csm = sqlc.arg(uid)
            OR a.backup_csm = sqlc.arg(uid)
        ))
    )
) x
GROUP BY period
ORDER BY period;

-- name: ReportSupportKPIs :one
-- KPI header + kartu SLA panel 2. total = Total Tiket; met/with_sla = Kepatuhan
-- SLA (hanya tiket ber-SLA jadi denominator — tiket tanpa deadline tak punya
-- target untuk dipenuhi); avg_resolution_hours = Rata Penyelesaian (tiket
-- selesai). breached = terlanggar (resolved telat ATAU belum selesai lewat
-- deadline); at_risk = belum selesai, deadline < 4 jam lagi (pola CountTicketKPIs).
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL)::bigint AS with_sla,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND t.resolved_at IS NOT NULL
                       AND t.resolved_at <= t.sla_deadline_at)::bigint AS met,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND ((t.resolved_at IS NOT NULL AND t.resolved_at > t.sla_deadline_at)
                            OR (t.resolved_at IS NULL AND t.status <> 'selesai'
                                AND t.sla_deadline_at < now())))::bigint AS breached,
    COUNT(*) FILTER (WHERE t.status <> 'selesai'
                       AND t.sla_deadline_at IS NOT NULL
                       AND t.sla_deadline_at > now()
                       AND t.sla_deadline_at < now() + INTERVAL '4 hours')::bigint AS at_risk,
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
);

-- name: ReportSLAByPriority :many
-- Panel 2 tabel per-prioritas. 3 tingkat NYATA (rendah/sedang/tinggi) — mockup
-- pakai 4 (Kritis tak ada di skema). target_minutes = target penyelesaian dari
-- sla_policies via snapshot t.sla_policy_id (MAX bila banyak policy per
-- prioritas; NULL bila tiket tak ber-policy → "—" di view). Terpenuhi% =
-- met/with_sla per prioritas.
SELECT
    t.priority AS priority,
    COUNT(*)::bigint AS total,
    COALESCE(MAX(sp.resolution_target_minutes), 0)::int AS target_minutes,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL)::bigint AS with_sla,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND t.resolved_at IS NOT NULL
                       AND t.resolved_at <= t.sla_deadline_at)::bigint AS met,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND ((t.resolved_at IS NOT NULL AND t.resolved_at > t.sla_deadline_at)
                            OR (t.resolved_at IS NULL AND t.status <> 'selesai'
                                AND t.sla_deadline_at < now())))::bigint AS breached
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN sla_policies sp ON sp.id = t.sla_policy_id
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
)
GROUP BY t.priority
ORDER BY CASE t.priority
    WHEN 'tinggi' THEN 1
    WHEN 'sedang' THEN 2
    WHEN 'rendah' THEN 3
    ELSE 4
END;

-- name: ReportResolutionByPriority :many
-- Panel 3 bar. Rata jam penyelesaian per prioritas (tiket selesai). NULL bila
-- belum ada tiket selesai di prioritas itu → "—" & bar 0 di view.
SELECT
    t.priority AS priority,
    COUNT(*) FILTER (WHERE t.status = 'selesai' AND t.resolved_at IS NOT NULL)::bigint AS resolved_count,
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
GROUP BY t.priority
ORDER BY CASE t.priority
    WHEN 'tinggi' THEN 1
    WHEN 'sedang' THEN 2
    WHEN 'rendah' THEN 3
    ELSE 4
END;

-- name: ReportResolutionMonthCompare :one
-- Panel 3 baris metrik: rata jam penyelesaian tiket yang RESOLVED bulan ini vs
-- bulan lalu (batas bulan UTC via date_trunc). Label bulan dirakit handler
-- (appTZ). NULL bila tak ada tiket selesai pada bulan itu.
SELECT
    ROUND((AVG(EXTRACT(EPOCH FROM (t.resolved_at - t.created_at)) / 3600.0)
           FILTER (WHERE t.status = 'selesai' AND t.resolved_at IS NOT NULL
                     AND t.resolved_at >= date_trunc('month', now())))::numeric, 1)
        AS this_month_hours,
    ROUND((AVG(EXTRACT(EPOCH FROM (t.resolved_at - t.created_at)) / 3600.0)
           FILTER (WHERE t.status = 'selesai' AND t.resolved_at IS NOT NULL
                     AND t.resolved_at >= date_trunc('month', now()) - INTERVAL '1 month'
                     AND t.resolved_at < date_trunc('month', now())))::numeric, 1)
        AS prev_month_hours
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
);

-- name: ReportAgentPerformance :many
-- Panel 5. Per agen (t.assigned_to, join users utk nama/email). handled = tiket
-- ditangani, resolved = selesai, avg_resolution_hours (selesai), met/with_sla =
-- Kepatuhan SLA agen. Badge Status diturunkan handler (const threshold). Agen
-- NULL (belum ditugaskan) dikecualikan.
SELECT
    t.assigned_to AS agent_id,
    u.name AS agent_name,
    u.email AS agent_email,
    COUNT(*)::bigint AS handled,
    COUNT(*) FILTER (WHERE t.status = 'selesai')::bigint AS resolved,
    ROUND((AVG(EXTRACT(EPOCH FROM (t.resolved_at - t.created_at)) / 3600.0)
           FILTER (WHERE t.status = 'selesai' AND t.resolved_at IS NOT NULL))::numeric, 1)
        AS avg_resolution_hours,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL)::bigint AS with_sla,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND t.resolved_at IS NOT NULL
                       AND t.resolved_at <= t.sla_deadline_at)::bigint AS met
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
JOIN users u ON u.id = t.assigned_to
WHERE t.assigned_to IS NOT NULL
  AND (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
)
GROUP BY t.assigned_to, u.name, u.email
ORDER BY handled DESC, agent_id;

-- name: ReportKBPublished :many
-- Panel 4 (SEBAGIAN BESAR DILEWATKAN). Hanya artikel Published: Judul · Dilihat
-- (view_count apa adanya — komentar 00018: auto-increment di luar scope, praktis
-- 0) · Status. TAK ada rating/deflection/membantu (tak ada datanya). Tenant via
-- RLS (h.q). Dibatasi 50 (panel ringkas, bukan daftar penuh).
SELECT
    article_title,
    view_count::bigint AS views,
    status
FROM kb_articles
WHERE status = 'Published'
ORDER BY view_count DESC, article_title
LIMIT 50;
