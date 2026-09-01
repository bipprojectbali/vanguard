-- sla_policies.sql — katalog master (SLA Policies), Modul 6 Customer Success
-- slice A1. Isolasi WORKSPACE ditegakkan RLS (GUC app.tenant_id di WithTenant);
-- tak ada filter tenant_id manual. sla_policies TANPA soft-delete: is_active=
-- false = pensiun (kebijakan lama tak dipakai lagi, tapi riwayat tiket lama
-- yang sudah memakainya tetap sah — D1/D2 menyalin target saat tiket dibuat,
-- bukan referensi hidup ke sini). Target respon/selesai satuan MENIT (migrasi
-- 00016). Meniru pola plans.sql.

-- name: ListSLAPolicies :many
-- Kebijakan aktif untuk picker (dipakai D1/D2 saat menentukan target tiket
-- baru per prioritas). Hanya is_active=true. Bounded katalog master
-- per-workspace → tanpa keyset.
SELECT * FROM sla_policies
WHERE is_active = true
ORDER BY sla_name ASC, id ASC;

-- name: GetSLAPolicy :one
-- Satu kebijakan (untuk baca target saat dipakai D1/D2). RLS menjamin
-- tenant_id. Tak filter is_active: handler memutuskan.
SELECT * FROM sla_policies
WHERE id = sqlc.arg(id);

-- name: ListSLAPoliciesAll :many
-- Seluruh katalog untuk tampilan kelola (TERMASUK yang pensiun) — beda dari
-- ListSLAPolicies (hanya aktif). Keyset (created_at DESC, id DESC) + LIMIT
-- (BL-6): katalog master pun bisa tumbuh, jadi halaman dibatasi & tetap
-- konsisten walau ada sisipan. Status aktif/pensiun tampak dari badge per
-- baris, bukan lagi dari urutan (grouping is_active dilepas demi kursor keyset
-- satu-kolom yang dipakai seluruh app).
SELECT * FROM sla_policies
WHERE (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: CreateSLAPolicy :one
-- Buat kebijakan SLA. tenant_id eksplisit (RLS WITH CHECK memverifikasinya =
-- GUC). is_active default true di skema tapi di-set eksplisit agar handler
-- bisa membuat draft pensiun bila diperlukan nanti.
INSERT INTO sla_policies (
    tenant_id, sla_name, applies_to_priority,
    first_response_target_minutes, resolution_target_minutes,
    business_hours, escalation_rule, is_active, created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(sla_name), sqlc.narg(applies_to_priority),
    sqlc.narg(first_response_target_minutes), sqlc.narg(resolution_target_minutes),
    sqlc.narg(business_hours), sqlc.narg(escalation_rule), sqlc.arg(is_active),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: UpdateSLAPolicy :one
-- Sunting profil kebijakan. is_active TAK di sini (SetSLAPolicyActive) —
-- pensiun/aktifkan adalah aksi tersendiri, bukan efek samping edit.
UPDATE sla_policies SET
    sla_name                       = sqlc.arg(sla_name),
    applies_to_priority            = sqlc.narg(applies_to_priority),
    first_response_target_minutes  = sqlc.narg(first_response_target_minutes),
    resolution_target_minutes      = sqlc.narg(resolution_target_minutes),
    business_hours                 = sqlc.narg(business_hours),
    escalation_rule                = sqlc.narg(escalation_rule),
    updated_by                     = sqlc.narg(updated_by),
    updated_at                     = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetSLAPolicyActive :exec
-- Pensiunkan (false) atau aktifkan kembali (true) kebijakan. Kebijakan
-- pensiun hilang dari ListSLAPolicies (picker) tapi tetap tampil di kelola.
UPDATE sla_policies SET
    is_active  = sqlc.arg(is_active),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id);
