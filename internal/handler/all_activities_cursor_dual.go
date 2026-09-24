package handler

import (
	"encoding/hex"
	"strings"
)

// all_activities_cursor_dual.go — perakitan PASANGAN sub-cursor (activities +
// engagements) jadi satu token ?after= untuk feed terpadu, dipisah dari
// all_activities_cursor.go (codec satu sub-cursor) agar file itu di bawah
// ambang tipe Route/Handler (150). Rasional format & migrasi BL-157k ada di
// header all_activities_cursor.go.

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
