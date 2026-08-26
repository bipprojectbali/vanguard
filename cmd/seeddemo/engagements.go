package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// engagements.go — bundel 4 sub-fitur onboarding/CS Modul 6: Engagements/
// Check-ins (6.5), Success Plans (6.3), Implementation Tracker (6.2.1.1),
// Training Schedule (6.2.1.2). Masing-masing disebar merata ke semua nilai
// enum status/tipe/channel-nya.

type bundleResult struct {
	engagements, successPlans, implTasks, trainings int
}

func seedEngagementsBundle(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo) (bundleResult, error) {
	var result bundleResult

	n, err := seedEngagements(ctx, q, tenantID, rng, owner, accounts)
	if err != nil {
		return result, fmt.Errorf("engagements: %w", err)
	}
	result.engagements = n

	n, err = seedSuccessPlans(ctx, q, tenantID, rng, owner, accounts)
	if err != nil {
		return result, fmt.Errorf("success_plans: %w", err)
	}
	result.successPlans = n

	n, err = seedImplTasks(ctx, q, tenantID, rng, owner, accounts)
	if err != nil {
		return result, fmt.Errorf("cs_impl_tasks: %w", err)
	}
	result.implTasks = n

	n, err = seedTrainings(ctx, q, tenantID, rng, owner, accounts)
	if err != nil {
		return result, fmt.Errorf("cs_trainings: %w", err)
	}
	result.trainings = n

	return result, nil
}

var engagementSubjects = []string{
	"Check-in rutin bulanan", "QBR triwulan", "Panggilan onboarding awal",
	"Eskalasi keluhan pengguna", "Diskusi renewal", "Tindak lanjut pelatihan",
}

const engagementTotal = 30

// seedEngagements — 30 baris, sebar merata ke 5 engagement_type × 4 status.
func seedEngagements(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo) (int, error) {
	types := []string{"touch_point", "qbr", "onboarding_call", "escalation", "check_in"}
	statuses := []string{"planned", "done", "skipped", "rescheduled"}
	channels := []string{"whatsapp", "call", "video", "site_visit"}
	frequencies := []string{"weekly", "monthly", "quarterly", "ad_hoc"}
	today := time.Now()

	count := 0
	for i := 0; i < engagementTotal; i++ {
		acc := accounts[i%len(accounts)]
		typ := types[i%len(types)]
		status := statuses[i%len(statuses)]

		var scheduled time.Time
		var outcome *string
		var nextDue pgtype.Date
		switch status {
		case "done":
			scheduled = today.AddDate(0, 0, -rng.Intn(60))
			outcome = ptr("Sesi selesai, desa merespons baik terhadap tindak lanjut yang diberikan.")
			nextDue = pgDate(today.AddDate(0, 0, 30+rng.Intn(60)))
		case "planned":
			scheduled = today.AddDate(0, 0, rng.Intn(30))
		case "rescheduled":
			scheduled = today.AddDate(0, 0, 5+rng.Intn(20))
		default: // skipped
			scheduled = today.AddDate(0, 0, -rng.Intn(20))
		}

		var engOwner *int64
		if rng.Intn(100) < 70 {
			engOwner = owner
		}
		var createdBy *int64
		if rng.Intn(100) < 70 {
			createdBy = owner
		}

		if _, err := q.CreateEngagement(ctx, db.CreateEngagementParams{
			TenantID:       tenantID,
			AccountID:      acc.ID,
			Subject:        pick(rng, engagementSubjects),
			EngagementType: typ,
			Frequency:      pickOrNil(rng, 60, frequencies),
			ScheduledAt:    pgtype.Timestamptz{Time: scheduled, Valid: true},
			Status:         status,
			Channel:        pickOrNil(rng, 80, channels),
			Outcome:        outcome,
			NextDueDate:    nextDue,
			OwnerID:        engOwner,
			CreatedBy:      createdBy,
		}); err != nil {
			return count, fmt.Errorf("engagement akun #%d: %w", acc.ID, err)
		}
		count++
	}
	return count, nil
}

var successPlanNames = []string{
	"Peningkatan Adopsi Modul Keuangan", "Rencana Onboarding Lanjutan",
	"Pemulihan Kesehatan Akun", "Ekspansi Penggunaan Modul Presensi",
	"Rencana Retensi Kontrak", "Penguatan Advokasi Desa Percontohan",
}
var successPlanObjectives = []string{
	"Meningkatkan penggunaan fitur pelaporan APBDes secara rutin.",
	"Memastikan seluruh perangkat desa terlatih menggunakan sistem.",
	"Menstabilkan skor kesehatan akun ke kategori Healthy.",
	"Memperluas pemakaian modul ke seluruh kaur/kasi.",
}
var successPlanMetrics = []string{
	"Login mingguan ≥ 5 kali", "Skor kesehatan ≥ 70", "Laporan APBDes tepat waktu",
	"Partisipasi training ≥ 80%",
}

const successPlanTotal = 15

// seedSuccessPlans — 15 baris, sebar ke seluruh 5 plan_status.
func seedSuccessPlans(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo) (int, error) {
	statuses := []string{"Draft", "Active", "Achieved", "At-Risk", "Cancelled"}
	today := time.Now()

	count := 0
	for i := 0; i < successPlanTotal; i++ {
		acc := accounts[i%len(accounts)]
		status := statuses[i%len(statuses)]

		var progress int16
		switch status {
		case "Achieved":
			progress = 100
		case "Cancelled":
			progress = int16(intn32(rng, 0, 40))
		case "At-Risk":
			progress = int16(intn32(rng, 10, 50))
		case "Active":
			progress = int16(intn32(rng, 20, 90))
		default: // Draft
			progress = 0
		}

		var ownerCsm *int64
		if rng.Intn(100) < 70 {
			ownerCsm = owner
		}
		var createdBy *int64
		if rng.Intn(100) < 70 {
			createdBy = owner
		}

		if _, err := q.CreateSuccessPlan(ctx, db.CreateSuccessPlanParams{
			TenantID:      tenantID,
			AccountID:     acc.ID,
			PlanName:      pick(rng, successPlanNames),
			Objective:     ptr(pick(rng, successPlanObjectives)),
			SuccessMetric: ptr(pick(rng, successPlanMetrics)),
			TargetDate:    pgDate(today.AddDate(0, 3+rng.Intn(6), 0)),
			PlanStatus:    status,
			Progress:      progress,
			OwnerCsm:      ownerCsm,
			CreatedBy:     createdBy,
		}); err != nil {
			return count, fmt.Errorf("success_plan akun #%d: %w", acc.ID, err)
		}
		count++
	}
	return count, nil
}

var implTaskNames = []string{
	"Setup akun admin desa", "Migrasi data penduduk awal", "Konfigurasi struktur organisasi",
	"Import data APBDes tahun berjalan", "Verifikasi hak akses kaur/kasi",
	"Uji coba modul presensi", "Setup template surat menyurat", "Aktivasi notifikasi WhatsApp",
}

const implTaskTotal = 40

// seedImplTasks — 40 baris, sebar ke seluruh 4 task_status.
func seedImplTasks(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo) (int, error) {
	statuses := []string{"to_do", "in_progress", "done", "blocked"}
	today := time.Now()

	count := 0
	for i := 0; i < implTaskTotal; i++ {
		acc := accounts[i%len(accounts)]
		status := statuses[i%len(statuses)]

		var dueDate pgtype.Date
		switch status {
		case "done":
			dueDate = pgDate(today.AddDate(0, 0, -rng.Intn(30)))
		case "blocked":
			dueDate = pgDate(today.AddDate(0, 0, -rng.Intn(10)))
		default:
			dueDate = pgDate(today.AddDate(0, 0, rng.Intn(30)))
		}

		var taskOwner *int64
		if rng.Intn(100) < 70 {
			taskOwner = owner
		}
		var createdBy *int64
		if rng.Intn(100) < 70 {
			createdBy = owner
		}

		if _, err := q.CreateCSImplTask(ctx, db.CreateCSImplTaskParams{
			TenantID:   tenantID,
			AccountID:  acc.ID,
			TaskName:   pick(rng, implTaskNames),
			TaskStatus: status,
			OwnerID:    taskOwner,
			DueDate:    dueDate,
			CreatedBy:  createdBy,
		}); err != nil {
			return count, fmt.Errorf("cs_impl_task akun #%d: %w", acc.ID, err)
		}
		count++
	}
	return count, nil
}

var trainingTopics = []string{
	"Training Admin Desa", "Pelatihan Modul Keuangan", "Pelatihan Presensi Perangkat Desa",
	"Pelatihan Pengaduan Warga", "Pelatihan Arsip Digital", "Pelatihan Surat Menyurat",
}

const trainingTotal = 20

// seedTrainings — 20 baris, sebar ke seluruh 4 training_status.
func seedTrainings(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo) (int, error) {
	statuses := []string{"scheduled", "completed", "rescheduled", "cancelled"}
	today := time.Now()

	count := 0
	for i := 0; i < trainingTotal; i++ {
		acc := accounts[i%len(accounts)]
		status := statuses[i%len(statuses)]

		var trainingDate time.Time
		switch status {
		case "completed":
			trainingDate = today.AddDate(0, 0, -rng.Intn(60))
		case "cancelled":
			trainingDate = today.AddDate(0, 0, -rng.Intn(20))
		case "rescheduled":
			trainingDate = today.AddDate(0, 0, 5+rng.Intn(15))
		default: // scheduled
			trainingDate = today.AddDate(0, 0, rng.Intn(30))
		}

		var trainerID *int64
		if rng.Intn(100) < 70 {
			trainerID = owner
		}
		participants := intn32(rng, 3, 20)
		var createdBy *int64
		if rng.Intn(100) < 70 {
			createdBy = owner
		}

		// CreateCSTraining selalu mulai di status 'scheduled' (default kolom);
		// attendance hanya settable via UpdateCSTrainingStatus (diisi setelah
		// training benar-benar 'completed') — meniru alur nyata, bukan tulis
		// status akhir langsung.
		t, err := q.CreateCSTraining(ctx, db.CreateCSTrainingParams{
			TenantID:       tenantID,
			AccountID:      acc.ID,
			TrainingTopic:  pick(rng, trainingTopics),
			TrainingDate:   pgtype.Timestamptz{Time: trainingDate, Valid: true},
			TrainerID:      trainerID,
			Participants:   &participants,
			TrainingStatus: "scheduled",
			CreatedBy:      createdBy,
		})
		if err != nil {
			return count, fmt.Errorf("cs_training akun #%d: %w", acc.ID, err)
		}

		if status != "scheduled" {
			var attendance pgtype.Numeric
			if status == "completed" {
				attendance, err = num(fmt.Sprintf("%d.00", 55+rng.Intn(45)))
				if err != nil {
					return count, err
				}
			}
			if _, err := q.UpdateCSTrainingStatus(ctx, db.UpdateCSTrainingStatusParams{
				TrainingStatus: status,
				Attendance:     attendance,
				Participants:   &participants,
				UpdatedBy:      owner,
				ID:             t.ID,
			}); err != nil {
				return count, fmt.Errorf("update status cs_training #%d: %w", t.ID, err)
			}
		}
		count++
	}
	return count, nil
}
