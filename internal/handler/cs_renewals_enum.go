package handler

import "go_starter/internal/ui/pages/panel"

// cs_renewals_enum.go — enum sah tahap & risiko CS Renewals (cermin CHECK
// migrasi), tab daftar, validasi, serta pemetaan kode → label & badge. Dipisah
// dari cs_renewals.go agar file di bawah ambang tipe Route/Handler (150). Satu
// paket handler — perilaku identik.

// csRenewalTabs — tab filter stage (sumber tunggal handler + view).
//
// Label "Won"/"Lost" sengaja ditampilkan sbg "Renewed"/"Terminate" (diskusi
// user 2026-09-25): "Won/Lost" jargon deal Sales Pipeline, kurang pas utk
// retensi pelanggan existing. Label ONLY — nilai DB tetap "Won"/"Lost"
// (cermin CHECK subs_renewal_stage_chk, migrasi 00012) agar tanpa migrasi +
// tanpa backfill data lama.
var csRenewalTabs = []panel.CSRenewalTab{
	{Key: "", Label: "Semua"},
	{Key: "Not Started", Label: "Belum Dimulai"},
	{Key: "Outreach", Label: "Outreach"},
	{Key: "Negotiation", Label: "Negosiasi"},
	{Key: "Won", Label: "Renewed"},
	{Key: "Lost", Label: "Terminate"},
}

// csRenewalStageValues — nilai sah stage, dipakai validasi.
var csRenewalStageValues = []string{
	"Not Started", "Outreach", "Negotiation", "Won", "Lost",
}

// csRenewalStageOptions — pilihan dropdown form edit (Value dari DB, Label
// tampilan via csRenewalStageLabel — lihat catatan csRenewalTabs).
var csRenewalStageOptions = buildCSRenewalStageOptions()

func buildCSRenewalStageOptions() []panel.CSRenewalStageOption {
	opts := make([]panel.CSRenewalStageOption, 0, len(csRenewalStageValues))
	for _, v := range csRenewalStageValues {
		val := v
		opts = append(opts, panel.CSRenewalStageOption{Value: val, Label: csRenewalStageLabel(&val)})
	}
	return opts
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
		return "Renewed"
	case "Lost":
		return "Terminate"
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
