-- kb_articles.sql — katalog master (Knowledge Base), Modul 6 Customer
-- Success slice A3. Isolasi WORKSPACE ditegakkan RLS (GUC app.tenant_id di
-- WithTenant); tak ada filter tenant_id manual. kb_articles TANPA
-- soft-delete: status='Draft' = draf, bukan terhapus. Meniru pola
-- playbooks.sql (A2), status 3-nilai (bukan is_active boolean) meniru
-- leads_status_chk/subs_status_chk.

-- name: ListKBArticlesAll :many
-- Katalog untuk tampilan kelola. Keyset (created_at DESC, id DESC) + LIMIT
-- (BL-6): katalog master pun bisa tumbuh, jadi halaman dibatasi & tetap
-- konsisten walau ada sisipan. Urutan pindah dari updated_at ke created_at
-- (kolom kursor STABIL: updated_at berubah saat artikel disunting → baris bisa
-- lompat antar-halaman saat paging; created_at tetap, sesuai konvensi keyset
-- app). Status tampak dari badge per baris.
--
-- BL-34: tab Aktif vs Arsip via SATU boolean `only_archived` (tanpa query
-- kedua). only_archived=false → status <> 'Archived' (Draft+Published, tab
-- default "Aktif"); only_archived=true → status = 'Archived' (tab "Arsip").
-- Arsip = pensiun-tanpa-hapus → disembunyikan dari daftar default.
--
-- BL-36: pencarian bebas (?q=), mengaktifkan `keywords` yang semula write-only.
-- search '' → tak menyaring; selain itu MEMPERSEMPIT di ATAS tab (ILIKE
-- substring, case-insensitive) — tak pernah melebarkan baris. Kolom cari =
-- judul + kata kunci + kategori (findability tiket → artikel). article_body
-- SENGAJA di luar kunci cari (bisa besar → relevansi kabur & mahal). keywords/
-- category NULL → ILIKE NULL = NULL → cabang OR false (aman). Parameter
-- ter-bind (BUKAN string-concat) → anti-injeksi; metachar LIKE (%/_) dibiarkan
-- literal-wildcard, konsisten daftar BL-6 lain (subscriptions).
SELECT * FROM kb_articles
WHERE (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
        (sqlc.arg(only_archived)::bool AND status = 'Archived')
     OR (NOT sqlc.arg(only_archived)::bool AND status <> 'Archived')
  )
  AND (
        sqlc.arg(search)::text = ''
     OR article_title ILIKE '%' || sqlc.arg(search) || '%'
     OR keywords ILIKE '%' || sqlc.arg(search) || '%'
     OR category ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: GetKBArticle :one
-- Satu artikel (baca detail/isi). RLS menjamin tenant_id.
SELECT * FROM kb_articles
WHERE id = sqlc.arg(id);

-- name: CreateKBArticle :one
-- Buat artikel. tenant_id eksplisit (RLS WITH CHECK memverifikasinya = GUC).
-- author_id = created_by (penulis awal; slice ini tak sediakan picker
-- reassign penulis — lihat komentar migrasi 00018).
INSERT INTO kb_articles (
    tenant_id, article_title, article_body, category, keywords,
    status, visibility, author_id, created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(article_title), sqlc.narg(article_body),
    sqlc.narg(category), sqlc.narg(keywords), sqlc.arg(status),
    sqlc.arg(visibility), sqlc.narg(author_id), sqlc.narg(created_by)
)
RETURNING *;

-- name: UpdateKBArticle :one
-- Sunting profil/isi artikel. status TAK di sini (SetKBArticleStatus) —
-- transisi status adalah aksi tersendiri, bukan efek samping edit.
UPDATE kb_articles SET
    article_title = sqlc.arg(article_title),
    article_body  = sqlc.narg(article_body),
    category      = sqlc.narg(category),
    keywords      = sqlc.narg(keywords),
    visibility    = sqlc.arg(visibility),
    updated_by    = sqlc.narg(updated_by),
    updated_at    = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetKBArticleStatus :exec
-- Transisi status (Draft/Published/Archived — CHECK di DB menegakkan domain
-- nilai, BL-34). Handler yang memutuskan transisi mana yang ditawarkan per
-- baris (Draft→Published, Published→Draft/Archived, Archived→Draft).
UPDATE kb_articles SET
    status     = sqlc.arg(status),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id);
