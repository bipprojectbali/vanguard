-- playbooks.sql — katalog master (Playbooks), Modul 6 Customer Success
-- slice A2. Isolasi WORKSPACE ditegakkan RLS (GUC app.tenant_id di
-- WithTenant); tak ada filter tenant_id manual. playbooks TANPA soft-delete:
-- is_active=false = draf/nonaktif (playbook lama tetap terbaca meski tak
-- direkomendasikan lagi, riwayat penerapannya kalau ada tetap sah). Meniru
-- pola sla_policies.sql (A1).

-- name: ListPlaybooks :many
-- Playbook aktif untuk picker (dipakai saat CSM memilih playbook untuk
-- health event/journey — belum ada di slice ini, disiapkan untuk B1+). Hanya
-- is_active=true. Bounded katalog master per-workspace → tanpa keyset.
SELECT * FROM playbooks
WHERE is_active = true
ORDER BY playbook_name ASC, id ASC;

-- name: GetPlaybook :one
-- Satu playbook (baca detail/langkah). RLS menjamin tenant_id. Tak filter
-- is_active: handler memutuskan.
SELECT * FROM playbooks
WHERE id = sqlc.arg(id);

-- name: ListPlaybooksAll :many
-- Seluruh katalog untuk tampilan kelola (TERMASUK draf) — beda dari
-- ListPlaybooks (hanya aktif). Keyset (created_at DESC, id DESC) + LIMIT
-- (BL-6): katalog master pun bisa tumbuh, jadi halaman dibatasi & tetap
-- konsisten walau ada sisipan. Status aktif/draf tampak dari badge per baris,
-- bukan lagi dari urutan (grouping is_active dilepas demi kursor keyset satu-
-- kolom yang dipakai seluruh app).
SELECT * FROM playbooks
WHERE (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: CreatePlaybook :one
-- Buat playbook. tenant_id eksplisit (RLS WITH CHECK memverifikasinya = GUC).
-- is_active default true di skema tapi di-set eksplisit agar handler bisa
-- membuat draf nonaktif bila diperlukan nanti.
INSERT INTO playbooks (
    tenant_id, playbook_name, trigger_scenario,
    description, steps, recommended_owner, is_active, created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(playbook_name), sqlc.narg(trigger_scenario),
    sqlc.narg(description), sqlc.narg(steps), sqlc.narg(recommended_owner),
    sqlc.arg(is_active), sqlc.narg(created_by)
)
RETURNING *;

-- name: UpdatePlaybook :one
-- Sunting profil playbook. is_active TAK di sini (SetPlaybookActive) —
-- draf/aktifkan adalah aksi tersendiri, bukan efek samping edit.
UPDATE playbooks SET
    playbook_name     = sqlc.arg(playbook_name),
    trigger_scenario  = sqlc.narg(trigger_scenario),
    description       = sqlc.narg(description),
    steps             = sqlc.narg(steps),
    recommended_owner = sqlc.narg(recommended_owner),
    updated_by        = sqlc.narg(updated_by),
    updated_at        = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetPlaybookActive :exec
-- Jadikan draf (false) atau aktifkan kembali (true) playbook. Playbook draf
-- hilang dari ListPlaybooks (picker) tapi tetap tampil di kelola.
UPDATE playbooks SET
    is_active  = sqlc.arg(is_active),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id);
