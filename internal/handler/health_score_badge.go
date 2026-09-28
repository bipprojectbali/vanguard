package handler

// health_score_badge.go — pemetaan status/tren skor → label + kelas badge daisyUI
// (healthScoreStatus, healthScoreTrendIcon), dipisah dari health_score_view.go
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

// healthScoreTrendIcon mengembalikan ikon panah tren + kelas warna semantik
// daisyUI, ditempel ke value kolom Skor (kolom "Tren" terpisah dibuang — ikon
// cukup mewakili arah tanpa makan lebar tabel). nil (belum ada pembanding) →
// ikon kosong, JANGAN tampilkan panah netral yang menyesatkan.
func healthScoreTrendIcon(trend *string) (icon, class string) {
	if trend == nil {
		return "", ""
	}
	switch *trend {
	case "Improving":
		return "↑", "text-success"
	case "Stable":
		return "→", "text-base-content/40"
	case "Declining":
		return "↓", "text-error"
	default:
		return "", ""
	}
}
