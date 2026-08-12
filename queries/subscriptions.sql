-- subscriptions.sql — langganan (Subscription), Modul 5 slice 2. Isolasi WORKSPACE
-- ditegakkan RLS (GUC app.tenant_id di WithTenant); isolasi ANTAR-DESA (F3)
-- ditegakkan di layer query lewat flag ownership di ListSubscriptions (kolom
-- subscription_owner) — pola sama dgn deals (lihat ownership.go).
--
-- KEPUTUSAN inti (docs/crm/skema.md §5, migrasi 00012):
--   • Renewal = INSERT baris BARU (CreateSubscription dgn previous_subscription_id
--     + previous_value), BUKAN update. Handler WAJIB meng-Expired baris lama
--     (UpdateSubscriptionStatus 'Expired') SEBELUM CreateSubscription baris aktif,
--     dalam SATU tx — kalau tidak, idx_subs_one_active (1 Active per account+plan)
--     menolak INSERT kedua. Urutan itu = invarian, ditegakkan test M5-5.
--   • Create-from-deal = CreateSubscription (source_deal_id) lalu
--     SetDealCreatedSubscription (deals.sql) dalam tx yang sama → tautan dua-arah.
--   • Churn = ChurnSubscription (status Cancelled/Churned + kolom churn 5.4 sekali
--     tulis; churn adalah SATU aksi bisnis, bukan edit profil).
--   • ARR DISIMPAN (bukan MRR×12); field-level masking ARR utk non-manager = urusan
--     handler/view (F4), bukan query.
-- entity_code (SUB-0001) dialokasikan GenerateEntityCode DALAM tx create.

-- name: CreateSubscription :one
-- Buat langganan. tenant_id eksplisit (RLS WITH CHECK memverifikasinya = GUC).
-- Dipakai DUA jalur: (a) create-from-deal (source_deal_id terisi, previous_* NULL);
-- (b) renewal (previous_subscription_id + previous_value terisi, source_deal_id
-- opsional). status default 'Trial' di skema tapi di-set eksplisit (renewal lahir
-- 'Active'). Kolom renewal-action (6.6) & churn (5.4) TIDAK di-set di sini — punya
-- jalur sendiri.
INSERT INTO subscriptions (
    tenant_id, entity_code, subscription_owner, account_id, plan_id,
    source_deal_id, previous_subscription_id,
    status, start_date, end_date, billing_cycle, auto_renew, contract_term_months,
    mrr, arr, quantity_seats, discount_pct, payment_status,
    renewal_status, renewal_type, renewal_owner, renewal_quote_id, previous_value,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.narg(entity_code), sqlc.narg(subscription_owner),
    sqlc.arg(account_id), sqlc.arg(plan_id),
    sqlc.narg(source_deal_id), sqlc.narg(previous_subscription_id),
    sqlc.arg(status), sqlc.narg(start_date), sqlc.narg(end_date),
    sqlc.narg(billing_cycle), sqlc.arg(auto_renew), sqlc.narg(contract_term_months),
    sqlc.narg(mrr), sqlc.narg(arr), sqlc.narg(quantity_seats), sqlc.narg(discount_pct),
    sqlc.narg(payment_status),
    sqlc.narg(renewal_status), sqlc.narg(renewal_type), sqlc.narg(renewal_owner),
    sqlc.narg(renewal_quote_id), sqlc.narg(previous_value),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetSubscription :one
-- Satu langganan hidup. RLS menjamin tenant_id; ownership (F3) diputuskan handler
-- atas baris (SubscriptionsListFilter.Allows), bukan di sini.
SELECT * FROM subscriptions
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListSubscriptions :many
-- Daftar langganan (menu /subscriptions), keyset (created_at DESC, id DESC) + filter
-- ownership F3 + filter status opsional. Dua flag ownership (sumber SATU dgn
-- SubscriptionsListFilter): scope_all → semua; is_own → subscription_owner = uid;
-- keduanya false → NOL baris (fail-closed). status_filter '' → semua status.
-- JOIN accounts+plans membawa nama untuk kolom (hindari N+1, rule 13); INNER JOIN
-- aman karena account_id/plan_id NOT NULL. accounts di-filter baris hidup.
SELECT s.*, a.village_name, p.plan_name
FROM subscriptions s
JOIN accounts a ON a.id = s.account_id
JOIN plans    p ON p.id = s.plan_id
WHERE s.deleted_at IS NULL
  AND a.deleted_at IS NULL
  AND (s.created_at, s.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
  AND (sqlc.arg(status_filter)::text = '' OR s.status = sqlc.arg(status_filter)::text)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListSubscriptionsForAccount :many
-- Daftar langganan satu desa (detail account → langganannya), keyset. Account sudah
-- ter-scope ownership di handler; di sini cukup filter account_id + baris hidup.
-- plan_name dibawa untuk kolom "Paket".
SELECT s.*, p.plan_name
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.deleted_at IS NULL
  AND s.account_id = sqlc.arg(account_id)
  AND (s.created_at, s.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListRenewalChain :many
-- Riwayat renewal (M5-4): telusuri rantai MUNDUR dari satu langganan lewat
-- previous_subscription_id (self-FK) sampai periode paling awal, lalu urut kronologis
-- (lama→baru). Rekursif via self-FK (bukan filter account+plan) karena renewal
-- Upsell/Downgrade bisa berganti plan — hanya self-FK yang otoritatif sbg tautan
-- rantai. Termasuk baris ter-soft-delete (riwayat tak boleh berlubang).
WITH RECURSIVE chain AS (
    SELECT s.* FROM subscriptions s WHERE s.id = sqlc.arg(id)
    UNION ALL
    SELECT s.* FROM subscriptions s
    JOIN chain c ON s.id = c.previous_subscription_id
)
SELECT * FROM chain
ORDER BY created_at ASC, id ASC;

-- name: UpdateSubscription :one
-- Sunting profil langganan. status, renewal-action (6.6), dan churn (5.4) punya
-- jalur sendiri (UpdateSubscriptionStatus/ChurnSubscription) agar transisi status &
-- churn terlihat sebagai aksi tersendiri, bukan efek samping edit.
UPDATE subscriptions SET
    subscription_owner   = sqlc.narg(subscription_owner),
    plan_id              = sqlc.arg(plan_id),
    start_date           = sqlc.narg(start_date),
    end_date             = sqlc.narg(end_date),
    billing_cycle        = sqlc.narg(billing_cycle),
    auto_renew           = sqlc.arg(auto_renew),
    contract_term_months = sqlc.narg(contract_term_months),
    mrr                  = sqlc.narg(mrr),
    arr                  = sqlc.narg(arr),
    quantity_seats       = sqlc.narg(quantity_seats),
    discount_pct         = sqlc.narg(discount_pct),
    payment_status       = sqlc.narg(payment_status),
    renewal_status       = sqlc.narg(renewal_status),
    renewal_type         = sqlc.narg(renewal_type),
    renewal_owner        = sqlc.narg(renewal_owner),
    renewal_quote_id     = sqlc.narg(renewal_quote_id),
    updated_by           = sqlc.narg(updated_by),
    updated_at           = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: UpdateSubscriptionStatus :exec
-- Transisi status (Trial→Active, Active→Expired saat renewal, dst). Aturan transisi
-- valid divalidasi handler (bukan constraint DB) agar pesan bisa diperbaiki user;
-- subs_status_chk hanya membatasi himpunan nilai legal. INVARIAN renewal: Expired
-- baris lama HARUS mendahului CreateSubscription baris aktif dalam tx yang sama.
UPDATE subscriptions SET
    status     = sqlc.arg(status),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ChurnSubscription :exec
-- Churn (5.4): satu aksi bisnis — set status (Cancelled/Churned) + seluruh kolom
-- churn sekaligus. Domain churn_reason/churn_type dijaga subs_churn_*_chk.
UPDATE subscriptions SET
    status            = sqlc.arg(status),
    cancellation_date = sqlc.narg(cancellation_date),
    churn_reason      = sqlc.narg(churn_reason),
    churn_type        = sqlc.narg(churn_type),
    churn_notes       = sqlc.narg(churn_notes),
    lost_value_mrr    = sqlc.narg(lost_value_mrr),
    win_back_eligible = sqlc.narg(win_back_eligible),
    updated_by        = sqlc.narg(updated_by),
    updated_at        = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SoftDeleteSubscription :exec
-- Soft-delete: baris disembunyikan dari list/get tapi tetap ada (rantai renewal &
-- FK dari deals.created_subscription_id tak putus). Idempotent: hanya baris hidup.
UPDATE subscriptions SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
