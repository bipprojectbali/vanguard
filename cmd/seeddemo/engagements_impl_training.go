package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// engagements_impl_training.go — seeder demo untuk implementation tasks &
// training sessions (bagian dari bundel engagements). Dipisah dari
// engagements.go agar file di bawah ambang tipe Service/Lainnya (300). Satu
// paket main — data seed identik.

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
