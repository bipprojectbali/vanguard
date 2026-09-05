package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_trainings_status_form.go — parsing form aksi status per-baris Training,
// dipisah dari cs_trainings_helpers.go (ukuran file). Perilaku identik; hanya
// organisasi file yang berubah.

// csTrainingStatusForm = data terurai dari form aksi status per-baris.
// Semua field hasil OPSIONAL (kosong → nil/invalid → query COALESCE menjaga
// nilai lama, BL-28 #1). TrainingDate hanya relevan untuk "Jadwal Ulang".
type csTrainingStatusForm struct {
	Status       string
	Attendance   pgtype.Numeric
	Participants *int32
	TrainingDate pgtype.Timestamptz
	Notes        *string
}

// parseCSTrainingStatusForm mengurai form aksi status per-baris. status wajib
// & sahih; attendance/participants/notes/training_date opsional. attendance
// memakai optNumeric (pola FeatureAdoptionRate) lalu divalidasi rentang 0–100.
// training_date WAJIB bila status = rescheduled (inti "jadwal ulang" = tanggal
// baru; tanpa itu aksi jadi no-op, BL-28 #2). Field kosong sengaja TAK menimpa
// nilai lama — penjagaan sesungguhnya di query (COALESCE).
func parseCSTrainingStatusForm(fv func(string) string) (csTrainingStatusForm, string) {
	status := fv("status")
	if !isValidEnum(status, csTrainingStatusValues) {
		return csTrainingStatusForm{}, "status"
	}
	att, code := optNumeric(fv("attendance"), "attendance")
	if code != "" {
		return csTrainingStatusForm{}, code
	}
	if att.Valid {
		if f8, err := att.Float64Value(); err == nil && (f8.Float64 < 0 || f8.Float64 > 100) {
			return csTrainingStatusForm{}, "attendance"
		}
	}
	f := csTrainingStatusForm{Status: status, Attendance: att}
	if s := fv("participants"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 32); err == nil && n >= 0 {
			p := int32(n)
			f.Participants = &p
		}
	}
	td, code := optDateTime(fv("training_date"))
	if code != "" {
		return csTrainingStatusForm{}, "datetime"
	}
	if status == "rescheduled" && !td.Valid {
		return csTrainingStatusForm{}, "required"
	}
	f.TrainingDate = td
	if s := strings.TrimSpace(fv("notes")); s != "" {
		f.Notes = &s
	}
	return f, ""
}
