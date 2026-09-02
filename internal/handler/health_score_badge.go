package handler

// health_score_badge.go — pemetaan status/tren skor → label + kelas badge daisyUI
// (healthScoreStatus, healthScoreTrend, scoreTrendBadge), dipisah dari
// health_score_view.go (params + row mapper) demi ambang tipe Route/Handler (150).
// Package sama; murni fungsi *string → string, tanpa import.

// healthScoreStatus mengembalikan label + kelas badge dari health_status.
func healthScoreStatus(status *string) (label, badge string) {
	if status == nil {
		return "—", "badge-ghost"
	}
	switch *status {
	case "Healthy":
		return "Sehat", "badge-success"
	case "At-Risk":
		return "Berisiko", "badge-warning"
	case "Critical":
		return "Kritis", "badge-error"
	default:
		return *status, "badge-ghost"
	}
}

// healthScoreTrend mengembalikan label tren singkat.
func healthScoreTrend(trend *string) string {
	if trend == nil {
		return "—"
	}
	switch *trend {
	case "Improving":
		return "↑ Naik"
	case "Stable":
		return "→ Stabil"
	case "Declining":
		return "↓ Turun"
	default:
		return *trend
	}
}

// scoreTrendBadge → kelas badge daisyUI untuk tren skor (BL-25, badge read-only
// di form CS). Selaras arah: Improving=success, Declining=error, Stable/nil=ghost
// (netral, "belum ada dasar" tak diberi warna menyesatkan). Label dari
// healthScoreTrend (satu sumber panah/teks).
func scoreTrendBadge(trend *string) string {
	if trend == nil {
		return "badge-ghost"
	}
	switch *trend {
	case "Improving":
		return "badge-success"
	case "Declining":
		return "badge-error"
	default:
		return "badge-ghost"
	}
}
