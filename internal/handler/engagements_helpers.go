package handler

import (
	"strconv"
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

// engagementRowView memetakan satu baris ListEngagements → EngagementRow view.
func engagementRowView(r db.ListEngagementsRow, slug string, tz *time.Location) panel.EngagementRow {
	statusLabel, statusBadge := engagementStatusLabel(r.Status)
	ownerName := "—"
	if r.OwnerName != nil && *r.OwnerName != "" {
		ownerName = *r.OwnerName
	}
	outcome := ""
	if r.Outcome != nil {
		outcome = *r.Outcome
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
		Outcome:     outcome,
		HrefDetail:  wsPath(slug, "/engagements/"+strconv.FormatInt(r.ID, 10)),
	}
}

// engagementKPIView memetakan CountEngagementKPIs → EngagementKPIs view.
func engagementKPIView(k db.CountEngagementKPIsRow) panel.EngagementKPIs {
	return panel.EngagementKPIs{
		Total:   int(k.TotalCount),
		Planned: int(k.PlannedCount),
		Done:    int(k.DoneCount),
		Missed:  int(k.MissedCount),
		DueSoon: int(k.DueSoonCount),
	}
}
