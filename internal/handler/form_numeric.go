package handler

import "strings"

// form_numeric.go — helper FORM bertema angka yang dipakai bersama lintas modul,
// pelengkap form_optional.go (kosong=NULL) & sales_format_number.go (parse/format
// numerik). Netral-domain: aturan "bersihkan pemisah ribuan" & "nomor telepon
// opsional yang sah" harus SATU tempat agar tiap modul memperlakukannya sama.

// cleanThousands membuang pemisah ribuan (titik) & spasi dari input UANG BULAT
// (mis. "5.000.000" → "5000000") sebelum optNumeric. Dipakai HANYA utk field
// rupiah bulat (leads estimated_value, dst.) yang TAK berdesimal — JANGAN utk
// field berdesimal (rate/persentase) karena di sana titik adalah pemisah desimal.
// Menjaga backend tetap penjaga: input terkelompok dari numgroup.js (BL-2) maupun
// ketikan manual "5.000.000" sama sahnya, dan tanpa JS pun form tetap jalan.
func cleanThousands(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// optPhone memvalidasi nomor telepon OPSIONAL (HP/WhatsApp lead, dst.): kosong →
// (nil, ""); terisi wajib "berupa nomor" — hanya digit, opsional '+' di DEPAN
// (prefix negara), boleh spasi/tanda hubung sebagai pemisah, dan 6–20 digit.
// MEMPERTAHANKAN format asli pengguna (leading zero "0812…" & prefix "+62…"
// utuh) — hanya memvalidasi, tak menormalkan — karena type="number" HTML akan
// membuang keduanya (BL-2 sengaja pakai type="tel"+inputmode). code dioper
// pemanggil ("mobile_phone"/"whatsapp") agar pesan galat menyebut field.
func optPhone(s, code string) (*string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	digits := 0
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+':
			if i != 0 { // '+' hanya boleh sebagai prefix paling depan.
				return nil, code
			}
		case r == ' ' || r == '-':
			// pemisah kosmetik — dibolehkan, tak dihitung sbg digit.
		default:
			return nil, code
		}
	}
	if digits < 6 || digits > 20 {
		return nil, code
	}
	return &s, ""
}
