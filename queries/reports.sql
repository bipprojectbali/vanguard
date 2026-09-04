-- reports.sql — preset report read-only Modul 8 (tasks.md M8-1). "Report bukan
-- objek data" (skema.md §8): nol tabel baru, query agregasi murni atas tabel yang
-- sudah ada. Ownership (F3) pakai flag SAMA dengan modul asal tabel — sumber SATU,
-- bukan duplikat logic scope (pola sama dengan dashboard.sql).

-- name: ReportPipelineByStage :many
-- Sales Report (wireframe 8.1 panel 1): SEMUA stage TERMASUK Closed Won/Lost —
-- beda sengaja dari DashboardPipelineByStage (yang exclude keduanya untuk chart
-- funnel). Report butuh gambaran penuh pipeline+hasil, bukan cuma yang terbuka.
-- BL-43: avg_probability = rata probabilitas per stage (baris probability NULL
-- dilewati AVG); weighted_value = SUM(amount×probability/100) — nilai pipeline
-- tertimbang. COALESCE(...)::tipe membungkus tiap agregat agar sqlc tak emit
-- interface{} (gotcha #14); avg dibulatkan ke bilangan bulat (persen).
-- BL-49: filter opsional Periode (created_at) + Owner (deal_owner), guard NULL =
-- tak menyaring. owner_filter di-AND DI ATAS scope (hanya menyempit).
SELECT
    stage,
    COUNT(*)::bigint                  AS deal_count,
    COALESCE(SUM(amount), 0)::numeric AS stage_value,
    COALESCE(ROUND(AVG(probability), 0), 0)::bigint         AS avg_probability,
    COALESCE(SUM(amount * probability / 100.0), 0)::numeric AS weighted_value
FROM deals
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
  AND (sqlc.narg(period_start)::timestamptz IS NULL OR created_at >= sqlc.narg(period_start))
  AND (sqlc.narg(period_end)::timestamptz IS NULL OR created_at < sqlc.narg(period_end))
  AND (sqlc.narg(owner_filter)::bigint IS NULL OR deal_owner = sqlc.narg(owner_filter))
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


-- ── Customer Success Report (wireframe 8.2, BL-45) ──────────────────────────
-- Enam panel spec 8.2; DIBANGUN hanya yang datanya SUDAH ADA (skema.md §8: nol
-- tabel baru, agregasi murni). DILEWATKAN (tak dirender): NPS/CSAT (tak ada
-- tabel surveys), tren health bulanan (tak ada snapshot per bulan), adopsi
-- per-fitur & kategori power-user (tak ada model pemakaian per-fitur),
-- onboarding per-tahap (tak ada timestamp tahap antara), net retention (tak ada
-- delta ekspansi MRR). Tiga bentuk F3 ownership berbeda karena tiga sumber:
--   • Health/Adoption/Onboarding (accounts+customer_success): scope_all/is_csm/
--     is_sales — PERSIS CountHealthScoreKPIs (health_score.sql), tak divergen
--     dari /health-scores.
--   • Retention/Churn (subscriptions.subscription_owner): scope_all/is_own —
--     PERSIS ListSubscriptions/SubscriptionsListFilterFor.
--   • Engagement (engagements JOIN accounts): scope_all/is_own atas kolom
--     kepemilikan account — PERSIS ListEngagements/EngagementsListFilterFor.

-- name: ReportCSHealth :one
-- Panel 1 (Health Score Report) + KPI Rata Health Score: rata skor + distribusi
-- band desa (Sehat 80–100 / Cukup 60–79 / Berisiko 40–59 / Kritis <40).
-- Band hanya menghitung desa BER-skor (overall_health_score NOT NULL — perban-
-- dingan NULL menghasilkan NULL, tak masuk FILTER). scored = denominator persen.
SELECT
    COUNT(*) FILTER (WHERE cs.overall_health_score IS NOT NULL)                              AS scored,
    ROUND(AVG(cs.overall_health_score)::numeric, 1)                                          AS avg_health,
    COUNT(*) FILTER (WHERE cs.overall_health_score >= 80)                                    AS band_healthy,
    COUNT(*) FILTER (WHERE cs.overall_health_score >= 60 AND cs.overall_health_score < 80)   AS band_fair,
    COUNT(*) FILTER (WHERE cs.overall_health_score >= 40 AND cs.overall_health_score < 60)   AS band_at_risk,
    COUNT(*) FILTER (WHERE cs.overall_health_score < 40)                                     AS band_critical
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

-- name: ReportCSAdoption :one
-- Panel 2 (Adoption Report, versi sederhana) + KPI Adoption Rate: rata
-- feature_adoption_rate + distribusi band (Tinggi 80–100 / Sedang 60–79 /
-- Rendah 40–59 / Sangat Rendah <40). DILEWATKAN (tak di query ini): bar
-- per-fitur & tabel kategori power-user — tak ada model pemakaian per-fitur
-- (hanya feature_adoption_rate numerik tunggal). F3 identik ReportCSHealth.
SELECT
    COUNT(*) FILTER (WHERE cs.feature_adoption_rate IS NOT NULL)                                 AS scored,
    ROUND(AVG(cs.feature_adoption_rate)::numeric, 1)                                             AS avg_adoption,
    COUNT(*) FILTER (WHERE cs.feature_adoption_rate >= 80)                                       AS band_high,
    COUNT(*) FILTER (WHERE cs.feature_adoption_rate >= 60 AND cs.feature_adoption_rate < 80)     AS band_mid,
    COUNT(*) FILTER (WHERE cs.feature_adoption_rate >= 40 AND cs.feature_adoption_rate < 60)     AS band_low,
    COUNT(*) FILTER (WHERE cs.feature_adoption_rate < 40)                                        AS band_poor
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

-- name: ReportRetention :one
-- Panel 3 (Retention/Churn) kartu + KPI Retention Rate: hitung aktif vs churned
-- dari subscriptions.status. Retention% & Churn% dihitung handler dari kedua
-- angka (active/(active+churned)). F3 ownership subscription_owner (PERSIS
-- ListSubscriptions). RLS mengurung tenant.
SELECT
    COUNT(*) FILTER (WHERE s.status = 'Active')                       AS active,
    COUNT(*) FILTER (WHERE s.status IN ('Cancelled', 'Churned'))      AS churned
FROM subscriptions s
WHERE s.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  );

-- name: ReportChurnReasons :many
-- Panel 3 tabel Alasan Churn: GROUP BY churn_reason (SUDAH picklist di 00012)
-- atas langganan Cancelled/Churned, dengan lost_value_mrr sebagai Nilai Hilang
-- (di-mask F4 di handler). churn_reason NULL → '(Tanpa alasan)'. Diurut jumlah
-- terbanyak. F3 identik ReportRetention.
SELECT
    COALESCE(s.churn_reason, '(Tanpa alasan)')  AS churn_reason,
    COUNT(*)::bigint                            AS account_count,
    COALESCE(SUM(s.lost_value_mrr), 0)::numeric AS lost_value
FROM subscriptions s
WHERE s.deleted_at IS NULL
  AND s.status IN ('Cancelled', 'Churned')
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
GROUP BY COALESCE(s.churn_reason, '(Tanpa alasan)')
ORDER BY account_count DESC, churn_reason;

-- name: ReportOnboarding :one
-- Panel 5 (Onboarding Report): rata durasi (actual_go_live − kickoff, hari) +
-- Selesai + Terlambat + distribusi onboarding_status. Terlambat = go-live nyata
-- melewati target ATAU onboarding belum selesai tapi kickoff >30 hari lalu
-- (today dioper handler, appTZ-aware; hindari AT TIME ZONE di SELECT sqlc,
-- gotcha #14). DILEWATKAN: tabel per-tahap (tak ada timestamp tahap antara).
-- F3 identik ReportCSHealth. completed_with_dates = denominator guard rata durasi.
SELECT
    COUNT(*) FILTER (WHERE cs.onboarding_status IS NOT NULL)              AS total,
    COUNT(*) FILTER (WHERE cs.onboarding_status = 'Completed')            AS completed,
    COUNT(*) FILTER (WHERE cs.onboarding_status = 'Not Started')          AS not_started,
    COUNT(*) FILTER (WHERE cs.onboarding_status = 'In Progress')          AS in_progress,
    COUNT(*) FILTER (WHERE cs.onboarding_status = 'Stalled')              AS stalled,
    COUNT(*) FILTER (WHERE cs.actual_go_live_date IS NOT NULL
                       AND cs.kickoff_date IS NOT NULL)                   AS completed_with_dates,
    ROUND(AVG(cs.actual_go_live_date - cs.kickoff_date)
          FILTER (WHERE cs.actual_go_live_date IS NOT NULL
                    AND cs.kickoff_date IS NOT NULL)::numeric, 1)         AS avg_duration_days,
    COUNT(*) FILTER (
        WHERE (cs.actual_go_live_date IS NOT NULL
               AND cs.target_go_live_date IS NOT NULL
               AND cs.actual_go_live_date > cs.target_go_live_date)
           OR (cs.onboarding_status IN ('Not Started', 'In Progress', 'Stalled')
               AND cs.kickoff_date IS NOT NULL
               AND cs.kickoff_date < sqlc.arg(today)::date - 30)
    )                                                                    AS late
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

-- name: ReportEngagementCompliance :many
-- Panel 6 (Engagement Report) per engagement_type: kepatuhan = done / total
-- terjadwal (persen dihitung handler). F3 ownership atas kolom kepemilikan
-- account (PERSIS ListEngagements). RLS mengurung tenant.
SELECT
    e.engagement_type                                    AS engagement_type,
    COUNT(*)::bigint                                     AS total,
    COUNT(*) FILTER (WHERE e.status = 'done')::bigint    AS done
FROM engagements e
JOIN accounts a ON e.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
)
GROUP BY e.engagement_type
ORDER BY e.engagement_type;

-- name: ReportEngagementByCSM :many
-- Panel 6 tabel per-CSM: Desa Dipegang (accounts.assigned_csm = owner, subquery
-- skalar bergantung kolom grup) · Touch Point done/total · kepatuhan% (handler).
-- GROUP BY engagements.owner_id. owner_id NULL (belum ditugaskan) → nama '—' di
-- handler. F3 identik ReportEngagementCompliance.
SELECT
    e.owner_id                                          AS owner_id,
    u.name                                              AS owner_name,
    u.email                                             AS owner_email,
    COUNT(*)::bigint                                    AS total,
    COUNT(*) FILTER (WHERE e.status = 'done')::bigint   AS done,
    (SELECT COUNT(*)::bigint FROM accounts ac
      WHERE ac.assigned_csm = e.owner_id
        AND ac.deleted_at IS NULL)                      AS accounts_assigned
FROM engagements e
JOIN accounts a ON e.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON e.owner_id = u.id
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
)
GROUP BY e.owner_id, u.name, u.email
ORDER BY total DESC, owner_id;

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
