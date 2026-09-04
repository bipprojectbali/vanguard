-- reports_sales.sql — agregasi panel Sales Report 8.1 (BL-43) yang BUKAN pipeline
-- per-stage (itu di reports.sql). "Report bukan objek data" (skema.md §8): nol
-- tabel baru, query agregasi murni atas tabel yang sudah ada. F3 ownership pakai
-- flag SAMA dengan modul asal (DealsListFilterFor/LeadsListFilterFor/
-- ActivitiesListFilterFor) — pola `scope_all OR (is_own AND owner = uid)`,
-- fail-closed (kedua flag false → nol baris). Empat gap yang butuh schema
-- (Target/Batal/picklist Alasan/qualified_at) SENGAJA di luar file ini → BL-44.

-- name: ReportSalesForecast :many
-- Panel 2 (Sales Forecast): bucket per BULAN dari expected_close_date, nilai =
-- SUM tertimbang (amount×probability/100). Hanya deal TERBUKA (exclude Closed
-- Won/Lost) — forecast = ekspektasi pendapatan yang BELUM terealisasi; Won sudah
-- masuk, Lost = 0. period 'YYYY-MM' agar urut leksikografis = kronologis. Kolom
-- Target DITUNDA ke BL-44 (tak ada tabel target).
SELECT
    to_char(date_trunc('month', expected_close_date), 'YYYY-MM')   AS period,
    COALESCE(SUM(amount * probability / 100.0), 0)::numeric         AS weighted_value
FROM deals
WHERE deleted_at IS NULL
  AND expected_close_date IS NOT NULL
  AND stage NOT IN ('Closed Won', 'Closed Lost')
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
GROUP BY date_trunc('month', expected_close_date)
ORDER BY date_trunc('month', expected_close_date);

-- name: ReportWinLossReasons :many
-- Panel 3 (tabel Alasan Kalah): GROUP BY win_loss_reason atas deal Closed Lost.
-- win_loss_reason = TEKS BEBAS (00009) → grouping apa adanya, rawan variasi ejaan
-- (versi picklist bersih = BL-44). Reason kosong/whitespace dipetakan ke penanda
-- eksplisit agar tetap satu baris terhitung. Porsi% dihitung di Go (butuh total).
SELECT
    (COALESCE(NULLIF(TRIM(win_loss_reason), ''), '(Tanpa alasan)'))::text AS reason,
    COUNT(*)::bigint                                                      AS deal_count
FROM deals
WHERE deleted_at IS NULL
  AND stage = 'Closed Lost'
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
GROUP BY COALESCE(NULLIF(TRIM(win_loss_reason), ''), '(Tanpa alasan)')
ORDER BY COUNT(*) DESC, reason;

-- name: ReportLeadFunnel :one
-- Panel 4 (funnel Lead Conversion) sisi LEADS: total lead masuk, terkualifikasi
-- (Qualified atau sudah Converted — keduanya lolos kualifikasi), jadi deal
-- (converted + punya converted_deal_id). avg_days_to_deal = rata (converted_at −
-- created_at) hari, HANYA lead terkonversi. Rata Waktu Lead→Terkualifikasi
-- DITUNDA ke BL-44 (leads tak simpan qualified_at). ROUND(...,1) + COALESCE agar
-- sqlc tak emit interface{} (gotcha #14).
SELECT
    COUNT(*)::bigint AS total_leads,
    COUNT(*) FILTER (WHERE lead_status IN ('Qualified', 'Converted'))::bigint AS qualified_count,
    COUNT(*) FILTER (WHERE converted AND converted_deal_id IS NOT NULL)::bigint AS deal_count,
    COALESCE(ROUND(
        AVG(EXTRACT(EPOCH FROM (converted_at - created_at)) / 86400.0)
        FILTER (WHERE converted AND converted_at IS NOT NULL), 1), 0)::numeric AS avg_days_to_deal
FROM leads
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  );

-- name: ReportWonTiming :one
-- Panel 4 (funnel) sisi DEALS: jumlah Closed Won + rata waktu Deal→Menang
-- (closed_date − created_at) hari. closed_date DATE − created_at::date = int hari.
-- Hanya deal yang punya closed_date dihitung untuk rata.
SELECT
    COUNT(*)::bigint AS won_count,
    COALESCE(ROUND(
        AVG(closed_date - created_at::date)
        FILTER (WHERE closed_date IS NOT NULL), 1), 0)::numeric AS avg_days_to_won
FROM deals
WHERE deleted_at IS NULL
  AND stage = 'Closed Won'
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  );

-- name: ReportSalesActivityByOwner :many
-- Panel 5 (Sales Activity Report): per-owner hitung aktivitas context='sales'
-- per kind (call/email/meeting) + total. LEFT JOIN users utk nama tampilan (pola
-- cs_impl_tasks.sql). owner_id nullable (NULL = tanpa pemilik → satu grup). Deal
-- Menang & Aktivitas/Deal digabung di Go dari ReportWonDealsByOwner (hindari
-- fan-out cross-join). ORDER total DESC agar sales paling aktif di atas.
SELECT
    a.owner_id                                              AS owner_id,
    u.name                                                  AS owner_name,
    u.email                                                 AS owner_email,
    COUNT(*) FILTER (WHERE a.kind = 'call')::bigint         AS call_count,
    COUNT(*) FILTER (WHERE a.kind = 'email')::bigint        AS email_count,
    COUNT(*) FILTER (WHERE a.kind = 'meeting')::bigint      AS meeting_count,
    COUNT(*)::bigint                                        AS total_count
FROM activities a
LEFT JOIN users u ON u.id = a.owner_id
WHERE a.deleted_at IS NULL
  AND a.activity_context = 'sales'
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND a.owner_id = sqlc.arg(uid))
  )
GROUP BY a.owner_id, u.name, u.email
ORDER BY total_count DESC, a.owner_id;

-- name: ReportWonDealsByOwner :many
-- Panel 5 pendamping: jumlah deal Closed Won per deal_owner. Digabung di Go by
-- owner id dengan ReportSalesActivityByOwner untuk kolom Deal Menang &
-- Aktivitas/Deal. F3 pakai flag deal (DealsListFilterFor), bukan activity.
SELECT
    deal_owner        AS owner_id,
    COUNT(*)::bigint  AS won_count
FROM deals
WHERE deleted_at IS NULL
  AND stage = 'Closed Won'
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
GROUP BY deal_owner;
