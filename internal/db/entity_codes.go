// entity_codes.go — jembatan ANTARA counter atomik di DB (entity_codes.sql.go,
// generated) DAN aturan perakitan kode di internal/codes (murni, tanpa DB). File
// ini ditulis tangan: sqlc tak menyentuhnya.
//
// Pembagian tugas yang disengaja:
//   - internal/codes  → BENTUK kode (prefix/separator/padding) & default. Murni,
//     bisa diuji tanpa DB, dipakai juga oleh handler/tampilan.
//   - queries (sqlc)  → alokasi nomor ATOMIK & simpanan format per tenant.
//   - di sini         → merangkai keduanya jadi satu kode siap-simpan.
//
// Kenapa dipisah begini: perakitan kode adalah aturan tampilan yang harus SAMA di
// titik pembuatan (sini) dan titik validasi/preview (handler). Menaruh render di
// paket murni membuatnya mustahil bercabang jadi dua versi kebenaran.
package db

import (
	"context"

	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5"
)

// GenerateEntityCode mengalokasikan SATU kode baru untuk entity di tenant ini dan
// mengembalikannya siap-simpan (mis. "DESA-001"). Dipanggil DI DALAM tx create
// (WithTenant) supaya alokasi nomor dan INSERT baris entitasnya berbagi transaksi
// yang sama — kalau create di-rollback, nomor tak "hangus" menyisakan lubang.
//
// Alurnya:
//  1. Ambil nomor urut berikutnya secara atomik (row lock via ON CONFLICT).
//  2. Ambil format tersimpan tenant; bila belum ada (pgx.ErrNoRows) → default
//     bawaan codes. Baris format absen BUKAN error — itu keadaan normal workspace
//     yang belum menyetel.
//  3. Rakit kode dari format + nomor (codes.Format.Render — tak pernah memotong).
//
// Catatan konkurensi: nomor dijamin unik oleh row lock counter, TAPI unik final
// kode tetap dijaga index unik (idx_accounts_entity_code) sebagai jaring — bila
// format diubah sehingga dua nomor berbeda merender string sama (mis. padding
// dikecilkan), INSERT-nya yang gagal, bukan data yang diam-diam bertabrakan.
func (q *Queries) GenerateEntityCode(ctx context.Context, tenantID int64, entity codes.Entity) (string, error) {
	// (1) Nomor urut atomik. RETURNING adalah next_val SESUDAH dinaikkan, jadi
	// nomor yang dialokasikan = hasil - 1 (lihat komentar query).
	next, err := q.NextEntityCodeSeq(ctx, NextEntityCodeSeqParams{
		TenantID: tenantID,
		Entity:   string(entity),
	})
	if err != nil {
		return "", err
	}
	seq := next - 1

	// (2) Format tersimpan, atau default bila belum diset.
	f, err := q.codeFormatOrDefault(ctx, tenantID, entity)
	if err != nil {
		return "", err
	}

	// (3) Rakit.
	return f.Render(seq), nil
}

// codeFormatOrDefault membaca format tenant untuk entity, jatuh ke default bawaan
// codes bila baris belum ada. Nilai DB yang tersimpan diasumsikan sudah lolos
// ValidFormat saat di-upsert (handler memvalidasinya), jadi tak divalidasi ulang
// di jalur panas pembuatan.
func (q *Queries) codeFormatOrDefault(ctx context.Context, tenantID int64, entity codes.Entity) (codes.Format, error) {
	row, err := q.GetCodeFormat(ctx, GetCodeFormatParams{
		TenantID: tenantID,
		Entity:   string(entity),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return codes.DefaultFormat(entity), nil
		}
		return codes.Format{}, err
	}
	return codes.Format{
		Prefix:    row.Prefix,
		Separator: row.Separator,
		Padding:   int(row.Padding),
	}, nil
}
