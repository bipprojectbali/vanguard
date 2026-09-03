package handler

import (
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// playbooks_form.go — parsing & validasi form Playbook, dipakai bersama
// create & update. Dipisah dari handler aksi agar aturan validasi (enum,
// wajib) punya SATU tempat: create & edit tak boleh menerima nilai yang
// berbeda sahnya untuk kolom yang sama. Meniru sla_policies_form.go (slice
// A1).
//
// trigger_scenario & recommended_owner TANPA CHECK di DB (kolom teks
// nullable) — divalidasi di handler demi konsistensi pelaporan (dropdown
// menawarkan set tetap, cermin wireframe 6.7).

const maxPlaybookNameLen = 200

var (
	// validPlaybookTriggerScenarios = skenario pemicu (cermin wireframe 6.7).
	// Opsional — playbook bisa dikaitkan ke satu skenario spesifik, atau
	// dikosongkan (belum dikaitkan) sebelum health event (B1) mengonsumsinya.
	validPlaybookTriggerScenarios = map[string]struct{}{
		"Health Drop": {}, "Low Adoption": {}, "Renewal Approaching": {}, "New Onboarding": {},
	}
	// validPlaybookRecommendedOwners = peran yang direkomendasikan menjalankan
	// playbook. Opsional. Diselaraskan ke subset peran BAWAAN yang masuk akal
	// jadi pemilik SOP CS (BL-33, aditif): Manager ditambahkan (menggarap
	// CS/renewal) di samping CSM/Sales/Support lama; Admin dikecualikan
	// (peran platform, bukan pemilik SOP). Field ini murni deskriptif (teks
	// tersimpan, nol dampak fungsional) — bukan FK user; nilai lama tetap
	// valid (nol data-loss).
	validPlaybookRecommendedOwners = map[string]struct{}{
		"Manager": {}, "CSM": {}, "Sales": {}, "Support": {},
	}
)

// playbookForm = nilai form Playbook yang SUDAH divalidasi & siap dipetakan
// ke Create/UpdatePlaybookParams. Kolom opsional pointer (nil = NULL).
// is_active TAK di sini — draf/aktifkan jalur tersendiri.
type playbookForm struct {
	PlaybookName     string
	TriggerScenario  *string
	Description      *string
	Steps            *string
	RecommendedOwner *string
}

// parsePlaybookForm membaca & memvalidasi form. (form, "") bila sah, atau
// (zero, kode) yang dipetakan playbooksErrMsg. Nilai user-controlled →
// validasi backend adalah penjaga sesungguhnya.
func parsePlaybookForm(fv func(string) string) (playbookForm, string) {
	var f playbookForm

	f.PlaybookName = strings.TrimSpace(fv("playbook_name"))
	if f.PlaybookName == "" || len(f.PlaybookName) > maxPlaybookNameLen {
		return playbookForm{}, "required"
	}

	// trigger_scenario opsional: kosong = NULL; terisi wajib enum.
	if s := strings.TrimSpace(fv("trigger_scenario")); s != "" {
		if _, ok := validPlaybookTriggerScenarios[s]; !ok {
			return playbookForm{}, "trigger_scenario"
		}
		f.TriggerScenario = &s
	}

	// recommended_owner opsional: kosong = NULL; terisi wajib enum.
	if s := strings.TrimSpace(fv("recommended_owner")); s != "" {
		if _, ok := validPlaybookRecommendedOwners[s]; !ok {
			return playbookForm{}, "recommended_owner"
		}
		f.RecommendedOwner = &s
	}

	// Teks bebas opsional: trim, kosong → NULL.
	f.Description = optTrim(fv("description"))
	f.Steps = optTrim(fv("steps"))

	return f, ""
}

// playbookTriggerScenarioOptions / playbookRecommendedOwnerOptions = opsi
// dropdown BERURUT (map validasi tak berurutan). Nilai HARUS himpunan sama
// dengan map validasi; urutan untuk tampilan (mengikuti urutan wireframe
// 6.7).
var (
	playbookTriggerScenarioOptions  = []string{"Health Drop", "Low Adoption", "Renewal Approaching", "New Onboarding"}
	playbookRecommendedOwnerOptions = []string{"Manager", "CSM", "Sales", "Support"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda =
// dropdown menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(playbookTriggerScenarioOptions) != len(validPlaybookTriggerScenarios) ||
		len(playbookRecommendedOwnerOptions) != len(validPlaybookRecommendedOwners) {
		panic("playbooks: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// playbookFormFields memetakan Playbook → nilai prefill form (semua string;
// nil → "").
func playbookFormFields(p db.Playbook) panel.PlaybookFormFields {
	return panel.PlaybookFormFields{
		PlaybookName:     p.PlaybookName,
		TriggerScenario:  deref(p.TriggerScenario),
		Description:      deref(p.Description),
		Steps:            deref(p.Steps),
		RecommendedOwner: deref(p.RecommendedOwner),
	}
}
