package handler

import (
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_impl_tasks_helpers.go — parsing form, enum, label, dan pemetaan DB→view
// untuk Implementation Tracker (Modul 6, sub-item Onboarding 6.2.1.1). Murni
// Go (tanpa HTTP/DB call) agar bisa diuji langsung. Meniru engagements_helpers.go.

// csImplTaskStatusValues = nilai sahih task_status; CERMIN persis CHECK
// constraint migrasi 00024. Urutan = urutan dropdown/tab.
var csImplTaskStatusValues = []string{
	"to_do", "in_progress", "done", "blocked",
}

// csImplTaskStatusLabel memetakan kode DB → label UI + badge class daisyUI.
func csImplTaskStatusLabel(status string) (label, badge string) {
	switch status {
	case "to_do":
		return "To Do", "badge-ghost"
	case "in_progress":
		return "In Progress", "badge-info"
	case "done":
		return "Done", "badge-success"
	case "blocked":
		return "Blocked", "badge-error"
	default:
		return "—", "badge-ghost"
	}
}

// csImplTaskDueDateLabel memformat due_date untuk tampilan tabel.
func csImplTaskDueDateLabel(d pgtype.Date) string {
	if !d.Valid {
		return "—"
	}
	return d.Time.Format("2 Jan 2006")
}

// csImplTaskForm = data terurai dari form POST task baru.
type csImplTaskForm struct {
	AccountID int64
	TaskName  string
	OwnerID   *int64
	DueDate   pgtype.Date
}

// parseCSImplTaskForm terurai dan memvalidasi form task baru. Mengembalikan
// (form, "") bila OK; (zero, errCode) bila validasi gagal.
func parseCSImplTaskForm(fv func(string) string) (csImplTaskForm, string) {
	accountIDStr := fv("account_id")
	if accountIDStr == "" {
		return csImplTaskForm{}, "required"
	}
	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		return csImplTaskForm{}, "account"
	}

	taskName := strings.TrimSpace(fv("task_name"))
	if taskName == "" {
		return csImplTaskForm{}, "required"
	}

	f := csImplTaskForm{AccountID: accountID, TaskName: taskName}

	dueDate, code := optDate(fv("due_date"))
	if code != "" {
		return csImplTaskForm{}, "required"
	}
	f.DueDate = dueDate

	if s := fv("owner_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			f.OwnerID = &id
		}
	}

	return f, ""
}

// parseCSImplTaskStatusForm mengurai form mini status-update (dari baris
// tabel). Hanya status yang wajib; due_date & owner_id opsional (kosong →
// dikosongkan, sama pola parseEngagementStatusForm).
func parseCSImplTaskStatusForm(fv func(string) string) (status string, dueDate pgtype.Date, ownerID *int64, errCode string) {
	status = fv("status")
	if !isValidEnum(status, csImplTaskStatusValues) {
		return "", pgtype.Date{}, nil, "status"
	}
	dd, code := optDate(fv("due_date"))
	if code != "" {
		return "", pgtype.Date{}, nil, "required"
	}
	if s := fv("owner_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			ownerID = &id
		}
	}
	return status, dd, ownerID, ""
}

// csImplTaskRowView memetakan satu baris ListCSImplTasks → CSImplTaskRow view.
func csImplTaskRowView(r db.ListCSImplTasksRow, slug string) panel.CSImplTaskRow {
	statusLabel, statusBadge := csImplTaskStatusLabel(r.TaskStatus)
	ownerName := "—"
	if r.OwnerName != nil && *r.OwnerName != "" {
		ownerName = *r.OwnerName
	}
	return panel.CSImplTaskRow{
		ID:          r.ID,
		AccountName: r.AccountName,
		TaskName:    r.TaskName,
		StatusLabel: statusLabel,
		StatusBadge: statusBadge,
		OwnerName:   ownerName,
		DueDate:     csImplTaskDueDateLabel(r.DueDate),
		HrefDetail:  wsPath(slug, "/impl-tasks/"+strconv.FormatInt(r.ID, 10)),
	}
}

// csImplTaskKPIView memetakan CountCSImplTaskKPIs → CSImplTaskKPIs view.
func csImplTaskKPIView(k db.CountCSImplTaskKPIsRow) panel.CSImplTaskKPIs {
	return panel.CSImplTaskKPIs{
		Total:      int(k.TotalCount),
		ToDo:       int(k.ToDoCount),
		InProgress: int(k.InProgressCount),
		Done:       int(k.DoneCount),
		Blocked:    int(k.BlockedCount),
	}
}
