// Package desaplus — HTTP client tipis ke API integrasi khusus produk
// desa-plus ("Des Plus"), dipakai BL-27 untuk mengisi otomatis section
// Product Adoption di Customer Success. Adapter TIPIS: satu method, satu
// endpoint, tak menyimpan state lintas-request — mirip prinsip internal/mcpserver.
package desaplus

import (
	"regexp"
	"strconv"
	"time"
)

// VillageSummary mencerminkan payload `data` pada balasan sukses
// GET /village-summary?codeVanguard=... — diverifikasi baca LANGSUNG source
// sistem-desa-mandiri (endpoint swagger mereka rusak, tak bisa dipakai).
type VillageSummary struct {
	Village            Village       `json:"village"`
	LastActivity       *LastActivity `json:"lastActivity"`
	ActiveUsers        int32         `json:"activeUsers"`
	TopFeatures        []TopFeature  `json:"topFeatures"`
	TodayActivityCount int32         `json:"todayActivityCount"`
}

// Village — identitas desa di sisi desa-plus (dipakai untuk log/debug,
// BUKAN dipetakan ke kolom manapun — vanguard tetap sumber kebenaran nama desa).
type Village struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CodeVanguard string `json:"codeVanguard"`
}

// LastActivity — aktivitas TERCATAT terakhir, SENGAJA tak termasuk aksi
// LOGIN/LOGOUT (dikonfirmasi desa-plus by design — lihat komentar route.ts
// mereka: "tidak termasuk LOGIN dan LOGOUT"). Karena itu label UI vanguard
// memakai "Aktivitas Terakhir", bukan "Login Terakhir" (keputusan user BL-27).
type LastActivity struct {
	Action    string `json:"action"`
	Feature   string `json:"feature"`
	Desc      string `json:"desc"`
	User      string `json:"user"`
	Timestamp string `json:"timestamp"` // string SUDAH diformat, lihat Date()
	Since     string `json:"since"`     // mis. "2 hari lalu", tak dipetakan
}

// TopFeature — satu entri `topFeatures` (maks 3, sudah diurutkan server
// desa-plus dari yang paling sering dipakai).
type TopFeature struct {
	Feature string `json:"feature"`
	Count   int32  `json:"count"`
}

// indonesianMonths — singkatan bulan hasil Intl.DateTimeFormat('id-ID',...)
// Node.js (server desa-plus), DIVERIFIKASI LANGSUNG (bukan ditebak — "Mei",
// "Agu", "Okt", "Des" berbeda dari singkatan Inggris "May"/"Aug"/"Oct"/"Dec").
var indonesianMonths = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March, "Apr": time.April,
	"Mei": time.May, "Jun": time.June, "Jul": time.July, "Agu": time.August,
	"Sep": time.September, "Okt": time.October, "Nov": time.November, "Des": time.December,
}

// timestampPattern menangkap "DD Mon YYYY" di awal string mis.
// "21 Sep 2026, 15.05" — jam/menit setelah koma sengaja diabaikan, lihat Date().
var timestampPattern = regexp.MustCompile(`^(\d{1,2}) (\p{L}{3}) (\d{4}),`)

// Date mem-parse HANYA komponen tanggal (bukan jam) dari Timestamp — string
// yang SUDAH diformat locale id-ID oleh server desa-plus (BUKAN ISO-8601;
// mis. "21 Sep 2026, 15.05"). Kolom tujuan (`customer_success.last_login_date`)
// bertipe DATE, jadi jam tak relevan — dan jam yang ada pun sudah dalam TZ
// lokal SERVER desa-plus yang tak diketahui, jadi tak seharusnya dipakai.
//
// ok=false bila la nil atau format tak dikenali (fail-soft: field opsional,
// lebih baik kosong daripada tanggal salah — dipanggil dari mapDesaPlusSummary
// yang men-skip field ini bila ok=false, bukan menimpa dgn nilai keliru).
func (la *LastActivity) Date() (y int, m time.Month, d int, ok bool) {
	if la == nil {
		return 0, 0, 0, false
	}
	match := timestampPattern.FindStringSubmatch(la.Timestamp)
	if match == nil {
		return 0, 0, 0, false
	}
	day, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, 0, 0, false
	}
	month, known := indonesianMonths[match[2]]
	if !known {
		return 0, 0, 0, false
	}
	year, err := strconv.Atoi(match[3])
	if err != nil {
		return 0, 0, 0, false
	}
	return year, month, day, true
}
