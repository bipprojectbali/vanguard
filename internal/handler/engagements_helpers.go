package handler

import (
	"strconv"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// engagements_helpers.go — parsing form, enum, label, dan pemetaan DB→view untuk
// Engagements / Check-ins (Modul 6 slice 6.5). Murni Go (tanpa HTTP/DB call)
// agar bisa diuji langsung. Meniru tickets_helpers.go / sales_format.go.

// Enum values — CERMIN persis CHECK constraint migrasi 00022.

// engagementTypeValues = nilai sahih engagement_type; urutan = urutan dropdown.
var engagementTypeValues = []string{
	"touch_point", "qbr", "onboarding_call", "escalation", "check_in",
}

// engagementFrequencyValues = nilai sahih frequency; opsional (nil = ad-hoc / tak diisi).
var engagementFrequencyValues = []string{
	"weekly", "monthly", "quarterly", "ad_hoc",
}

// engagementStatusValues = nilai sahih status lifecycle engagement.
var engagementStatusValues = []string{
	"planned", "done", "skipped", "rescheduled",
}

// engagementChannelValues = nilai sahih channel komunikasi.
var engagementChannelValues = []string{
	"whatsapp", "call", "video", "site_visit",
}

// engagementTypeLabel memetakan kode DB → label UI.
func engagementTypeLabel(t string) string {
	switch t {
	case "touch_point":
		return "Touch Point"
	case "qbr":
		return "QBR"
	case "onboarding_call":
		return "Onboarding Call"
	case "escalation":
		return "Escalation"
	case "check_in":
		return "Check-in"
	default:
		return t
	}
}

// engagementStatusLabel memetakan kode DB → label UI + badge class daisyUI.
func engagementStatusLabel(status string) (label, badge string) {
	switch status {
	case "planned":
		return "Planned", "badge-info"
	case "done":
		return "Done", "badge-success"
	case "skipped":
		return "Skipped", "badge-ghost"
	case "rescheduled":
		return "Rescheduled", "badge-warning"
	default:
		return "—", "badge-ghost"
	}
}

// engagementChannelLabel memetakan kode DB → label UI.
func engagementChannelLabel(ch *string) string {
	if ch == nil {
		return "—"
	}
	switch *ch {
	case "whatsapp":
		return "WhatsApp"
	case "call":
		return "Call"
	case "video":
		return "Video"
	case "site_visit":
		return "Site Visit"
	default:
		return *ch
	}
}

// engagementScheduledLabel memformat scheduled_at untuk kolom tabel.
// Format: "2 Jan 2006 15:04" di timezone workspace.
func engagementScheduledLabel(ts pgtype.Timestamptz, tz *time.Location) string {
	if !ts.Valid {
		return "—"
	}
	if tz == nil {
		tz = time.UTC
	}
	return ts.Time.In(tz).Format("2 Jan 2006 15:04")
}

// engagementNextDueLabel memformat next_due_date untuk tampilan.
func engagementNextDueLabel(d pgtype.Date) string {
	if !d.Valid {
		return "—"
	}
	return d.Time.Format("2 Jan 2006")
}

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

// engagementRowView memetakan satu baris ListEngagements → EngagementRow view.
func engagementRowView(r db.ListEngagementsRow, slug string, tz *time.Location) panel.EngagementRow {
	statusLabel, statusBadge := engagementStatusLabel(r.Status)
	ownerName := "—"
	if r.OwnerName != nil && *r.OwnerName != "" {
		ownerName = *r.OwnerName
	}
	return panel.EngagementRow{
		ID:          r.ID,
		AccountName: r.AccountName,
		Subject:     r.Subject,
		TypeLabel:   engagementTypeLabel(r.EngagementType),
		Channel:     engagementChannelLabel(r.Channel),
		Scheduled:   engagementScheduledLabel(r.ScheduledAt, tz),
		StatusLabel: statusLabel,
		StatusBadge: statusBadge,
		OwnerName:   ownerName,
		NextDue:     engagementNextDueLabel(r.NextDueDate),
		HrefDetail:  wsPath(slug, "/engagements/"+strconv.FormatInt(r.ID, 10)),
	}
}

// engagementKPIView memetakan CountEngagementKPIs → EngagementKPIs view.
func engagementKPIView(k db.CountEngagementKPIsRow) panel.EngagementKPIs {
	return panel.EngagementKPIs{
		Total:    int(k.TotalCount),
		Planned:  int(k.PlannedCount),
		Done:     int(k.DoneCount),
		Missed:   int(k.MissedCount),
		DueSoon:  int(k.DueSoonCount),
	}
}
