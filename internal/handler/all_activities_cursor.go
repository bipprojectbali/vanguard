package handler

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// all_activities_cursor.go — codec CURSOR KOMPOSIT lintas-tabel untuk feed
// terpadu, dipisah dari all_activities_unified.go (ukuran file). Rasional
// keyset dua-sumber ada di doc all_activities_unified.go (#3); logika di sini
// identik.
//
// BL-157k GENERALISASI: format lama (sebelum modul ini) hanya membawa
// (created_at, id) per sumber — cukup selama satu-satunya sumbu urut adalah
// Tanggal. Menambah 4 sumbu (Jenis/Subjek/Pemilik/Status) butuh sub-cursor
// yang membawa NILAI TEKS (mungkin NULL) per sumber, bukan cuma timestamp.
// Diganti MENYELURUH (bukan disambung di sebelah yang lama) — dua format
// cursor komposit hidup bersisian untuk satu param ?after= butuh sniffing
// rapuh, sementara token di sini murni navigasi tak ditandatangani (rusak →
// halaman pertama, filosofi pageCursor); migrasi format sekali-jalan sejalan
// preseden BL-157j (pageCursorTimestamp juga format baru, bukan sambungan).

// genSubCursor = posisi keyset SATU SUMBER pada sumbu sort AKTIF, dinormalisasi
// ke bentuk (isNull, val, id) yang sama untuk kelima sumbu: created_at
// diencode sebagai 19-digit zero-padded UnixNano (lihat subCursorTimeVal)
// sehingga perbandingan STRING == perbandingan waktu asli — satu comparator
// generik (pageEntries) melayani seluruh sumbu, tak perlu percabangan
// per-tipe di merge-sort.
type genSubCursor struct {
	hasCursor bool // false = sumber ini di halaman 1 (lewati predikat keyset SQL)
	isNull    bool // bermakna hanya bila hasCursor
	val       string
	id        int64
}

// firstGenSubCursor = posisi halaman-1 satu sumber (sisanya tak bermakna).
func firstGenSubCursor() genSubCursor { return genSubCursor{} }

// dualCursorGen = pasangan sub-cursor (activities + engagements) untuk satu
// posisi halaman feed terpadu. Tiap sumber maju independen: sumber yang tak
// menyumbang baris ke halaman ini biarkan sub-cursornya tak berubah (halaman
// berikut mulai dari titik yang sama untuk sumber itu).
type dualCursorGen struct {
	act genSubCursor
	eng genSubCursor
}

func firstDualCursorGen() dualCursorGen {
	return dualCursorGen{act: firstGenSubCursor(), eng: firstGenSubCursor()}
}

// subCursorTimeWidth = lebar zero-pad UnixNano. MaxInt64 = 19 digit; nano
// baris nyata (pasca-1970) selalu positif & muat 19 digit → pemisahan
// deterministik & perbandingan string == perbandingan waktu.
const subCursorTimeWidth = 19

// subCursorTimeVal mengencode UnixNano (UTC) jadi val 19-digit dipakai sumbu
// Tanggal (satu-satunya sumbu bertipe waktu; sumbu lain sudah string alami).
func subCursorTimeVal(nano int64) string {
	if nano < 0 {
		nano = 0 // mustahil di praktik (created_at pasca-1970); jaga digit-murni.
	}
	return fmt.Sprintf("%0*d", subCursorTimeWidth, nano)
}

// parseSubCursorTimeVal membalik subCursorTimeVal → UnixNano. val rusak → 0.
func parseSubCursorTimeVal(val string) int64 {
	nano, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}
	return nano
}

// ── Pengkodean satu sub-cursor: DIPANJANGKAN-AWALAN (length-prefixed), bukan
// pembatas — val bisa berisi karakter apa pun (mis. subjek bebas user), jadi
// tak ada pembatas yang aman. started/isNull 1 digit masing², panjang val
// 10-digit zero-pad, id 19-digit zero-pad (int64 max). Fully self-delimiting
// → dua sub-cursor bisa disambung TANPA pembatas di antaranya dan tetap
// terbaca lagi (decodeGenSub kedua lanjut persis di sisa byte setelah
// sub-cursor pertama). ───────────────────────────────────────────────────
const (
	subValLenWidth = 10
	subIDWidth     = 19
)

func encodeGenSub(c genSubCursor) string {
	started, isNull := "0", "0"
	if c.hasCursor {
		started = "1"
	}
	if c.isNull {
		isNull = "1"
	}
	val := c.val
	if !c.hasCursor || c.isNull {
		val = "" // tak bermakna; jaga blob minimal & deterministik.
	}
	return started + isNull +
		fmt.Sprintf("%0*d", subValLenWidth, len(val)) + val +
		fmt.Sprintf("%0*d", subIDWidth, c.id)
}

// decodeGenSub membaca satu sub-cursor dari AWAL blob, mengembalikan sisa byte
// (untuk sub-cursor berikutnya dalam blob yang sama) dan ok=false bila blob
// terlalu pendek/rusak.
func decodeGenSub(blob string) (c genSubCursor, remaining string, ok bool) {
	const headerLen = 1 + 1 + subValLenWidth
	if len(blob) < headerLen {
		return genSubCursor{}, "", false
	}
	valLen, err := strconv.Atoi(blob[2:headerLen])
	if err != nil || valLen < 0 {
		return genSubCursor{}, "", false
	}
	rest := blob[headerLen:]
	if len(rest) < valLen+subIDWidth {
		return genSubCursor{}, "", false
	}
	val := rest[:valLen]
	id, err := strconv.ParseInt(rest[valLen:valLen+subIDWidth], 10, 64)
	if err != nil {
		return genSubCursor{}, "", false
	}
	return genSubCursor{
		hasCursor: blob[0] == '1',
		isNull:    blob[1] == '1',
		val:       val,
		id:        id,
	}, rest[valLen+subIDWidth:], true
}

// encodeDualCursorGen merakit token halaman: hex(blob) + cursorSep + "0".
// Blob = dua sub-cursor bersambung (act lalu eng), masing² self-delimiting
// (lihat encodeGenSub) — tak perlu pembatas di antaranya. "_0" akhir dummy
// (SELURUH state ada di blob hex) — sengaja dipertahankan agar token tetap
// berbentuk <hexdigits>_<decimaldigits>, memenuhi ui.validTrailToken TANPA
// menyentuh pager.go (pola sama sortcursor.go: manfaatkan grammar yang sudah
// ada, jangan ubah lagi).
func encodeDualCursorGen(c dualCursorGen) string {
	blob := encodeGenSub(c.act) + encodeGenSub(c.eng)
	return hex.EncodeToString([]byte(blob)) + cursorSep + "0"
}

// decodeDualCursorGen membaca ?after= jadi dualCursorGen. Kosong/rusak →
// halaman pertama (fail-open, filosofi pageCursor: URL yang bisa disunting
// tak boleh menggagalkan render).
func decodeDualCursorGen(raw string) dualCursorGen {
	if raw == "" {
		return firstDualCursorGen()
	}
	hexBlob, _, found := strings.Cut(raw, cursorSep)
	if !found {
		return firstDualCursorGen()
	}
	b, err := hex.DecodeString(hexBlob)
	if err != nil {
		return firstDualCursorGen()
	}
	act, rest, ok := decodeGenSub(string(b))
	if !ok {
		return firstDualCursorGen()
	}
	eng, rest2, ok := decodeGenSub(rest)
	if !ok || rest2 != "" {
		return firstDualCursorGen()
	}
	return dualCursorGen{act: act, eng: eng}
}
