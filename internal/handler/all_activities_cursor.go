package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// all_activities_cursor.go — codec CURSOR KOMPOSIT lintas-tabel untuk feed
// terpadu, dipisah dari all_activities_unified.go (ukuran file). Rasional keyset
// dua-sumber ada di doc all_activities_unified.go (#3); logika di sini identik.

// subCursor = posisi keyset (created_at, id) satu sumber. Salinan bentuk yang
// dipakai pageCursor, tapi eksplisit agar cursor komposit merakit dua darinya.
type subCursor struct {
	at pgtype.Timestamptz
	id int64
}

// dualCursor = pasangan sub-cursor (activities + engagements) untuk satu posisi
// halaman feed terpadu. Tiap sumber maju independen: sumber yang tak menyumbang
// baris ke halaman ini biarkan sub-cursornya tak berubah (halaman berikut mulai
// dari titik yang sama untuk sumber itu).
type dualCursor struct {
	act subCursor
	eng subCursor
}

// firstSubCursor = sub-cursor halaman pertama: (created_at, id) maksimum
// (Infinity, MaxInt64) sehingga semua baris lolos syarat keyset `< cursor`.
// Sejajar firstPageCursor (dev_users.go) tapi dibungkus subCursor.
func firstSubCursor() subCursor {
	at, id := firstPageCursor()
	return subCursor{at: at, id: id}
}

// firstDualCursor = posisi awal feed (kedua sumber di halaman pertama).
func firstDualCursor() dualCursor {
	return dualCursor{act: firstSubCursor(), eng: firstSubCursor()}
}

// subFieldSentinel menandai sub-cursor "di awal" (firstSubCursor) dalam token.
// Panjang 1 → mustahil bentrok dengan pengkodean baris nyata (selalu ≥20 char:
// 19 digit nano + ≥1 digit id).
const subFieldSentinel = "0"

// subFieldNanoWidth = lebar zero-pad UnixNano dalam token. MaxInt64 = 19 digit;
// nano baris nyata (pasca-1970) selalu positif & muat 19 digit, jadi lebar tetap
// membuat pemisahan nano|id deterministik (potong di indeks 19).
const subFieldNanoWidth = 19

// encodeSubCursor mengemas satu sub-cursor jadi field digit-murni untuk token.
// firstSubCursor / tak-valid → sentinel "0". Baris nyata → 19-digit-nano + id.
func encodeSubCursor(c subCursor) string {
	if !c.at.Valid || c.at.InfinityModifier != pgtype.Finite {
		return subFieldSentinel
	}
	nano := c.at.Time.UTC().UnixNano()
	if nano < 0 {
		// Mustahil di praktik (created_at selalu pasca-1970); jaga token tetap
		// digit-murni bila terjadi anomali data.
		return subFieldSentinel
	}
	return fmt.Sprintf("%0*d%d", subFieldNanoWidth, nano, c.id)
}

// decodeSubCursor membalik encodeSubCursor. Sentinel/rusak/kependekan → halaman
// pertama sub-sumber (fail-open ke awal, sejalan filosofi pageCursor: masukan
// URL yang bisa disunting tak boleh menggagalkan render).
func decodeSubCursor(field string) subCursor {
	if len(field) < subFieldNanoWidth+1 {
		return firstSubCursor()
	}
	nano, err1 := strconv.ParseInt(field[:subFieldNanoWidth], 10, 64)
	id, err2 := strconv.ParseInt(field[subFieldNanoWidth:], 10, 64)
	if err1 != nil || err2 != nil {
		return firstSubCursor()
	}
	return subCursor{
		at: pgtype.Timestamptz{Time: time.Unix(0, nano).UTC(), Valid: true},
		id: id,
	}
}

// encodeDualCursor merakit token halaman = "<fieldAct>_<fieldEng>". Tepat satu
// underscore, kedua sisi digit-murni → lolos ui.validTrailToken, jadi mengalir
// lewat jejak BL-7 tanpa perlakuan khusus.
func encodeDualCursor(c dualCursor) string {
	return encodeSubCursor(c.act) + cursorSep + encodeSubCursor(c.eng)
}

// decodeDualCursor membaca ?after= jadi dualCursor. Kosong/rusak → halaman
// pertama (kedua sumber dari awal).
func decodeDualCursor(raw string) dualCursor {
	if raw == "" {
		return firstDualCursor()
	}
	actField, engField, ok := strings.Cut(raw, cursorSep)
	if !ok {
		return firstDualCursor()
	}
	return dualCursor{act: decodeSubCursor(actField), eng: decodeSubCursor(engField)}
}
