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
--
-- FILTER interaktif Periode + Paket (BL-52): SEMANTIK SENGAJA PER-PANEL, bukan
-- kolom waktu seragam.
--   • period_start/period_end (timestamptz narg, [start,end)) memotong panel
--     BER-DIMENSI-WAKTU di kolom yang BENAR: pergerakan MRR baru/ekspansi/
--     kontraksi by start_date; churn (komponen MRR & breakdown) by
--     cancellation_date; renewal by end_date. Bila NULL → perilaku BL-47
--     (pergerakan MRR jatuh ke jendela "bulan ini" via COALESCE default).
--   • Panel SNAPSHOT (MRR/ARR berjalan, Revenue-by-Plan, Aging) = nilai
--     SEKARANG, TAK ber-dimensi-waktu → period_* TAK diterapkan (hanya Paket).
--   • plan_filter (bigint narg) menyaring SEMUA panel ke satu plan_id; NULL →
--     semua paket. Di-AND DI ATAS scope F3 (menyempit, tak melebarkan).

-- name: ReportSubMRR :one
-- KPI MRR/ARR + Panel 1 (MRR Movement): nilai berjalan (Active) + 4 komponen
-- pergerakan, EKSAK dari previous_value + status + start_date:
--   • MRR baru      = start_date dalam jendela & TANPA previous_subscription_id.
--   • Ekspansi      = renewal (previous_subscription_id NOT NULL) mulai dalam
--                     jendela dgn mrr > previous_value → nilai = SUM(mrr−previous).
--   • Kontraksi     = renewal mulai dalam jendela dgn mrr < previous_value (BUKAN
--                     churn) → nilai = SUM(previous−mrr) (positif, penyusutan).
--   • Churn         = Cancelled/Churned dgn cancellation_date dalam jendela →
--                     nilai = SUM(lost_value_mrr).
-- Jendela pergerakan = [period_start,period_end) bila diset (BL-52), else "bulan
-- ini" (BL-47 default via COALESCE). MRR/ARR berjalan = SNAPSHOT (tak ber-jendela).
-- ARR = kolom arr bila ada, else mrr×12 (billing annual bisa diskon → arr≠×12).
SELECT
    COALESCE(SUM(mrr) FILTER (WHERE status = 'Active'), 0)::numeric                    AS mrr_active,
    COALESCE(SUM(COALESCE(arr, mrr * 12)) FILTER (WHERE status = 'Active'), 0)::numeric AS arr_active,
    COUNT(*) FILTER (WHERE status = 'Active')::bigint                                  AS active_count,

    COALESCE(SUM(mrr) FILTER (
        WHERE previous_subscription_id IS NULL
          AND start_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND start_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    ), 0)::numeric AS new_mrr,
    COUNT(*) FILTER (
        WHERE previous_subscription_id IS NULL
          AND start_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND start_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    )::bigint AS new_count,

    COALESCE(SUM(mrr - previous_value) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr > previous_value
          AND start_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND start_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    ), 0)::numeric AS expansion_mrr,
    COUNT(*) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr > previous_value
          AND start_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND start_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    )::bigint AS expansion_count,

    COALESCE(SUM(previous_value - mrr) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr < previous_value
          AND status NOT IN ('Cancelled', 'Churned')
          AND start_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND start_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    ), 0)::numeric AS contraction_mrr,
    COUNT(*) FILTER (
        WHERE previous_subscription_id IS NOT NULL AND previous_value IS NOT NULL
          AND mrr < previous_value
          AND status NOT IN ('Cancelled', 'Churned')
          AND start_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND start_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    )::bigint AS contraction_count,

    COALESCE(SUM(lost_value_mrr) FILTER (
        WHERE status IN ('Cancelled', 'Churned')
          AND cancellation_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND cancellation_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    ), 0)::numeric AS churn_mrr,
    COUNT(*) FILTER (
        WHERE status IN ('Cancelled', 'Churned')
          AND cancellation_date >= COALESCE(sqlc.narg(period_start)::timestamptz::date, date_trunc('month', sqlc.arg(today)::date)::date)
          AND cancellation_date <  COALESCE(sqlc.narg(period_end)::timestamptz::date, (date_trunc('month', sqlc.arg(today)::date) + interval '1 month')::date)
    )::bigint AS churn_count
FROM subscriptions
WHERE deleted_at IS NULL
  AND (sqlc.narg(plan_filter)::bigint IS NULL OR EXISTS (
      SELECT 1 FROM subscription_items si
      WHERE si.subscription_id = subscriptions.id AND si.plan_id = sqlc.narg(plan_filter)
  ))
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND subscription_owner = sqlc.arg(uid))
  );

-- name: ReportRenewalSummary :one
-- KPI Renewal Rate + Panel 2 kartu: rate = diperpanjang / jatuh-tempo (hanya
-- yang SUDAH jatuh tempo, end_date < today — masa depan belum bisa diperpanjang).
-- "diperpanjang" = ada baris renewal anak (previous_subscription_id menunjuk
-- balik). renewed_value = SUM(mrr) baris renewal. due_30 = Active dgn end_date
-- dalam 30 hari ke depan (index idx_subs_renewal).
-- BL-52: Periode memotong jatuh-tempo/diperpanjang di end_date (kohort yang
-- jatuh tempo dalam rentang); renewed_value memakai start_date anak renewal
-- (kapan perpanjangan terjadi). due_30 = SNAPSHOT forward-looking dari today
-- (inheren relatif; Periode TAK berlaku). Paket menyaring semua.
SELECT
    COUNT(*) FILTER (
        WHERE s.end_date IS NOT NULL AND s.end_date < sqlc.arg(today)::date
          AND (sqlc.narg(period_start)::timestamptz IS NULL OR s.end_date >= sqlc.narg(period_start)::timestamptz::date)
          AND (sqlc.narg(period_end)::timestamptz IS NULL OR s.end_date < sqlc.narg(period_end)::timestamptz::date)
    )::bigint AS due_past,
    COUNT(*) FILTER (
        WHERE s.end_date IS NOT NULL AND s.end_date < sqlc.arg(today)::date
          AND (sqlc.narg(period_start)::timestamptz IS NULL OR s.end_date >= sqlc.narg(period_start)::timestamptz::date)
          AND (sqlc.narg(period_end)::timestamptz IS NULL OR s.end_date < sqlc.narg(period_end)::timestamptz::date)
          AND EXISTS (
              SELECT 1 FROM subscriptions r
              WHERE r.previous_subscription_id = s.id AND r.deleted_at IS NULL
          )
    )::bigint AS renewed_past,
    COALESCE(SUM(s.mrr) FILTER (
        WHERE s.previous_subscription_id IS NOT NULL
          AND (sqlc.narg(period_start)::timestamptz IS NULL OR s.start_date >= sqlc.narg(period_start)::timestamptz::date)
          AND (sqlc.narg(period_end)::timestamptz IS NULL OR s.start_date < sqlc.narg(period_end)::timestamptz::date)
    ), 0)::numeric AS renewed_value,
    COUNT(*) FILTER (
        WHERE s.status = 'Active' AND s.end_date IS NOT NULL
          AND s.end_date >= sqlc.arg(today)::date
          AND s.end_date <  sqlc.arg(today)::date + 30
    )::bigint AS due_30
FROM subscriptions s
WHERE s.deleted_at IS NULL
  AND (sqlc.narg(plan_filter)::bigint IS NULL OR EXISTS (
      SELECT 1 FROM subscription_items si
      WHERE si.subscription_id = s.id AND si.plan_id = sqlc.narg(plan_filter)
  ))
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  );

-- name: ReportRenewalByMonth :many
-- Panel 2 tabel bulanan (Periode · Jatuh Tempo · Diperpanjang · Rate): bucket
-- dari end_date (fakta tersimpan, bukan rekonstruksi). diperpanjang = ada baris
-- renewal anak. Rate dihitung handler (renewed/due). Diurut kronologis.
-- BL-52: Periode memotong end_date dalam [start,end); Paket menyaring plan_id.
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
  AND (sqlc.narg(period_start)::timestamptz IS NULL OR s.end_date >= sqlc.narg(period_start)::timestamptz::date)
  AND (sqlc.narg(period_end)::timestamptz IS NULL OR s.end_date < sqlc.narg(period_end)::timestamptz::date)
  AND (sqlc.narg(plan_filter)::bigint IS NULL OR EXISTS (
      SELECT 1 FROM subscription_items si
      WHERE si.subscription_id = s.id AND si.plan_id = sqlc.narg(plan_filter)
  ))
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
-- BL-52: Periode memotong cancellation_date dalam [start,end) (kohort churn di
-- rentang); Paket menyaring plan_id.
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
  AND (sqlc.narg(period_start)::timestamptz IS NULL OR cancellation_date >= sqlc.narg(period_start)::timestamptz::date)
  AND (sqlc.narg(period_end)::timestamptz IS NULL OR cancellation_date < sqlc.narg(period_end)::timestamptz::date)
  AND (sqlc.narg(plan_filter)::bigint IS NULL OR EXISTS (
      SELECT 1 FROM subscription_items si
      WHERE si.subscription_id = subscriptions.id AND si.plan_id = sqlc.narg(plan_filter)
  ))
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND subscription_owner = sqlc.arg(uid))
  );

-- name: ReportRevenueByPlan :many
-- Panel 4 (Revenue by Plan): BL-88 PR2b — agregasi per-produk PINDAH ke
-- subscription_items (langganan kini multi-paket; s.plan_id tak lagi otoritatif).
-- Baris = item ber-parent Active (parent_active). MRR = SUM(item.mrr); village_count =
-- desa DISTINCT (invarian 1 item Active per account+plan → tepat 1 item/desa/paket,
-- tapi DISTINCT eksplisit tetap benar bila invarian berubah). Nama paket dari plans
-- (di-seed tenant). Rata per Desa dihitung handler. Diurut MRR terbesar. RLS mengurung
-- tenant. BL-52: SNAPSHOT (Active SEKARANG) → Periode TAK diterapkan; Paket menyaring.
SELECT
    p.plan_name                                 AS plan_name,
    COUNT(DISTINCT si.account_id)::bigint       AS village_count,
    COALESCE(SUM(si.mrr), 0)::numeric           AS mrr
FROM subscription_items si
JOIN plans p ON p.id = si.plan_id
WHERE si.parent_active
  AND (sqlc.narg(plan_filter)::bigint IS NULL OR si.plan_id = sqlc.narg(plan_filter))
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND EXISTS (
          SELECT 1 FROM subscriptions s
          WHERE s.id = si.subscription_id AND s.subscription_owner = sqlc.arg(uid)
      ))
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
-- BL-52: SNAPSHOT (umur & nilai SEKARANG) → Periode TAK diterapkan; Paket
-- menyaring ke plan_id terpilih.
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
  AND (sqlc.narg(plan_filter)::bigint IS NULL OR EXISTS (
      SELECT 1 FROM subscription_items si
      WHERE si.subscription_id = s.id AND si.plan_id = sqlc.narg(plan_filter)
  ))
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
GROUP BY bucket
ORDER BY bucket;

-- name: ReportSubscriptionPlans :many
-- BL-52: isi dropdown "Paket" Subscription Report — DITURUNKAN DARI DATA (paket
-- yang benar-benar dipakai langganan dalam cakupan pemakai), bukan daftar plans
-- penuh. Pola sama ReportSalesOwners: "pilihan yang pasti kosong lebih buruk
-- daripada pilihan yang tak ada". F3 pakai flag subscription yang SAMA
-- (SubscriptionsListFilterFor). TAK disaring Periode/Paket agar daftar stabil
-- (paket terpilih selalu tampil walau rentang dipersempit).
-- BL-88 PR2b: paket dari subscription_items (langganan multi-paket → tiap paket
-- yang dipakai muncul; parent plan_id bisa NULL). subscription_count = langganan
-- DISTINCT yang memuat paket itu.
SELECT
    p.id::bigint                                AS plan_id,
    p.plan_name                                 AS plan_name,
    COUNT(DISTINCT si.subscription_id)::bigint  AS subscription_count
FROM subscription_items si
JOIN plans p ON p.id = si.plan_id
JOIN subscriptions s ON s.id = si.subscription_id
WHERE s.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
GROUP BY p.id, p.plan_name
ORDER BY p.plan_name;
