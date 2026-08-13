-- kb_articles.sql — katalog master (Knowledge Base), Modul 6 Customer
-- Success slice A3. Isolasi WORKSPACE ditegakkan RLS (GUC app.tenant_id di
-- WithTenant); tak ada filter tenant_id manual. kb_articles TANPA
-- soft-delete: status='Draft' = draf, bukan terhapus. Meniru pola
-- playbooks.sql (A2), status 3-nilai (bukan is_active boolean) meniru
-- leads_status_chk/subs_status_chk.

-- name: ListKBArticlesAll :many
-- Seluruh katalog untuk tampilan kelola (semua status). Terbaru diperbarui
-- dulu — perilaku umum KB admin view. Bounded katalog master per-workspace
-- → tanpa keyset.
SELECT * FROM kb_articles
ORDER BY updated_at DESC, id DESC;

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
-- Transisi status (Draft/Review/Published — CHECK di DB menegakkan domain
-- nilai). Handler yang memutuskan transisi mana yang ditawarkan per baris.
UPDATE kb_articles SET
    status     = sqlc.arg(status),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id);
