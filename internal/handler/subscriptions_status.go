package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_status.go — derivasi kolom "Paket" & "Masa Berlaku"/Status
// daftar Subscription Lists (BL-95, band BL-151), dipakai subRowView di
// subscriptions_page.go.

// subPlanDisplay merender kolom "Paket" daftar langganan (BL-88 PR2b): >1 item →
// "N paket" (langganan multi-paket, parent plan_name NULL); 1 item → nama paket
// (single-plan mengisi plan_name parent lewat JOIN); tanpa plan_name & ≤1 item
// (paket terhapus / langganan lama tanpa item) → "—". itemCount dari subquery
// COUNT(subscription_items) di query daftar.
func subPlanDisplay(planName *string, itemCount int64) string {
	if itemCount > 1 {
		return strconv.FormatInt(itemCount, 10) + " paket"
	}
	if planName != nil && *planName != "" {
		return *planName
	}
	return "—"
}

// Ambang band derivasi kolom "Masa Berlaku" daftar langganan (BL-151, kolom
// KHUSUS — SENGAJA TERPISAH dari dueSoonDays=30 yang dipakai jendela Renewals).
// Dulu satu ambang 30 hari bikin langganan Monthly (termin 30 hari) langsung
// "Jatuh Tempo" sejak lahir; kini gradasi 5 tingkat, "Jatuh Tempo" dipersempit
// ke tepat hari-H (d==0). JANGAN reuse dueSoonDays di subDerivedStatus.
const (
	dueSoonMaxDays   = 7  // 1..7   → "Segera Jatuh Tempo"
	attentionMaxDays = 14 // 8..14  → "Perlu Perhatian"
)

// subDerivedStatus = kolom "Masa Berlaku" daftar langganan (BL-95, band BL-151).
// Untuk langganan Active, DERIVASI dari sisa hari end_date dgn 5 band gradasi:
//   - d < 0             → "Masa Tenggang"       (error)          — sudah lewat tempo
//   - d == 0            → "Jatuh Tempo"         (warning)        — tepat hari-H
//   - 1 ≤ d ≤ 7         → "Segera Jatuh Tempo"  (warning outline)
//   - 8 ≤ d ≤ 14        → "Perlu Perhatian"     (info)
//   - d ≥ 15            → "Aman"                (success)
//
// Status daur hidup NON-Active/PendingApproval (Trial/Expired/Cancelled/Churned)
// dikembalikan apa adanya dgn badge lifecycle (subStatusLifecycleClass) — derivasi
// timing renewal tak bermakna untuk status terminal (mis. Cancelled ber-end_date
// lampau ≠ "Masa Tenggang"). PendingApproval DIPERLAKUKAN SAMA seperti Active
// (BL-175, keputusan user 28 Sep) — upsell menunggu approval tetap ber-end_date
// aktif, baris "Akan Jatuh Tempo"/"Diperpanjang" Renewals mencakup keduanya
// (renewalDerivedStatus), jadi band urgensi tetap bermakna, bukan cuma label
// lifecycle mentah. Mengembalikan label + class badge daisyUI (token semantik).
// Ambang di sini TERPISAH dari dueSoonDays (jendela Renewals, 30 hari).
func subDerivedStatus(status string, end pgtype.Date, now time.Time) (label, badgeClass string) {
	if status != "Active" && status != "PendingApproval" {
		return status, subStatusLifecycleClass(status)
	}
	if !end.Valid {
		return "Aman", "badge badge-success"
	}
	a := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Time.Year(), end.Time.Month(), end.Time.Day(), 0, 0, 0, 0, time.UTC)
	d := int(b.Sub(a).Hours() / 24)
	switch {
	case d < 0:
		return "Masa Tenggang", "badge badge-error"
	case d == 0:
		return "Jatuh Tempo", "badge badge-warning"
	case d <= dueSoonMaxDays:
		return "Segera Jatuh Tempo", "badge badge-warning badge-outline"
	case d <= attentionMaxDays:
		return "Perlu Perhatian", "badge badge-info"
	default:
		return "Aman", "badge badge-success"
	}
}

// bandTextClass mengonversi badgeClass (token daisyUI, hasil subDerivedStatus)
// jadi kelas WARNA TEKS Tailwind polos (BL-175, kolom Sisa Hari Renewals —
// TANPA badge/label, cuma warna angka). Cocokkan token "warning"/"error"/dst
// SEBELUM cek badge-outline dsb — urutan tak masalah karena substring unik per
// warna. badge-ghost/tanpa warna → "" (teks default, tak diwarnai).
func bandTextClass(badgeClass string) string {
	switch {
	case strings.Contains(badgeClass, "badge-error"):
		return "text-error"
	case strings.Contains(badgeClass, "badge-warning"):
		return "text-warning"
	case strings.Contains(badgeClass, "badge-info"):
		return "text-info"
	case strings.Contains(badgeClass, "badge-success"):
		return "text-success"
	default:
		return ""
	}
}

// subStatusLifecycleClass = badge daisyUI (token semantik) untuk status daur hidup
// langganan NON-Active pada kolom Status daftar (BL-95). Dipindah dari
// panel.subStatusBadge saat Status jadi derivasi: view kini menerima class jadi
// dari handler. Active tak lewat sini (di-derivasi subDerivedStatus).
func subStatusLifecycleClass(status string) string {
	switch status {
	case "Trial":
		return "badge badge-info"
	case "Suspended", "PendingApproval":
		return "badge badge-warning"
	case "Expired", "Cancelled", "Churned":
		return "badge badge-error"
	default:
		return "badge badge-ghost"
	}
}
