-- customer_success.sql — snapshot Health/Journey/Onboarding/Adoption SATU
-- baris per desa (Modul 6 slice B1). Isolasi WORKSPACE ditegakkan RLS (GUC
-- app.tenant_id di WithTenant); tak ada filter tenant_id manual. TANPA
-- soft-delete (ikut hidup account, ON DELETE CASCADE).
--
-- TANPA upsert ON CONFLICT — tak ada precedent di codebase; handler baca baris
-- existing dulu (GetCustomerSuccessByAccountID) untuk masking F2 per-section
-- sebelum menulis, jadi get-then-branch (Create kalau absen, Update kalau
-- ada) justru pola yang sudah dibutuhkan, bukan beban tambahan.

-- name: GetCustomerSuccessByAccountID :one
-- Satu baris per desa. pgx.ErrNoRows → belum pernah disimpan (jalur Create).
SELECT * FROM customer_success
WHERE account_id = sqlc.arg(account_id);

-- name: CreateCustomerSuccess :one
-- Buat baris pertama kali desa ini disimpan. tenant_id eksplisit (RLS WITH
-- CHECK memverifikasinya = GUC). usage_data_source TAK dioper (default
-- 'Manual', v1 tak ada field form untuknya).
INSERT INTO customer_success (
    tenant_id, account_id,
    overall_health_score, health_status, adoption_score, engagement_score,
    support_score, sentiment_score, score_trend, health_last_calculated,
    lifecycle_stage, stage_entry_date,
    onboarding_status, kickoff_date, target_go_live_date, actual_go_live_date,
    onboarding_progress,
    last_login_date, active_users, login_frequency, feature_adoption_rate,
    key_features_used, usage_trend,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(account_id),
    sqlc.narg(overall_health_score), sqlc.narg(health_status), sqlc.narg(adoption_score), sqlc.narg(engagement_score),
    sqlc.narg(support_score), sqlc.narg(sentiment_score), sqlc.narg(score_trend), sqlc.narg(health_last_calculated),
    sqlc.narg(lifecycle_stage), sqlc.narg(stage_entry_date),
    sqlc.narg(onboarding_status), sqlc.narg(kickoff_date), sqlc.narg(target_go_live_date), sqlc.narg(actual_go_live_date),
    sqlc.narg(onboarding_progress),
    sqlc.narg(last_login_date), sqlc.narg(active_users), sqlc.narg(login_frequency), sqlc.narg(feature_adoption_rate),
    sqlc.narg(key_features_used), sqlc.narg(usage_trend),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: UpdateCustomerSuccess :one
-- Sunting seluruh snapshot, key by account_id (semua lookup mulai dari {id}
-- account di URL — bukan surrogate id tabel ini). Section yang aktor tak
-- berhak tulis WAJIB diisi handler dari baris existing sebelum dioper ke sini
-- (masking F2 per-section, lihat customer_success.go).
UPDATE customer_success SET
    overall_health_score   = sqlc.narg(overall_health_score),
    health_status           = sqlc.narg(health_status),
    adoption_score          = sqlc.narg(adoption_score),
    engagement_score        = sqlc.narg(engagement_score),
    support_score           = sqlc.narg(support_score),
    sentiment_score          = sqlc.narg(sentiment_score),
    score_trend             = sqlc.narg(score_trend),
    health_last_calculated  = sqlc.narg(health_last_calculated),
    lifecycle_stage         = sqlc.narg(lifecycle_stage),
    stage_entry_date        = sqlc.narg(stage_entry_date),
    onboarding_status       = sqlc.narg(onboarding_status),
    kickoff_date            = sqlc.narg(kickoff_date),
    target_go_live_date     = sqlc.narg(target_go_live_date),
    actual_go_live_date     = sqlc.narg(actual_go_live_date),
    onboarding_progress     = sqlc.narg(onboarding_progress),
    last_login_date         = sqlc.narg(last_login_date),
    active_users            = sqlc.narg(active_users),
    login_frequency         = sqlc.narg(login_frequency),
    feature_adoption_rate   = sqlc.narg(feature_adoption_rate),
    key_features_used       = sqlc.narg(key_features_used),
    usage_trend             = sqlc.narg(usage_trend),
    updated_by              = sqlc.narg(updated_by),
    updated_at              = now()
WHERE account_id = sqlc.arg(account_id)
RETURNING *;
