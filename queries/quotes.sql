-- quotes.sql — penawaran (Quote) + baris (quote_item), Modul 4 Sales slice 2.
-- Isolasi WORKSPACE ditegakkan RLS (GUC app.tenant_id di WithTenant). Ownership
-- F3 TAK ditegakkan di sini: quote diakses NEST di bawah deal (/deals/{id}/quotes),
-- handler memuat deal ber-owner dulu (loadOwnedDeal) lalu quote-nya — tak ada kolom
-- quote_owner. grand_total/tax_amount = SNAPSHOT: dihitung ulang app dari quote_items
-- tiap item berubah (bukan agregat live saat baca).
-- entity_code (QUO-001) dialokasikan GenerateEntityCode DALAM tx create.

-- name: CreateQuote :one
-- Buat quote. tenant_id di-set eksplisit (RLS WITH CHECK memverifikasinya = GUC).
-- deal_id nullable di skema tapi selalu terisi di alur nest; account_id NN = jangkar.
INSERT INTO quotes (
    tenant_id, entity_code, deal_id, account_id,
    quote_name, quote_status, expiration_date, payment_terms, notes_terms,
    prepared_by, grand_total, tax_amount, created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(entity_code), sqlc.narg(deal_id), sqlc.arg(account_id),
    sqlc.narg(quote_name), sqlc.arg(quote_status), sqlc.narg(expiration_date),
    sqlc.narg(payment_terms), sqlc.narg(notes_terms),
    sqlc.narg(prepared_by), sqlc.narg(grand_total), sqlc.narg(tax_amount),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetQuote :one
-- Satu quote hidup. RLS menjamin tenant_id; kelayakan akses (via deal ber-owner)
-- diputuskan handler sebelum memanggil ini.
SELECT * FROM quotes
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListQuotesForDeal :many
-- Daftar quote milik satu deal (detail deal → daftar quote-nya), keyset
-- (created_at DESC, id DESC). Deal sudah ter-scope ownership di handler; di sini
-- cukup filter deal_id + baris hidup. First page: cursor = (now(), max bigint).
SELECT * FROM quotes
WHERE deleted_at IS NULL
  AND deal_id = sqlc.arg(deal_id)
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListQuotes :many
-- Daftar quote LINTAS-deal (menu Quotes global), keyset (created_at DESC, id DESC)
-- + filter ownership F3 DIWARISI dari deal induk (JOIN deals → deal_owner). Dua flag
-- sama dengan ListDeals: scope_all → semua; is_own → deal_owner = uid; keduanya
-- false → NOL baris (fail-closed). INNER JOIN deals: quote selalu menempel ke deal
-- (deal_id di-set saat create); quote tanpa deal hidup TAK tampil di daftar global
-- (tak punya owner untuk disaring). deal_name dibawa untuk kolom "Deal".
--
-- search '' → tak menyaring; selain itu MEMPERSEMPIT (ILIKE substring, case-
-- insensitive) di ATAS ownership warisan deal — tak pernah melebarkan baris. Kolom
-- tak-tersamar yang tampil di tabel: quote_name + entity_code (quote) + d.deal_name
-- (deal induk); nilai grand total tak dijadikan kunci cari.
SELECT q.*, d.deal_name
FROM quotes q
JOIN deals d ON d.id = q.deal_id
WHERE q.deleted_at IS NULL
  AND d.deleted_at IS NULL
  AND (q.created_at, q.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND d.deal_owner = sqlc.arg(uid))
  )
  AND (
      sqlc.arg(search)::text = ''
      OR q.quote_name ILIKE '%' || sqlc.arg(search) || '%'
      OR q.entity_code ILIKE '%' || sqlc.arg(search) || '%'
      OR d.deal_name ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY q.created_at DESC, q.id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateQuote :one
-- Sunting profil quote. quote_status punya jalur khusus (UpdateQuoteStatus) dan
-- total punya jalur khusus (UpdateQuoteTotals) — keduanya TAK di sini agar
-- perubahan status & rekalkulasi harga terlihat sebagai aksi tersendiri.
UPDATE quotes SET
    quote_name      = sqlc.narg(quote_name),
    deal_id         = sqlc.narg(deal_id),
    account_id      = sqlc.arg(account_id),
    expiration_date = sqlc.narg(expiration_date),
    payment_terms   = sqlc.narg(payment_terms),
    notes_terms     = sqlc.narg(notes_terms),
    prepared_by     = sqlc.narg(prepared_by),
    updated_by      = sqlc.narg(updated_by),
    updated_at      = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: UpdateQuoteStatus :exec
-- Transisi status (Draft→Sent→…). Aturan transisi valid divalidasi handler (bukan
-- constraint DB) agar pesan bisa diperbaiki user; CHECK di DB hanya membatasi
-- himpunan nilai legal.
UPDATE quotes SET
    quote_status = sqlc.arg(quote_status),
    updated_by   = sqlc.narg(updated_by),
    updated_at   = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: UpdateQuoteTotals :exec
-- Rekalkulasi total quote (snapshot) setelah item berubah. grand_total & tax_amount
-- dihitung app dari quote_items lalu ditulis di sini — bukan agregat live saat baca.
UPDATE quotes SET
    grand_total = sqlc.narg(grand_total),
    tax_amount  = sqlc.narg(tax_amount),
    updated_by  = sqlc.narg(updated_by),
    updated_at  = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SoftDeleteQuote :exec
-- Soft-delete: quote disembunyikan dari list/get tapi tetap ada. quote_items
-- anaknya di-hard-delete oleh CASCADE hanya bila quote benar-benar di-DROP —
-- di sini baris quote hanya ditandai, item tetap tersimpan.
UPDATE quotes SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;


-- ── quote_items — baris penawaran (hard-delete, tanpa soft-delete/audit) ──────

-- name: AddQuoteItem :one
-- Tambah baris item. tenant_id eksplisit (RLS WITH CHECK). unit_price & subtotal =
-- SNAPSHOT: unit_price disalin dari plans.base_price saat dibuat, subtotal dihitung
-- app (unit_price * quantity * (1 - discount_pct/100)); keduanya beku sesudahnya.
INSERT INTO quote_items (
    quote_id, tenant_id, plan_id, quantity, unit_price, discount_pct, subtotal, line_no
) VALUES (
    sqlc.arg(quote_id), sqlc.arg(tenant_id), sqlc.narg(plan_id), sqlc.arg(quantity),
    sqlc.arg(unit_price), sqlc.narg(discount_pct), sqlc.arg(subtotal), sqlc.narg(line_no)
)
RETURNING *;

-- name: ListQuoteItems :many
-- Baris item satu quote, urut tampil (line_no lalu id). Menopang detail quote &
-- rekalkulasi total. Bounded per-quote (bukan daftar global) → tanpa keyset.
SELECT * FROM quote_items
WHERE quote_id = sqlc.arg(quote_id)
ORDER BY line_no ASC NULLS LAST, id ASC;

-- name: UpdateQuoteItem :one
-- Sunting qty/diskon satu item; subtotal (snapshot) dihitung ulang app dan ditulis
-- di sini. unit_price TAK diubah (tetap snapshot harga saat item dibuat).
UPDATE quote_items SET
    quantity     = sqlc.arg(quantity),
    discount_pct = sqlc.narg(discount_pct),
    subtotal     = sqlc.arg(subtotal),
    line_no      = sqlc.narg(line_no)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteQuoteItem :exec
-- Hard-delete satu baris item (tanpa soft-delete). Total quote direkalkulasi app
-- setelahnya via UpdateQuoteTotals.
DELETE FROM quote_items WHERE id = sqlc.arg(id);

-- name: QuoteItemsSubtotal :one
-- Jumlah subtotal seluruh item satu quote dalam satu round-trip (untuk rekalkulasi
-- grand_total tanpa memuat semua baris ke Go). COALESCE(...)::numeric membungkus
-- agregat agar sqlc tak meng-emit interface{} (gotcha #14).
SELECT COALESCE(SUM(subtotal), 0)::numeric AS items_subtotal
FROM quote_items
WHERE quote_id = sqlc.arg(quote_id);
