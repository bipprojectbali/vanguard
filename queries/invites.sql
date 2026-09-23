-- name: CreateInvite :one
-- Undangan bergabung ke workspace. token = rahasia URL (crypto/rand hex via
-- oauth.NewState). email boleh milik orang yang BELUM punya akun. business_role
-- (nullable, BL-170) = Peran CRM yang sudah ditentukan admin di muka, diterapkan
-- otomatis saat invite diterima; kind = Jenis Anggota (internal/eksternal),
-- dipakai memfilter business_role yang valid di form Undang.
INSERT INTO invites (tenant_id, email, role, token, invited_by, expires_at, business_role, kind)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: DeletePendingInviteByEmail :execrows
-- Hapus undangan PENDING existing utk email yang sama di tenant ini (BL-170,
-- pola upsert-by-replace §c): dipanggil InviteCreate SEBELUM insert baru, agar
-- re-undang dengan field terbaru menggantikan yang lama alih-alih ditolak
-- bentrok atau menumpuk duplikat. :execrows (bukan :exec) agar pemanggil tahu
-- apakah ini undangan BARU atau KIRIM ULANG (>0 baris terhapus) — dipakai
-- memilih pesan sukses yang tepat (toast harus selalu tampil, lihat InviteCreate).
DELETE FROM invites
WHERE tenant_id = $1 AND lower(email) = lower($2) AND accepted_at IS NULL;

-- name: MemberExistsByEmail :one
-- Guard InviteCreate (BL-170 §b): true bila email sudah jadi anggota tenant ini.
-- Query TARGETED (JOIN memberships+users), BUKAN scan ListMembersByTenant penuh
-- — dipanggil tiap submit form Undang.
SELECT EXISTS (
    SELECT 1 FROM memberships m
    JOIN users u ON u.id = m.user_id
    WHERE m.tenant_id = $1 AND lower(u.email) = lower($2)
);

-- name: GetInviteByToken :one
-- Jalur PUBLIK (/invite/{token}) — penerima belum tentu login/anggota. Validasi
-- kedaluwarsa & sudah-dipakai dilakukan di handler agar pesannya spesifik.
SELECT i.*, t.name AS tenant_name, t.slug AS tenant_slug
FROM invites i
JOIN tenants t ON t.id = i.tenant_id
WHERE i.token = $1;

-- name: ListInvitesByTenant :many
-- Undangan PENDING satu workspace (panel anggota) — yang sudah diterima disaring.
SELECT * FROM invites
WHERE tenant_id = $1 AND accepted_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: AcceptInvite :exec
-- Tandai terpakai (one-time). Guard accepted_at IS NULL → race dua klik tak
-- menghasilkan dua membership (UNIQUE di memberships jadi jaring kedua).
UPDATE invites SET accepted_at = now()
WHERE token = $1 AND accepted_at IS NULL AND expires_at > now();

-- name: DeleteInvite :exec
-- Batalkan undangan yang belum diterima (sisi PENGUNDANG, di panel anggota).
DELETE FROM invites WHERE id = $1 AND tenant_id = $2;

-- name: ListPendingInvitesByEmail :many
-- Undangan yang ditujukan ke SATU ORANG (halaman notifikasi). Dicari per-email,
-- bukan user_id: saat diundang penerima belum tentu punya akun.
--
-- lower(email) = $1 — pemanggil WAJIB mengirim email yang sudah di-lowercase
-- (pola auth.go/invite.go). Ditopang index partial idx_invites_email.
SELECT i.*, t.name AS tenant_name
FROM invites i
JOIN tenants t ON t.id = i.tenant_id
WHERE lower(i.email) = $1 AND i.accepted_at IS NULL AND i.expires_at > now()
ORDER BY i.created_at DESC;

-- name: CountPendingInvitesByEmail :one
-- Bagian "perlu tindakan" dari badge. Undangan pending SENGAJA tak pernah
-- ter-auto-read: ia tugas, bukan kabar — lihat MarkNotificationsRead.
SELECT count(*)::bigint FROM invites
WHERE lower(email) = $1 AND accepted_at IS NULL AND expires_at > now();

-- name: DeclineInvite :exec
-- Tolak undangan (sisi PENERIMA). Kunci ganda token + email: penerima tak punya
-- scope ke workspace pengundang, jadi DeleteInvite (yang butuh tenant_id) tak
-- bisa dipakai. Mencocokkan email mencegah pemegang token menolak undangan
-- milik orang lain.
DELETE FROM invites
WHERE token = $1 AND lower(email) = $2 AND accepted_at IS NULL;
