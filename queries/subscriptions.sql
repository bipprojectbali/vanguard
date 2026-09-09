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
    status, approval_status, start_date, end_date, billing_cycle, auto_renew, contract_term_months,
    mrr, arr, quantity_seats, discount_pct, payment_status,
    renewal_status, renewal_type, renewal_owner, renewal_quote_id, previous_value,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.narg(entity_code), sqlc.narg(subscription_owner),
    sqlc.arg(account_id), sqlc.narg(plan_id),
    sqlc.narg(source_deal_id), sqlc.narg(previous_subscription_id),
    sqlc.arg(status), sqlc.narg(approval_status), sqlc.narg(start_date), sqlc.narg(end_date),
    sqlc.narg(billing_cycle), sqlc.arg(auto_renew), sqlc.narg(contract_term_months),
    sqlc.narg(mrr), sqlc.narg(arr), sqlc.narg(quantity_seats), sqlc.narg(discount_pct),
    sqlc.narg(payment_status),
    sqlc.narg(renewal_status), sqlc.narg(renewal_type), sqlc.narg(renewal_owner),
    sqlc.narg(renewal_quote_id), sqlc.narg(previous_value),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: HasActiveItemForQuotePlans :one
-- Pre-check Won (BL-88 PR2b): benar bila SALAH SATU paket di quote yang di-Accept sudah
-- punya item Active untuk desa ini. Menggantikan HasActiveSubscriptionForPlan berbasis
-- deals.plan_requested_id (single-plan) — kini invarian ada di subscription_items
-- (parent_active = status Active & belum terhapus). Pesan ramah SEBELUM buat langganan;
-- idx_subscription_items_one_active tetap penjaga keras bila balapan. RLS jamin tenant.
SELECT EXISTS (
    SELECT 1
    FROM subscription_items si
    JOIN quote_items qi ON qi.plan_id = si.plan_id
    WHERE si.account_id = sqlc.arg(account_id)
      AND si.parent_active
      AND qi.quote_id = sqlc.arg(quote_id)
      AND qi.plan_id IS NOT NULL
);

-- name: HasActiveItemConflictForSubscription :one
-- Pre-check aktivasi Trial→Active (BL-88 PR2b): benar bila SALAH SATU paket langganan
-- yang hendak diaktifkan sudah punya item Active di langganan LAIN untuk desa yang sama.
-- Langganan yg diaktifkan masih Trial (parent_active=false pada item-nya) → sisi `other`
-- (parent_active) mengecualikannya dgn sendirinya. RLS jamin tenant.
SELECT EXISTS (
    SELECT 1
    FROM subscription_items si
    JOIN subscription_items other
      ON other.account_id = si.account_id
     AND other.plan_id    = si.plan_id
     AND other.subscription_id <> si.subscription_id
     AND other.parent_active
    WHERE si.subscription_id = sqlc.arg(subscription_id)
      AND si.plan_id IS NOT NULL
);

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
-- JOIN accounts membawa nama desa (INNER — account_id NOT NULL). plans di-LEFT JOIN:
-- BL-88 PR2b plan_id NULLABLE (langganan multi-paket → identitas di subscription_items,
-- plan_name parent NULL). accounts di-filter baris hidup. Kolom "Paket" dirakit handler
-- ("N paket" bila >1 item) — plan_name di sini hanya label cepat single-plan.
--
-- search '' → tak menyaring; selain itu MEMPERSEMPIT (ILIKE substring, case-
-- insensitive) di ATAS ownership+status — tak pernah melebarkan baris. Hanya
-- kolom tak-tersamar yang TAMPIL di tabel jadi kunci cari (desa, paket, kode
-- entitas); nilai MRR/ARR tersamar TIDAK dijadikan kunci cari (BL-6).
SELECT s.*, a.village_name, p.plan_name,
    (SELECT COUNT(*) FROM subscription_items si WHERE si.subscription_id = s.id)::bigint AS item_count
FROM subscriptions s
JOIN accounts a ON a.id = s.account_id
LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.deleted_at IS NULL
  AND a.deleted_at IS NULL
  AND (s.created_at, s.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
  AND (sqlc.arg(status_filter)::text = '' OR s.status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR a.village_name ILIKE '%' || sqlc.arg(search) || '%'
      OR p.plan_name ILIKE '%' || sqlc.arg(search) || '%'
      OR s.entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListRenewals :many
-- Dasbor Renewals (Menu 5.2, READ-ONLY — aksi perpanjangan ada di detail langganan,
-- bukan di sini). Langganan yang punya dimensi renewal (end_date terisi), di-scope
-- ownership (F3) dengan flag yang SAMA dgn ListSubscriptions. Empat JENDELA lewat
-- window_filter (today dioper handler agar mengikuti zona waktu app & bisa
-- dideterministikkan test):
--   • 'due'     : Active/PendingApproval, end_date ∈ [today, today+30] — jatuh tempo.
--   • 'grace'   : Active, end_date < today — lewat tempo tapi masih berjalan.
--   • 'renewed' : renewal_status = 'Renewed' — sudah diperpanjang.
--   • lainnya   : semua langganan ber-end_date (jendela 'Semua').
-- Urut created_at DESC + keyset SAMA dgn ListSubscriptions (reuse pageCursor/
-- splitPage); pengurutan "paling dekat jatuh tempo" ditunda ke slice KPI/agregasi.
-- previous_value dibawa di s.* untuk kolom "Prev→Current" (tanpa JOIN tambahan).
SELECT s.*, a.village_name, p.plan_name,
    (SELECT COUNT(*) FROM subscription_items si WHERE si.subscription_id = s.id)::bigint AS item_count
FROM subscriptions s
JOIN accounts a ON a.id = s.account_id
LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.deleted_at IS NULL
  AND a.deleted_at IS NULL
  AND s.end_date IS NOT NULL
  AND (s.created_at, s.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
  AND (
      CASE sqlc.arg(window_filter)::text
        WHEN 'due'     THEN s.status IN ('Active','PendingApproval')
                            AND s.end_date >= sqlc.arg(today)::date
                            AND s.end_date <= (sqlc.arg(today)::date + 30)
        WHEN 'grace'   THEN s.status = 'Active' AND s.end_date < sqlc.arg(today)::date
        WHEN 'renewed' THEN s.renewal_status = 'Renewed'
        ELSE TRUE
      END
  )
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_size);

-- name: RenewalKPIs :one
-- KPI dasbor Renewals (BL-94) dalam SATU round-trip, di-scope ownership (flag
-- SAMA dgn ListRenewals/ListSubscriptions: scope_all → semua; is_own →
-- subscription_owner = uid; keduanya false → NOL, fail-closed). Predikat cacah
-- MENGIKUTI jendela ListRenewals agar KPI konsisten dgn tab:
--   * due_30      : Active/PendingApproval, end_date in [today, today+30] (= window 'due').
--   * grace       : Active, end_date < today (= window 'grace').
--   * renewed     : renewal_status = 'Renewed' (= window 'renewed', sudah diperpanjang).
--   * Renewal Rate 12 bln (BL-94, definisi SAMA dgn ReportRenewalSummary): renewed_past
--     / due_past atas kohort jatuh tempo (end_date < today) DALAM 12 bln terakhir;
--     "diperpanjang" = ada baris renewal anak (previous_subscription_id menunjuk balik).
-- today dioper handler (zona waktu app, deterministik utk test). Nilai Rp tak di sini
-- (KPI ini murni cacah + rasio); scope RLS menegakkan tenant (tanpa filter manual).
SELECT
    COUNT(*) FILTER (
        WHERE s.status IN ('Active','PendingApproval')
          AND s.end_date >= sqlc.arg(today)::date
          AND s.end_date <= (sqlc.arg(today)::date + 30)
    )::bigint AS due_30,
    COUNT(*) FILTER (
        WHERE s.status = 'Active' AND s.end_date < sqlc.arg(today)::date
    )::bigint AS grace,
    COUNT(*) FILTER (
        WHERE s.renewal_status = 'Renewed'
    )::bigint AS renewed_count,
    COUNT(*) FILTER (
        WHERE s.end_date < sqlc.arg(today)::date
          AND s.end_date >= (sqlc.arg(today)::date - INTERVAL '12 months')::date
    )::bigint AS due_past_12m,
    COUNT(*) FILTER (
        WHERE s.end_date < sqlc.arg(today)::date
          AND s.end_date >= (sqlc.arg(today)::date - INTERVAL '12 months')::date
          AND EXISTS (
              SELECT 1 FROM subscriptions r
              WHERE r.previous_subscription_id = s.id AND r.deleted_at IS NULL
          )
    )::bigint AS renewed_past_12m
FROM subscriptions s
WHERE s.deleted_at IS NULL
  AND s.end_date IS NOT NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  );

-- name: SubscriptionListKPIs :one
-- KPI header halaman Subscription Lists (BL-95) dalam SATU round-trip, di-scope
-- ownership SAMA dgn ListSubscriptions (scope_all → semua; is_own →
-- subscription_owner = uid; keduanya false → NOL, fail-closed). RLS mengurung tenant.
--   • total_mrr    : SUM(mrr) langganan Active (nilai berulang berjalan).
--   • total_arr    : total_mrr × 12 (proyeksi 12 bln, keputusan user — bukan kolom
--                    arr yg bisa diskon tahunan; label kartu "proyeksi 12 bln" persis).
--   • new_mrr      : SUM(mrr) langganan BARU (tanpa previous_subscription_id — logo
--                    baru, bukan perpanjangan) yang mulai BULAN berjalan. Definisi
--                    "MRR baru bln ini" (keputusan user) selaras ReportSubMRR.new_mrr
--                    (renewal punya previous_subscription_id → sengaja tak dihitung
--                    agar delta = pertumbuhan bersih, bukan sekadar start_date baru).
--   • active_count : COUNT langganan Active (kartu "Active Subs").
--   • churned_30   : COUNT Cancelled/Churned dgn cancellation_date dalam 30 hari
--                    terakhir → pembilang Churn Rate (definisi churn% app: churned /
--                    (active + churned), jendela 30 hari untuk churned).
--   • total_accounts : COUNT semua akun (desa) hidup di tenant — denominator "dari
--                    N desa" (keputusan user: total akun, bukan hanya yg berlangganan).
--                    Subquery SENGAJA di luar filter ownership: denominator = basis
--                    desa penuh, bukan yg dimiliki pemanggil.
-- today dioper handler (zona waktu app, deterministik utk test).
SELECT
    COALESCE(SUM(s.mrr) FILTER (WHERE s.status = 'Active'), 0)::numeric AS total_mrr,
    (COALESCE(SUM(s.mrr) FILTER (WHERE s.status = 'Active'), 0) * 12)::numeric AS total_arr,
    COALESCE(SUM(s.mrr) FILTER (
        WHERE s.status = 'Active'
          AND s.previous_subscription_id IS NULL
          AND s.start_date >= date_trunc('month', sqlc.arg(today)::date)::date
          AND s.start_date <  (date_trunc('month', sqlc.arg(today)::date) + INTERVAL '1 month')::date
    ), 0)::numeric AS new_mrr,
    COUNT(*) FILTER (WHERE s.status = 'Active')::bigint AS active_count,
    COUNT(*) FILTER (
        WHERE s.status IN ('Cancelled', 'Churned')
          AND s.cancellation_date >= (sqlc.arg(today)::date - 30)
          AND s.cancellation_date <= sqlc.arg(today)::date
    )::bigint AS churned_30,
    (SELECT COUNT(*) FROM accounts a WHERE a.deleted_at IS NULL)::bigint AS total_accounts
FROM subscriptions s
WHERE s.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  );

-- name: ListChurned :many
-- Dasbor Churn (Menu 5.2/5.4, READ-ONLY). Langganan yang telah berhenti
-- (status Cancelled/Churned), di-scope ownership (F3) dengan flag yang SAMA dgn
-- ListSubscriptions (scope_all → semua; is_own → subscription_owner = uid; keduanya
-- false → NOL baris, fail-closed). Filter tipe churn opsional lewat type_filter
-- (Voluntary/Involuntary); '' → semua tipe. Kolom churn (lost_value_mrr, churn_reason,
-- cancellation_date) dibawa di s.* → tanpa JOIN tambahan. Urut created_at DESC +
-- keyset SAMA dgn ListSubscriptions (reuse pageCursor/splitPage).
SELECT s.*, a.village_name, p.plan_name,
    (SELECT COUNT(*) FROM subscription_items si WHERE si.subscription_id = s.id)::bigint AS item_count
FROM subscriptions s
JOIN accounts a ON a.id = s.account_id
LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.deleted_at IS NULL
  AND a.deleted_at IS NULL
  AND s.status IN ('Cancelled', 'Churned')
  AND (s.created_at, s.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND s.subscription_owner = sqlc.arg(uid))
  )
  AND (sqlc.arg(type_filter)::text = '' OR s.churn_type = sqlc.arg(type_filter)::text)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListSubscriptionsForAccount :many
-- Daftar langganan satu desa (detail account → langganannya), keyset. Account sudah
-- ter-scope ownership di handler; di sini cukup filter account_id + baris hidup.
-- plan_name dibawa untuk kolom "Paket".
SELECT s.*, p.plan_name,
    (SELECT COUNT(*) FROM subscription_items si WHERE si.subscription_id = s.id)::bigint AS item_count
FROM subscriptions s
LEFT JOIN plans p ON p.id = s.plan_id
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
    plan_id              = sqlc.narg(plan_id),
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

-- name: ApproveRenewal :one
-- Setujui renewal Upsell yang menunggu (M5-3c): baris 'PendingApproval' → 'Active'.
-- Handler WAJIB meng-Expired baris lama (previous_subscription_id) SEBELUM query ini
-- dalam tx yang sama — invarian idx_subs_one_active (1 Active per account+plan).
-- Filter status='PendingApproval' = penjaga transisi: baris yang sudah diputus tak
-- bisa disetujui dua kali (0 baris ter-update → handler kabari "tak lagi pending").
UPDATE subscriptions SET
    status          = 'Active',
    approval_status = 'Approved',
    approved_by     = sqlc.narg(approved_by),
    approved_at     = now(),
    updated_by      = sqlc.narg(updated_by),
    updated_at      = now()
WHERE id = sqlc.arg(id) AND status = 'PendingApproval' AND deleted_at IS NULL
RETURNING *;

-- name: RejectRenewal :one
-- Tolak renewal Upsell yang menunggu (M5-3c): baris 'PendingApproval' → 'Cancelled'.
-- Baris lama TAK disentuh — ia tetap 'Active' (renewal batal, langganan berjalan).
-- Filter status='PendingApproval' = penjaga transisi (idem ApproveRenewal).
UPDATE subscriptions SET
    status          = 'Cancelled',
    approval_status = 'Rejected',
    approved_by     = sqlc.narg(approved_by),
    approved_at     = now(),
    updated_by      = sqlc.narg(updated_by),
    updated_at      = now()
WHERE id = sqlc.arg(id) AND status = 'PendingApproval' AND deleted_at IS NULL
RETURNING *;

-- name: ListCSRenewals :many
-- Daftar Renewal Management CS (Menu 6.6). Menampilkan langganan yang punya
-- dimensi renewal (end_date terisi), berikut field AKSI CS (renewal_stage,
-- renewal_risk, renewal_action_plan, renewal_next_action_date, renewal_owner).
-- Data sumber tetap di subscriptions (keputusan "Renewal Dua-Rumah").
--
-- F3 ownership via akun (assigned_csm/backup_csm/account_owner = uid) — identik
-- dengan TicketsListFilter/EngagementsListFilter, BUKAN subscription_owner,
-- karena CS melihat semua desa binaan terlepas siapa sales-owner langganannya.
--
-- filter_stage '' → semua stage; non-'' → cocokkan persis.
-- Keyset (created_at DESC, id DESC) — reuse pageCursor/splitPage standar.
SELECT
    s.id,
    s.account_id,
    s.status,
    s.end_date,
    s.renewal_status,
    s.renewal_stage,
    s.renewal_risk,
    s.renewal_action_plan,
    s.renewal_next_action_date,
    s.renewal_owner,
    s.created_at,
    a.village_name,
    p.plan_name,
    u.name AS renewal_owner_name,
    (SELECT COUNT(*) FROM subscription_items si WHERE si.subscription_id = s.id)::bigint AS item_count
FROM subscriptions s
JOIN accounts a ON a.id = s.account_id AND a.deleted_at IS NULL
LEFT JOIN plans p ON p.id = s.plan_id
LEFT JOIN users u ON u.id = s.renewal_owner
WHERE s.deleted_at IS NULL
  AND s.end_date IS NOT NULL
  AND (s.created_at, s.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm  = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_stage)::text = '' OR s.renewal_stage = sqlc.arg(filter_stage)::text)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateCSRenewalAction :one
-- Perbarui field AKSI CS renewal (6.6 "Renewal Dua-Rumah"). Hanya empat field
-- milik CS yang disentuh; field inti langganan (status, MRR, dsb.) tidak berubah.
-- Handler menegakkan F3 (loadCSRenewal) sebelum memanggil query ini.
UPDATE subscriptions SET
    renewal_stage            = sqlc.narg(renewal_stage),
    renewal_risk             = sqlc.narg(renewal_risk),
    renewal_action_plan      = sqlc.narg(renewal_action_plan),
    renewal_next_action_date = sqlc.narg(renewal_next_action_date),
    renewal_owner            = sqlc.narg(renewal_owner),
    updated_by               = sqlc.narg(updated_by),
    updated_at               = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, renewal_stage, renewal_risk, renewal_action_plan,
          renewal_next_action_date, renewal_owner, updated_at;

-- name: SoftDeleteSubscription :exec
-- Soft-delete: baris disembunyikan dari list/get tapi tetap ada (rantai renewal &
-- FK dari deals.created_subscription_id tak putus). Idempotent: hanya baris hidup.
UPDATE subscriptions SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetLatestSubscriptionForAccount :one
-- Langganan TERBARU satu desa (kartu "Ringkasan Langganan" di detail Account) —
-- satu baris tanpa keyset, beda kebutuhan dari ListSubscriptionsForAccount (daftar
-- berhalaman). plan_name ikut lewat JOIN plans yang sama. pgx.ErrNoRows = desa
-- belum pernah berlangganan (empty-state di handler, bukan error).
SELECT s.*, p.plan_name,
    (SELECT COUNT(*) FROM subscription_items si WHERE si.subscription_id = s.id)::bigint AS item_count
FROM subscriptions s
LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.account_id = sqlc.arg(account_id) AND s.deleted_at IS NULL
ORDER BY s.created_at DESC, s.id DESC
LIMIT 1;


-- ── subscription_items — baris langganan (BL-88 PR2a; hard-delete, mirror quote_items) ──

-- name: AddSubscriptionItem :one
-- Tambah baris item langganan. tenant_id eksplisit (RLS WITH CHECK). unit_price &
-- subtotal = SNAPSHOT komersial disalin dari quote_items saat Closed Won; mrr/arr
-- per-item diturunkan app (subtotal/bulan-termin). Beku sesudahnya.
INSERT INTO subscription_items (
    subscription_id, tenant_id, plan_id, quantity, unit_price, discount_pct,
    subtotal, mrr, arr, line_no
) VALUES (
    sqlc.arg(subscription_id), sqlc.arg(tenant_id), sqlc.narg(plan_id), sqlc.arg(quantity),
    sqlc.arg(unit_price), sqlc.narg(discount_pct), sqlc.arg(subtotal),
    sqlc.narg(mrr), sqlc.narg(arr), sqlc.narg(line_no)
)
RETURNING *;

-- name: ListSubscriptionItems :many
-- Baris item satu langganan, urut tampil (line_no lalu id). Menopang tabel item di
-- detail langganan & agregasi per-produk. Bounded per-langganan → tanpa keyset.
SELECT * FROM subscription_items
WHERE subscription_id = sqlc.arg(subscription_id)
ORDER BY line_no ASC NULLS LAST, id ASC;

-- name: ListSubscriptionItemsWithPlan :many
-- Sama seperti ListSubscriptionItems + nama paket (LEFT JOIN plans agar baris item
-- ber-plan_id NULL / plan terhapus tetap muncul, nama → NULL). Menopang tabel item di
-- detail langganan (BL-88 PR2b). Bounded per-langganan → tanpa keyset.
SELECT si.id, si.subscription_id, si.tenant_id, si.plan_id, si.quantity,
       si.unit_price, si.discount_pct, si.subtotal, si.mrr, si.arr, si.line_no,
       si.account_id, si.parent_active,
       p.plan_name AS plan_name
FROM subscription_items si
LEFT JOIN plans p ON p.id = si.plan_id
WHERE si.subscription_id = sqlc.arg(subscription_id)
ORDER BY si.line_no ASC NULLS LAST, si.id ASC;
