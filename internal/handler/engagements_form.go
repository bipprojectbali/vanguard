package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// engagements_form.go — tipe form engagement tervalidasi & parsing form (create
// + update status). Dipisah dari label, enum, & pemetaan view di
// engagements_helpers.go agar file di bawah ambang tipe Route/Handler (150).
// Satu paket handler — perilaku identik.

// engagementForm = data terurai dari form POST engagement baru/update.
type engagementForm struct {
	AccountID      int64
	Subject        string
	EngagementType string
	Frequency      *string
	ScheduledAt    pgtype.Timestamptz
	Status         string
	Channel        *string
	Outcome        *string
	NextDueDate    pgtype.Date
	OwnerID        *int64
}

// parseEngagementForm terurai dan memvalidasi form engagement baru. Mengembalikan
// (form, "") bila OK; (zero, errCode) bila validasi gagal.
// scheduled_at WAJIB (berbeda dari optDateTime yang opsional); sisanya opsional.
func parseEngagementForm(fv func(string) string) (engagementForm, string) {
	accountIDStr := fv("account_id")
	if accountIDStr == "" {
		return engagementForm{}, "required"
	}
	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		return engagementForm{}, "account"
	}

	subject := strings.TrimSpace(fv("subject"))
	if subject == "" {
		return engagementForm{}, "required"
	}

	engType := fv("engagement_type")
	if !isValidEnum(engType, engagementTypeValues) {
		return engagementForm{}, "type"
	}

	scheduledRaw := fv("scheduled_at")
	if strings.TrimSpace(scheduledRaw) == "" {
		return engagementForm{}, "required"
	}
	scheduledAt, code := optDateTime(scheduledRaw)
	if code != "" {
		return engagementForm{}, "required"
	}

	f := engagementForm{
		AccountID:      accountID,
		Subject:        subject,
		EngagementType: engType,
		ScheduledAt:    scheduledAt,
		Status:         "planned",
	}

	// Opsional: frequency
	if freq := fv("frequency"); freq != "" {
		if !isValidEnum(freq, engagementFrequencyValues) {
			return engagementForm{}, "required"
		}
		f.Frequency = &freq
	}

	// Opsional: channel
	if ch := fv("channel"); ch != "" {
		if !isValidEnum(ch, engagementChannelValues) {
			return engagementForm{}, "required"
		}
		f.Channel = &ch
	}

	// Opsional: outcome (teks panjang)
	if outcome := strings.TrimSpace(fv("outcome")); outcome != "" {
		f.Outcome = &outcome
	}

	// Opsional: next_due_date
	nextDue, nextCode := optDate(fv("next_due_date"))
	if nextCode != "" {
		return engagementForm{}, "required"
	}
	f.NextDueDate = nextDue

	// Opsional: owner_id
	if s := fv("owner_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			f.OwnerID = &id
		}
	}

	return f, ""
}

// parseEngagementStatusForm mengurai form mini status-update (dari baris tabel).
// Hanya status yang wajib; outcome & next_due_date opsional.
func parseEngagementStatusForm(fv func(string) string) (status string, outcome *string, nextDue pgtype.Date, errCode string) {
	status = fv("status")
	if !isValidEnum(status, engagementStatusValues) {
		return "", nil, pgtype.Date{}, "status"
	}
	if s := strings.TrimSpace(fv("outcome")); s != "" {
		outcome = &s
	}
	nd, code := optDate(fv("next_due_date"))
	if code != "" {
		return "", nil, pgtype.Date{}, "required"
	}
	return status, outcome, nd, ""
}
