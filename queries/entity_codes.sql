-- name: NextEntityCodeSeq :one
-- Alokasi nomor urut BERIKUTNYA untuk (tenant, entity), ATOMIK.
--
-- Kenapa satu pernyataan INSERT..ON CONFLICT dan bukan SELECT max+1 di Go: ON
-- CONFLICT DO UPDATE mengambil ROW LOCK pada baris counter, jadi dua create
-- bersamaan diserialkan oleh Postgres — mustahil dapat nomor sama. Menghitung di
-- aplikasi (baca lalu tulis) justru race yang paling sering lolos sampai dua
-- baris bertabrakan di produksi.
--
-- next_val = "nomor yang akan DIPAKAI berikutnya" (mulai 1). Kita menyimpan hasil
-- SESUDAH dinaikkan lalu pemanggil mengurangi 1 (lihat GenerateEntityCode):
--   • baris belum ada → INSERT next_val=2, RETURNING 2 → alokasi 2-1=1
--   • baris next_val=N → UPDATE jadi N+1, RETURNING N+1 → alokasi (N+1)-1=N
-- RETURNING kolom mentah (BIGINT NOT NULL) supaya sqlc mengetiknya int64, bukan
-- pointer nullable (ekspresi aritmetika di RETURNING akan ditebak nullable).
INSERT INTO code_sequences (tenant_id, entity, next_val)
VALUES ($1, $2, 2)
ON CONFLICT (tenant_id, entity) DO UPDATE
SET next_val = code_sequences.next_val + 1
RETURNING next_val;

-- name: GetCodeFormat :one
-- Format kode satu (tenant, entity). Tak ada baris = SAH: pemanggil jatuh ke
-- codes.DefaultFormat (workspace baru berkode tanpa seed). pgx.ErrNoRows bukan
-- kegagalan yang perlu diributkan.
SELECT * FROM code_formats WHERE tenant_id = $1 AND entity = $2;

-- name: ListCodeFormats :many
-- Semua format kode workspace, untuk halaman pengaturan. Sedikit barisnya (satu
-- per entitas), jadi tak dipaginasi. Urut per entity agar tampilannya stabil.
SELECT * FROM code_formats WHERE tenant_id = $1 ORDER BY entity;

-- name: UpsertCodeFormat :exec
-- Simpan/ubah format satu entitas. UPSERT: baris mungkin belum ada (default masih
-- dipakai) — operator hanya menyentuh baris saat ingin MENGUBAH default. created_by
-- diisi saat pertama dibuat; updated_by/updated_at tiap kali diubah.
INSERT INTO code_formats (tenant_id, entity, prefix, separator, padding, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (tenant_id, entity) DO UPDATE
SET prefix     = EXCLUDED.prefix,
    separator  = EXCLUDED.separator,
    padding    = EXCLUDED.padding,
    updated_by = EXCLUDED.updated_by,
    updated_at = now();
