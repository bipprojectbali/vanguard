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

// activeCSRenewalStages = tahap aktif (bukan terminal) — 3 elemen pertama
// csRenewalStageValues. Basis urutan maju state machine (BL-176, mirror
// activeDealStages di sales_deals_stage_sequence.go, BL-159).
var activeCSRenewalStages = csRenewalStageValues[:len(csRenewalStageValues)-2]

// normalizeCSRenewalStage: "" (belum pernah diisi) DIPERLAKUKAN sebagai alias
// "Not Started" HANYA untuk keperluan lookup transisi — tak pernah ditulis
// balik ke DB sebagai "Not Started" (field tetap boleh NULL/"").
func normalizeCSRenewalStage(s string) string {
	if s == "" {
		return "Not Started"
	}
	return s
}

// nextCSRenewalStages = tahap SAH berikutnya dari current (BL-176, mirror
// nextDealStages). Tahap aktif biasa → {tahap berikut, "Lost"} (bisa gugur di
// tahap mana pun, bukan cuma di ujung); tahap aktif TERAKHIR (Negotiation) →
// {"Won", "Lost"}. current terminal atau tak dikenal → nil (tak ada tahap
// lanjutan sah).
func nextCSRenewalStages(current string) []string {
	current = normalizeCSRenewalStage(current)
	for i, s := range activeCSRenewalStages {
		if s != current {
			continue
		}
		if i == len(activeCSRenewalStages)-1 {
			return []string{"Won", "Lost"}
		}
		return []string{activeCSRenewalStages[i+1], "Lost"}
	}
	return nil
}

// isCSRenewalStageTerminal — Won/Lost = tak bisa diubah lagi. Dipakai
// CSRenewalUpdate mengunci SELURUH form (bukan cuma field stage) begitu
// renewal sudah tuntas — periode berikutnya lahir sbg baris subscriptions
// BARU (previous_subscription_id), mulai lagi dari kosong (Not Started).
func isCSRenewalStageTerminal(stage string) bool {
	s := normalizeCSRenewalStage(stage)
	return s == "Won" || s == "Lost"
}

// isValidCSRenewalStageTransition menegakkan urutan DI BACKEND (jaring
// terakhir, dropdown UI SEKARANG turut dibatasi via csRenewalStageOptionsFor —
// keduanya harus tetap selaras bila urutan berubah). current==next (termasuk
// alias "" ≡ "Not Started") SELALU sah — no-op, perlu krn form edit re-submit
// stage saat ini walau cuma field lain (risk/action_plan) yang diubah.
func isValidCSRenewalStageTransition(current, next string) bool {
	cur, nxt := normalizeCSRenewalStage(current), normalizeCSRenewalStage(next)
	if cur == nxt {
		return true
	}
	if isCSRenewalStageTerminal(cur) {
		return false
	}
	for _, s := range nextCSRenewalStages(cur) {
		if s == nxt {
			return true
		}
	}
	return false
}

// csRenewalStageOptionsFor — pilihan dropdown form edit DIBATASI (BL-176
// lanjutan, diskusi user 2026-09-28): stage SAAT INI (agar submit ulang stage
// sama tetap sah sbg opsi ter-render — form edit selalu re-submit renewal_stage
// walau cuma field lain yang diubah) + HANYA tahap SAH berikutnya
// (nextCSRenewalStages), bukan lagi seluruh 5 nilai tanpa peduli current
// (beda dari dealStageControl BL-159 yang justru MENGECUALIKAN stage
// sekarang — di sana ganti-tahap form terpisah, tak ada re-submit nilai sama).
// current terminal (Won/Lost) → nextCSRenewalStages nil → HANYA current
// sendiri tampil (selaras form terkunci total di backend, CSRenewalUpdate).
// current=="" (belum pernah diisi) → tak ditambah eksplisit di sini; opsi
// "— Pilih Stage —" placeholder (csRenewalStageField) yang mewakilinya.
func csRenewalStageOptionsFor(current string) []panel.CSRenewalStageOption {
	seen := map[string]bool{}
	opts := make([]panel.CSRenewalStageOption, 0, 3)
	add := func(v string) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		val := v
		opts = append(opts, panel.CSRenewalStageOption{Value: val, Label: csRenewalStageLabel(&val)})
	}
	add(current)
	for _, s := range nextCSRenewalStages(current) {
		add(s)
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
