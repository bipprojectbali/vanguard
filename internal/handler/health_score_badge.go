package handler

// health_score_badge.go — pemetaan status/tren skor → label + kelas badge daisyUI
// (healthScoreStatus, healthScoreTrend), dipisah dari health_score_view.go
// (params + row mapper) demi ambang tipe Route/Handler (150). Package sama; murni
// fungsi *string → string, tanpa import.

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
