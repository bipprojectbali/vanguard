package handler

import "go_starter/internal/ui/pages/panel"

// cs_renewals_enum.go — enum sah tahap & risiko CS Renewals (cermin CHECK
// migrasi), tab daftar, validasi, serta pemetaan kode → label & badge. Dipisah
// dari cs_renewals.go agar file di bawah ambang tipe Route/Handler (150). Satu
// paket handler — perilaku identik.

// csRenewalTabs — tab filter stage (sumber tunggal handler + view).
var csRenewalTabs = []panel.CSRenewalTab{
	{Key: "", Label: "Semua"},
	{Key: "Not Started", Label: "Belum Dimulai"},
	{Key: "Outreach", Label: "Outreach"},
	{Key: "Negotiation", Label: "Negosiasi"},
	{Key: "Won", Label: "Won"},
	{Key: "Lost", Label: "Lost"},
}

// csRenewalStageValues — nilai sah stage, dipakai validasi + dropdown.
var csRenewalStageValues = []string{
	"Not Started", "Outreach", "Negotiation", "Won", "Lost",
}

// csRenewalRiskValues — nilai sah risk, dipakai validasi + dropdown.
var csRenewalRiskValues = []string{
	"Low", "Medium", "High",
}

// isValidCSRenewalStage — validasi nilai stage form.
func isValidCSRenewalStage(s string) bool {
	for _, v := range csRenewalStageValues {
		if v == s {
			return true
		}
	}
	return false
}

// isValidCSRenewalRisk — validasi nilai risk form.
func isValidCSRenewalRisk(s string) bool {
	for _, v := range csRenewalRiskValues {
		if v == s {
			return true
		}
	}
	return false
}

// csRenewalStageLabel → label Indonesia dari nilai DB (nil = "—").
func csRenewalStageLabel(s *string) string {
	if s == nil {
		return "—"
	}
	switch *s {
	case "Not Started":
		return "Belum Dimulai"
	case "Outreach":
		return "Outreach"
	case "Negotiation":
		return "Negosiasi"
	case "Won":
		return "Won"
	case "Lost":
		return "Lost"
	default:
		return *s
	}
}

// csRenewalStageBadge → kelas badge daisyUI per stage.
func csRenewalStageBadge(s *string) string {
	if s == nil {
		return "badge-ghost"
	}
	switch *s {
	case "Not Started":
		return "badge-neutral"
	case "Outreach":
		return "badge-info"
	case "Negotiation":
		return "badge-warning"
	case "Won":
		return "badge-success"
	case "Lost":
		return "badge-error"
	default:
		return "badge-ghost"
	}
}

// csRenewalRiskLabel → label Indonesia dari nilai DB (nil = "—").
func csRenewalRiskLabel(s *string) string {
	if s == nil {
		return "—"
	}
	switch *s {
	case "Low":
		return "Rendah"
	case "Medium":
		return "Sedang"
	case "High":
		return "Tinggi"
	default:
		return *s
	}
}

// csRenewalRiskBadge → kelas badge daisyUI per risk level.
func csRenewalRiskBadge(s *string) string {
	if s == nil {
		return "badge-ghost"
	}
	switch *s {
	case "Low":
		return "badge-success"
	case "Medium":
		return "badge-warning"
	case "High":
		return "badge-error"
	default:
		return "badge-ghost"
	}
}
