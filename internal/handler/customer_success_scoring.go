package handler

import (
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_scoring.go — skoring & derivasi CS (computeOverallHealthScore,
// ambang health, deriveHealthStatus, deriveScoreTrend, daysInStageLabel), dipisah
// dari customer_success_view.go (pemetaan ke view) agar keduanya di bawah ambang
// tipe Route/Handler (150). Fungsi murni: dipanggil handler save & mapper view.

// computeOverallHealthScore = rata-rata KOMPONEN non-NULL (adoption/engagement/
// support/sentiment), dibulatkan. nil bila SEMUA komponen kosong (belum ada
// dasar hitung) — kolom tetap NULL, bukan 0 yang menyesatkan (0 berarti "skor
// terendah", bukan "belum dihitung"). Dipanggil handler SAJA (customer_success_
// save.go); form tak pernah mengirim overall_health_score sendiri (lihat komentar
// migrasi 00019: kolom disengaja app-computed, bukan generated, agar rumus bisa
// berubah tanpa DDL).
func computeOverallHealthScore(adoption, engagement, support, sentiment *int16) *int16 {
	var sum, n int
	for _, p := range []*int16{adoption, engagement, support, sentiment} {
		if p != nil {
			sum += int(*p)
			n++
		}
	}
	if n == 0 {
		return nil
	}
	avg := int16((sum + n/2) / n) // pembulatan biasa, bukan selalu ke bawah
	return &avg
}

// Ambang status kesehatan — CERMIN persis label kartu KPI di halaman Health
// Score (health_score_list.go: "Sehat (≥80)" / "Berisiko (40–79)" / "Kritis
// (<40)"). Konstanta bernama (bukan angka telanjang) agar ambang punya SATU
// sumber; ubah di sini bila kelak label KPI ikut berubah.
const (
	healthHealthyMin = 80 // skor ≥ ini → Healthy
	healthAtRiskMin  = 40 // skor ≥ ini (dan < healthHealthyMin) → At-Risk; di bawah → Critical

	// Dead-band tren skor (BL-25): |delta| ≤ ini → "Stable". Ambang bisnis (rule
	// §15, keputusan v1 = 3 poin) — perubahan lebih kecil dianggap noise, bukan
	// arah. delta > +T → Improving, delta < −T → Declining, selain itu Stable.
	healthTrendDeadband = 3
)

// deriveHealthStatus memetakan overall_health_score → status kesehatan
// (Healthy/At-Risk/Critical) MENGIKUTI ambang label KPI. Status kini TURUNAN
// skor, bukan pilihan manual operator (BL-24) — satu sumber kebenaran. nil
// (belum ada dasar hitung, semua komponen kosong) → nil: JANGAN default ke
// status apa pun, "belum dinilai" bukan "sehat". Nilai kembali tetap dalam
// 3-set enum sah (cs_health_status_chk), jadi tanpa DDL.
func deriveHealthStatus(overall *int16) *string {
	if overall == nil {
		return nil
	}
	var s string
	switch {
	case *overall >= healthHealthyMin:
		s = "Healthy"
	case *overall >= healthAtRiskMin:
		s = "At-Risk"
	default:
		s = "Critical"
	}
	return &s
}

// deriveScoreTrend menurunkan arah pergerakan skor kesehatan dari skor LAMA ke
// skor SEKARANG (BL-25): tren kini turunan riwayat skor, bukan pilihan manual
// operator — satu sumber kebenaran. prev/curr nil (belum ada pembanding: snapshot
// pertama atau semua komponen kosong) → nil ("—", belum ada dasar), JANGAN
// default "Stable". deadband = ambang |delta| di bawah mana dianggap Stable.
// Nilai kembali tetap dalam 3-set enum sah (cs_score_trend_chk), jadi tanpa DDL.
func deriveScoreTrend(prev, curr *int16, deadband int) *string {
	if prev == nil || curr == nil {
		return nil
	}
	delta := int(*curr) - int(*prev)
	var s string
	switch {
	case delta > deadband:
		s = "Improving"
	case delta < -deadband:
		s = "Declining"
	default:
		s = "Stable"
	}
	return &s
}

// daysInStageLabel = selisih hari (kalender) HARI INI terhadap stage_entry_date,
// diformat "N hari di tahap ini". Kebalikan arah daysLeftLabel (subscriptions_
// renewals.go: hitung SISA hari ke depan) — di sini menghitung MUNDUR sejak
// masuk tahap. Kedua tanggal dinormalkan ke tanggal sipil (UTC midnight) agar
// bebas jam/zona.
func daysInStageLabel(now time.Time, entry pgtype.Date) string {
	if !entry.Valid {
		return "—"
	}
	a := time.Date(entry.Time.Year(), entry.Time.Month(), entry.Time.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	d := int(b.Sub(a).Hours() / 24)
	if d < 0 {
		d = 0 // stage_entry_date di masa depan (data janggal) — tampilkan 0, bukan negatif
	}
	return strconv.Itoa(d) + " hari di tahap ini"
}
