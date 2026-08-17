-- dashboard.sql — agregasi Beranda ruang kerja (Modul 1, tasks.md M1-1). Empat
-- kartu: ARR total, pipeline per-stage, distribusi health, renewal jatuh tempo.
-- Distribusi health TIDAK dituliskan ulang di sini — reuse CountHealthScoreKPIs
-- (health_score.sql), sudah tepat bentuknya. Ownership (F3) pakai flag SAMA
-- dengan modul asal tiap tabel: deals → DealsListFilter (deal_owner), subscriptions
-- → SubscriptionsListFilter (subscription_owner) — sumber SATU, bukan duplikat
-- logic scope. COALESCE(...)::bigint/::numeric membungkus tiap agregat agar sqlc
-- tak meng-emit interface{} (gotcha #14).

-- name: DashboardARRTotal :one
-- ARR total dari langganan AKTIF dalam cakupan ownership. ARR disimpan (bukan
-- MRR×12, keputusan skema §5); F4 (masking non-manager) urusan handler/view,
-- bukan query — nilai mentah selalu dihitung, disembunyikan di lapis tampilan.
SELECT COALESCE(SUM(arr) FILTER (WHERE status = 'Active'), 0)::numeric AS arr_total
FROM subscriptions
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND subscription_owner = sqlc.arg(uid))
  );

-- name: DashboardPipelineByStage :many
-- Pipeline per-stage (bukan agregat open/won/lost seperti DealPipelineStats) —
-- satu baris per stage TERBUKA (exclude Closed Won/Lost, sama seperti kanban),
-- diurutkan CASE mengikuti urutan alami dealStageOptions (sales_deals_form.go)
-- agar chart bar x-axis-nya berurutan tanpa re-sort di Go.
SELECT
    stage,
    COUNT(*)::bigint          AS deal_count,
    COALESCE(SUM(amount), 0)::numeric AS stage_value
FROM deals
WHERE deleted_at IS NULL
  AND stage NOT IN ('Closed Won', 'Closed Lost')
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
    ELSE 6
END;

-- name: DashboardRenewalsDue :one
-- Langganan jatuh tempo 30 hari ke depan (jendela SAMA dgn ListRenewals window
-- "due": status Active/PendingApproval, end_date antara today..today+30) + ARR
-- yang mengambang di dalamnya (masking F4 di handler, bukan di sini).
SELECT
    COUNT(*)::bigint                    AS due_count,
    COALESCE(SUM(arr), 0)::numeric       AS due_arr
FROM subscriptions
WHERE deleted_at IS NULL
  AND status IN ('Active', 'PendingApproval')
  AND end_date >= sqlc.arg(today)::date
  AND end_date <= sqlc.arg(today)::date + 30
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND subscription_owner = sqlc.arg(uid))
  );
