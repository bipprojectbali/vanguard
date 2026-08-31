package handler

import (
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// cs_trainings_panelview.go — pemetaan baris/KPI DB → view-model panel untuk
// Training Schedule (Modul 6). Dipisah dari cs_trainings_helpers.go (enum,
// label, parsing form) agar tiap file di bawah ambang Route/Handler (150).
// View murni-data: tak panggil authz/session; flag sudah diprecompute di handler.
// csTrainingRowView memetakan satu baris ListCSTrainings → CSTrainingRow view.
func csTrainingRowView(r db.ListCSTrainingsRow, slug string) panel.CSTrainingRow {
	statusLabel, statusBadge := csTrainingStatusLabel(r.TrainingStatus)
	trainerName := "—"
	if r.TrainerName != nil && *r.TrainerName != "" {
		trainerName = *r.TrainerName
	}
	participants := "—"
	if r.Participants != nil {
		participants = strconv.FormatInt(int64(*r.Participants), 10)
	}
	_ = slug // href detail reserved, sama pola cs_impl_tasks (belum ada halaman detail)
	attendance := numericStr(r.Attendance)
	if attendance == "" {
		attendance = "—"
	}
	return panel.CSTrainingRow{
		ID:            r.ID,
		AccountName:   r.AccountName,
		TrainingTopic: r.TrainingTopic,
		StatusLabel:   statusLabel,
		StatusBadge:   statusBadge,
		TrainerName:   trainerName,
		TrainingDate:  csTrainingDateLabel(r.TrainingDate),
		Participants:  participants,
		Attendance:    attendance,
	}
}

// csTrainingKPIView memetakan CountCSTrainingKPIs → CSTrainingKPIs view.
func csTrainingKPIView(k db.CountCSTrainingKPIsRow) panel.CSTrainingKPIs {
	return panel.CSTrainingKPIs{
		Total:       int(k.TotalCount),
		Scheduled:   int(k.ScheduledCount),
		Completed:   int(k.CompletedCount),
		Rescheduled: int(k.RescheduledCount),
		Cancelled:   int(k.CancelledCount),
	}
}
