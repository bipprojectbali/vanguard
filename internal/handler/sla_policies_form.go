package handler

import (
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sla_policies_form.go — parsing & validasi form SLA Policy, dipakai bersama
// create & update. Dipisah dari handler aksi agar aturan validasi (enum,
// angka, wajib) punya SATU tempat: create & edit tak boleh menerima nilai
// yang berbeda sahnya untuk kolom yang sama. Meniru plans_form.go.
//
// applies_to_priority & business_hours TANPA CHECK di DB (kolom teks
// nullable) — divalidasi di handler demi konsistensi pelaporan (dropdown
// menawarkan set tetap, cermin wireframe 6.11). Target respon/selesai satuan
// MENIT (migrasi 00016), int32 nullable via optInt32.

const maxSLANameLen = 200

var (
	// validSLAPriorities = prioritas tiket (cermin wireframe 6.11). Opsional —
	// satu kebijakan bisa berlaku ke satu prioritas spesifik, atau dikosongkan
	// (berlaku umum) sebelum matriks per-prioritas lengkap.
	validSLAPriorities = map[string]struct{}{
		"Kritis": {}, "Tinggi": {}, "Sedang": {}, "Rendah": {},
	}
	// validSLABusinessHours = jam berlaku SLA (cermin wireframe 6.11 & banner
	// "Kritis dihitung 24/7, lainnya jam kerja"). Opsional.
	validSLABusinessHours = map[string]struct{}{
		"Jam kerja": {}, "24/7": {},
	}
)

// slaPolicyForm = nilai form SLA Policy yang SUDAH divalidasi & siap dipetakan
// ke Create/UpdateSLAPolicyParams. Kolom opsional pointer (nil = NULL).
// is_active TAK di sini — pensiun/aktifkan jalur tersendiri.
type slaPolicyForm struct {
	SLAName                    string
	AppliesToPriority          *string
	FirstResponseTargetMinutes *int32
	ResolutionTargetMinutes    *int32
	BusinessHours              *string
	EscalationRule             *string
}

// parseSLAPolicyForm membaca & memvalidasi form. (form, "") bila sah, atau
// (zero, kode) yang dipetakan slaPoliciesErrMsg. Nilai user-controlled →
// validasi backend adalah penjaga sesungguhnya.
func parseSLAPolicyForm(fv func(string) string) (slaPolicyForm, string) {
	var f slaPolicyForm

	f.SLAName = strings.TrimSpace(fv("sla_name"))
	if f.SLAName == "" || len(f.SLAName) > maxSLANameLen {
		return slaPolicyForm{}, "required"
	}

	// applies_to_priority opsional: kosong = NULL; terisi wajib enum.
	if s := strings.TrimSpace(fv("applies_to_priority")); s != "" {
		if _, ok := validSLAPriorities[s]; !ok {
			return slaPolicyForm{}, "priority"
		}
		f.AppliesToPriority = &s
	}

	// business_hours opsional: kosong = NULL; terisi wajib enum.
	if s := strings.TrimSpace(fv("business_hours")); s != "" {
		if _, ok := validSLABusinessHours[s]; !ok {
			return slaPolicyForm{}, "business_hours"
		}
		f.BusinessHours = &s
	}

	// Target respon/selesai (menit): kosong = NULL; terisi wajib bulat ≥0.
	firstResp, code := optInt32(fv("first_response_target_minutes"))
	if code != "" {
		return slaPolicyForm{}, code
	}
	f.FirstResponseTargetMinutes = firstResp

	resolution, code := optInt32(fv("resolution_target_minutes"))
	if code != "" {
		return slaPolicyForm{}, code
	}
	f.ResolutionTargetMinutes = resolution

	// Teks bebas opsional: trim, kosong → NULL.
	f.EscalationRule = optTrim(fv("escalation_rule"))

	return f, ""
}

// slaPriorityOptions / slaBusinessHoursOptions = opsi dropdown BERURUT (map
// validasi tak berurutan). Nilai HARUS himpunan sama dengan map validasi;
// urutan untuk tampilan (mengikuti urutan Kritis→Rendah di wireframe).
var (
	slaPriorityOptions      = []string{"Kritis", "Tinggi", "Sedang", "Rendah"}
	slaBusinessHoursOptions = []string{"Jam kerja", "24/7"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(slaPriorityOptions) != len(validSLAPriorities) ||
		len(slaBusinessHoursOptions) != len(validSLABusinessHours) {
		panic("sla_policies: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// slaPolicyFormFields memetakan SlaPolicy → nilai prefill form (semua string;
// nil → "").
func slaPolicyFormFields(p db.SlaPolicy) panel.SLAPolicyFormFields {
	return panel.SLAPolicyFormFields{
		SLAName:                    p.SlaName,
		AppliesToPriority:          deref(p.AppliesToPriority),
		FirstResponseTargetMinutes: int32Str(p.FirstResponseTargetMinutes),
		ResolutionTargetMinutes:    int32Str(p.ResolutionTargetMinutes),
		BusinessHours:              deref(p.BusinessHours),
		EscalationRule:             deref(p.EscalationRule),
	}
}
