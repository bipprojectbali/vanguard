package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_trainings_helpers.go — parsing form, enum, label, dan pemetaan DB→view
// untuk Training Schedule (Modul 6, sub-item Onboarding 6.2.1.2). Murni Go
// (tanpa HTTP/DB call) agar bisa diuji langsung. Meniru cs_impl_tasks_helpers.go
// dan engagements_helpers.go (khusus attendance: pola optNumeric/numericStr
// dari sales_format.go/accounts_view.go, sudah dipakai FeatureAdoptionRate).

// csTrainingStatusValues = nilai sahih training_status; CERMIN persis CHECK
// constraint migrasi 00025. Urutan = urutan dropdown/tab.
var csTrainingStatusValues = []string{
	"scheduled", "completed", "rescheduled", "cancelled",
}

// csTrainingStatusLabel memetakan kode DB → label UI + badge class daisyUI.
func csTrainingStatusLabel(status string) (label, badge string) {
	switch status {
	case "scheduled":
		return "Scheduled", "badge-info"
	case "completed":
		return "Completed", "badge-success"
	case "rescheduled":
		return "Rescheduled", "badge-warning"
	case "cancelled":
		return "Cancelled", "badge-error"
	default:
		return "—", "badge-ghost"
	}
}

// csTrainingDateLabel memformat training_date (NOT NULL) untuk tampilan
// tabel, memakai appTZ (zona waktu aplikasi) — sama pola engagementScheduledLabel.
func csTrainingDateLabel(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return "—"
	}
	return ts.Time.In(appTZ).Format("2 Jan 2006 15:04")
}

// csTrainingForm = data terurai dari form POST training baru.
type csTrainingForm struct {
	AccountID     int64
	TrainingTopic string
	TrainingDate  pgtype.Timestamptz
	TrainerID     *int64
	Participants  *int32
}

// parseCSTrainingForm terurai dan memvalidasi form training baru. Berbeda
// dari parseCSImplTaskForm: training_date WAJIB diisi (kolom NOT NULL di DB,
// jadwal training harus pasti saat dibuat) — pola sama parseEngagementForm
// utk scheduled_at. Mengembalikan (form, "") bila OK; (zero, errCode) bila gagal.
func parseCSTrainingForm(fv func(string) string) (csTrainingForm, string) {
	accountIDStr := fv("account_id")
	if accountIDStr == "" {
		return csTrainingForm{}, "required"
	}
	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		return csTrainingForm{}, "account"
	}

	topic := strings.TrimSpace(fv("training_topic"))
	if topic == "" {
		return csTrainingForm{}, "required"
	}

	dateStr := fv("training_date")
	if dateStr == "" {
		return csTrainingForm{}, "required"
	}
	trainingDate, code := optDateTime(dateStr)
	if code != "" {
		return csTrainingForm{}, "required"
	}

	f := csTrainingForm{AccountID: accountID, TrainingTopic: topic, TrainingDate: trainingDate}

	if s := fv("trainer_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			f.TrainerID = &id
		}
	}
	if s := fv("participants"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 32); err == nil && n >= 0 {
			p := int32(n)
			f.Participants = &p
		}
	}

	return f, ""
}

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
