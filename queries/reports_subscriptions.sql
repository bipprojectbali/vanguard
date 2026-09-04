-- reports_subscriptions.sql — Subscription Report 8.4 (Modul 8, BL-47). "Report
-- bukan objek data" (skema.md §8): NOL tabel baru, agregasi murni atas
-- subscriptions (+ plans, customer_success). F3 ownership PERSIS
-- ListSubscriptions/SubscriptionsListFilterFor (subscription_owner) — sumber
-- SATU dengan modul asal, bukan duplikat logic scope. RLS mengurung tenant.
--
-- KPI churn rate + panel breakdown Alasan REUSE query yang sudah ada di
-- reports.sql (ReportRetention active/churned + ReportChurnReasons GROUP BY
-- churn_reason) — tak ditulis ulang. File ini menambah MRR movement, Renewal,
-- Revenue-by-Plan, & Aging.
--
-- DILEWATKAN (keputusan sadar BL-47, "data belum ada DILEWATKAN dulu"): tren
-- MRR bulanan historis (Apr–Agu) & delta "vs bulan lalu" di kartu MRR —
-- subscriptions.mrr = nilai SEKARANG (bukan snapshot per bulan lampau);
-- rekonstruksi mundur salah utk sub yang sudah di-renew beda MRR, dan tak ada
-- tabel snapshot MRR historis. Butuh tabel + job terjadwal (BL sendiri).
--
-- Semua agregat dibungkus COALESCE(...)::tipe agar sqlc tak emit interface{}
-- (gotcha #14). Jendela "bulan ini" dari sqlc.arg(today)::date (appTZ-aware,
-- dioper handler — hindari AT TIME ZONE di SELECT list sqlc).

-- name: ReportSubMRR :one
-- KPI MRR/ARR + Panel 1 (MRR Movement): nilai berjalan (Active) + 4 komponen
-- pergerakan BULAN INI, EKSAK dari previous_value + status + start_date:
--   • MRR baru      = start_date bulan ini & TANPA previous_subscription_id.
--   • Ekspansi      = renewal (previous_subscription_id NOT NULL) mulai bulan
--                     ini dgn mrr > previous_value → nilai = SUM(mrr−previous).
--   • Kontraksi     = renewal mulai bulan ini dgn mrr < previous_value (BUKAN
--                     churn) → nilai = SUM(previous−mrr) (positif, penyusutan).
--   • Churn         = Cancelled/Churned dgn cancellation_date bulan ini →
--                     nilai = SUM(lost_value_mrr).
-- ARR = kolom arr bila ada, else mrr×12 (billing annual bisa diskon → arr≠×12).
SELECT
    COALESCE(SUM(mrr) FILTER (WHERE status = 'Active'), 0)::numeric                    AS mrr_active,
    COALESCE(SUM(COALESCE(arr, mrr * 12)) FILTER (WHERE status = 'Active'), 0)::numeric AS arr_active,
    COUNT(*) FILTER (WHERE status = 'Active')::bigint                                  AS active_count,

    COALESCE(SUM(mrr) FILTER (
        WHERE previous_subscription_id IS NULL
          AND start_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND start_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    ), 0)::numeric AS new_mrr,
    COUNT(*) FILTER (
        WHERE previous_subscription_id IS NULL
          AND start_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND start_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    )::bigint AS new_count,

    COALESCE(SUM(mrr - previous_value) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr > previous_value
          AND start_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND start_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    ), 0)::numeric AS expansion_mrr,
    COUNT(*) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr > previous_value
          AND start_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND start_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    )::bigint AS expansion_count,

    COALESCE(SUM(previous_value - mrr) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr < previous_value
          AND status NOT IN ('Cancelled', 'Churned')
          AND start_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND start_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    ), 0)::numeric AS contraction_mrr,
    COUNT(*) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr < previous_value
          AND status NOT IN ('Cancelled', 'Churned')
          AND start_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND start_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    )::bigint AS contraction_count,

    COALESCE(SUM(lost_value_mrr) FILTER (
        WHERE status IN ('Cancelled', 'Churned')
          AND cancellation_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND cancellation_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    ), 0)::numeric AS churn_mrr,
    COUNT(*) FILTER (
        WHERE status IN ('Cancelled', 'Churned')
          AND cancellation_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND cancellation_date <  (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date
    )::bigint AS churn_count
FROM subscriptions
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND subscription_owner = sqlc.arg(uid))
  );

-- name: ReportRenewalSummary :one
-- KPI Renewal Rate + Panel 2 kartu: rate = diperpanjang / jatuh-tempo (hanya
-- yang SUDAH jatuh tempo, end_date < today — masa depan belum bisa diperpanjang).
-- "diperpanjang" = ada baris renewal anak (previous_subscription_id menunjuk
-- balik). renewed_value = SUM(mrr) SEMUA baris renewal (termasuk upsell).
-- due_30 = Active dgn end_date dalam 30 hari ke depan (index idx_subs_renewal).
SELECT
    COUNT(*) FILTER (
        WHERE s.end_date IS NOT NULL AND s.end_date < sqlc.arg(today)::date
    )::bigint AS due_past,
    COUNT(*) FILTER (
        WHERE s.end_date IS NOT NULL AND s.end_date < sqlc.arg(today)::date
          AND EXISTS (
              SELECT 1 FROM subscriptions r
              WHERE r.previous_subscription_id = s.id AND r.deleted_at IS NULL
          )
    )::bigint AS renewed_past,
    COALESCE(SUM(s.mrr) FILTER (WHERE s.previous_subscription_id IS NOT NULL), 0)::numeric AS renewed_value,
    COUNT(*) FILTER (
        WHERE s.status = 'Active' AND s.end_date IS NOT NULL
          AND s.end_date >= sqlc.arg(today)::date
          AND s.end_date <  sqlc.arg(today)::date + 30
    )::bigint AS due_30
FROM subscriptions s
WHERE s.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  );

-- name: ReportRenewalByMonth :many
-- Panel 2 tabel bulanan (Periode · Jatuh Tempo · Diperpanjang · Rate): bucket
-- dari end_date (fakta tersimpan, bukan rekonstruksi). diperpanjang = ada baris
-- renewal anak. Rate dihitung handler (renewed/due). Diurut kronologis.
SELECT
    date_trunc('month', s.end_date)::date              AS period,
    COUNT(*)::bigint                                    AS due,
    COUNT(*) FILTER (WHERE EXISTS (
        SELECT 1 FROM subscriptions r
        WHERE r.previous_subscription_id = s.id AND r.deleted_at IS NULL
    ))::bigint                                          AS renewed
FROM subscriptions s
WHERE s.deleted_at IS NULL
  AND s.end_date IS NOT NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
GROUP BY date_trunc('month', s.end_date)
ORDER BY period;

-- name: ReportChurnAge :one
-- Panel 3 kartu Nilai Hilang + Rata Umur: total lost_value_mrr & rata umur
-- (cancellation_date − start_date, hari) atas langganan Cancelled/Churned.
-- aged_count = denominator guard rata umur (kedua tanggal terisi). Jumlah desa
-- churn & breakdown alasan REUSE ReportRetention/ReportChurnReasons.
SELECT
    COALESCE(SUM(lost_value_mrr) FILTER (WHERE status IN ('Cancelled', 'Churned')), 0)::numeric AS lost_value,
    COUNT(*) FILTER (
        WHERE status IN ('Cancelled', 'Churned')
          AND cancellation_date IS NOT NULL AND start_date IS NOT NULL
    )::bigint AS aged_count,
    ROUND(AVG(cancellation_date - start_date) FILTER (
        WHERE status IN ('Cancelled', 'Churned')
          AND cancellation_date IS NOT NULL AND start_date IS NOT NULL
    )::numeric, 0) AS avg_age_days
FROM subscriptions
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND subscription_owner = sqlc.arg(uid))
  );

-- name: ReportRevenueByPlan :many
-- Panel 4 (Revenue by Plan): JOIN subscriptions × plans GROUP BY plan_id atas
-- langganan Active. Nama paket dari plans.plan_name (apa pun yang di-seed
-- tenant, BUKAN hardcode). Rata per Desa dihitung handler (mrr/desa). Diurut
-- MRR terbesar. RLS mengurung tenant di kedua tabel.
SELECT
    p.plan_name                                 AS plan_name,
    COUNT(*)::bigint                            AS village_count,
    COALESCE(SUM(s.mrr), 0)::numeric            AS mrr
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.deleted_at IS NULL
  AND s.status = 'Active'
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
GROUP BY p.id, p.plan_name
ORDER BY mrr DESC, p.plan_name;

-- name: ReportSubscriptionAging :many
-- Panel 5 (Subscription Aging): bucket umur (today − start_date) atas langganan
-- BER-start_date. Per bucket: desa Active · MRR (Active) · rata health
-- (customer_success.overall_health_score, agregat — bukan PII per-desa) ·
-- jatuh-tempo/diperpanjang/churned (untuk rate handler). Bucket key numerik
-- (1..4) agar handler beri label + Catatan interpretatif (const bernama).
--   1: <6 bln (<183 hari) · 2: 6–12 bln (<366) · 3: 1–2 thn (<731) · 4: >2 thn
SELECT
    CASE
        WHEN sqlc.arg(today)::date - s.start_date < 183 THEN 1
        WHEN sqlc.arg(today)::date - s.start_date < 366 THEN 2
        WHEN sqlc.arg(today)::date - s.start_date < 731 THEN 3
        ELSE 4
    END::int AS bucket,
    COUNT(*) FILTER (WHERE s.status = 'Active')::bigint                 AS active_villages,
    COALESCE(SUM(s.mrr) FILTER (WHERE s.status = 'Active'), 0)::numeric AS mrr,
    ROUND(AVG(cs.overall_health_score)::numeric, 0)                     AS avg_health,
    COUNT(*) FILTER (WHERE s.end_date IS NOT NULL AND s.end_date < sqlc.arg(today)::date)::bigint AS due_past,
    COUNT(*) FILTER (
        WHERE s.end_date IS NOT NULL AND s.end_date < sqlc.arg(today)::date
          AND EXISTS (
              SELECT 1 FROM subscriptions r
              WHERE r.previous_subscription_id = s.id AND r.deleted_at IS NULL
          )
    )::bigint AS renewed_past,
    COUNT(*) FILTER (WHERE s.status IN ('Cancelled', 'Churned'))::bigint AS churned,
    COUNT(*)::bigint                                                     AS total
FROM subscriptions s
LEFT JOIN customer_success cs
       ON cs.account_id = s.account_id AND cs.tenant_id = s.tenant_id
WHERE s.deleted_at IS NULL
  AND s.start_date IS NOT NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
GROUP BY bucket
ORDER BY bucket;
