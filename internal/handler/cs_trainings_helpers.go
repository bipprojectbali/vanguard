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

// parseCSTrainingStatusForm mengurai form mini status-update (dari baris
// tabel). status wajib; attendance & participants opsional (kosong →
// dikosongkan). attendance memakai optNumeric (pola FeatureAdoptionRate,
// customer_success_helpers.go).
func parseCSTrainingStatusForm(fv func(string) string) (status string, attendance pgtype.Numeric, participants *int32, errCode string) {
	status = fv("status")
	if !isValidEnum(status, csTrainingStatusValues) {
		return "", pgtype.Numeric{}, nil, "status"
	}
	att, code := optNumeric(fv("attendance"), "attendance")
	if code != "" {
		return "", pgtype.Numeric{}, nil, code
	}
	if s := fv("participants"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 32); err == nil && n >= 0 {
			p := int32(n)
			participants = &p
		}
	}
	return status, att, participants, ""
}
