package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets.go — 25 tiket dukungan tersebar ke SEMUA 4 status (baru/ditugaskan/
// eskalasi/selesai). sla_deadline_at disimpan sbg snapshot saat create
// (00021_crm_cs_tickets.sql) — bukan dihitung ulang dari sla_policies — jadi
// bebas ditulis di MASA LALU utk ≥5 tiket TERBUKA (breach), independen dari
// created_at.

var ticketSubjects = []string{
	"Login tidak bisa masuk", "Data APBDes tidak sinkron", "Laporan gagal diunduh",
	"Permintaan reset password admin", "Modul presensi error", "Fitur pengaduan warga tak muncul",
	"Import data penduduk gagal", "Cetak surat keterangan bermasalah", "Notifikasi WhatsApp tidak terkirim",
	"Perlu pelatihan ulang modul keuangan", "Server lambat diakses", "Data ganda pada arsip surat",
}

const ticketTotal = 25

// seedTickets mengembalikan ID tiap tiket yang dibuat (dipakai activities.go
// sbg target_id nyata utk target_type='ticket').
func seedTickets(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, owner *int64, accounts []accountInfo, slaPolicies []int64) ([]int64, error) {
	var ids []int64
	today := time.Now()

	// Distribusi status: 8 baru, 6 ditugaskan, 5 eskalasi, 6 selesai (total 25).
	statusPlan := make([]string, 0, ticketTotal)
	for i := 0; i < 8; i++ {
		statusPlan = append(statusPlan, "baru")
	}
	for i := 0; i < 6; i++ {
		statusPlan = append(statusPlan, "ditugaskan")
	}
	for i := 0; i < 5; i++ {
		statusPlan = append(statusPlan, "eskalasi")
	}
	for i := 0; i < 6; i++ {
		statusPlan = append(statusPlan, "selesai")
	}
	rng.Shuffle(len(statusPlan), func(i, j int) { statusPlan[i], statusPlan[j] = statusPlan[j], statusPlan[i] })

	breachBudget := 5 // ≥5 tiket TERBUKA dgn sla_deadline_at di masa lalu.
	for i, finalStatus := range statusPlan {
		acc := accounts[i%len(accounts)]
		priority := weightedPick(rng, []weighted[string]{
			{"rendah", 3}, {"sedang", 5}, {"tinggi", 2},
		})
		subject := pick(rng, ticketSubjects)
		desc := ptr("Dilaporkan via seeddemo untuk keperluan demo dasbor Support.")

		var slaPolicyID *int64
		if len(slaPolicies) > 0 && rng.Intn(100) < 85 {
			id := pick(rng, slaPolicies)
			slaPolicyID = &id
		}

		// Deadline: tiket TERBUKA (baru/ditugaskan/eskalasi) sebagian sengaja
		// breach (masa lalu); tiket 'selesai' deadline wajar (masa lalu tapi
		// diselesaikan sebelum lewat, tercermin dari resolved_at via UpdateTicketStatus).
		var deadline time.Time
		openStatus := finalStatus != "selesai"
		breach := openStatus && breachBudget > 0
		switch {
		case breach:
			deadline = today.AddDate(0, 0, -(1 + rng.Intn(5)))
			breachBudget--
		case openStatus:
			deadline = today.AddDate(0, 0, 1+rng.Intn(7))
		default: // selesai — deadline lampau, tapi tak dihitung breach (sudah tuntas)
			deadline = today.AddDate(0, 0, -(3 + rng.Intn(20)))
		}

		var assignedTo *int64
		if finalStatus != "baru" && rng.Intn(100) < 80 {
			assignedTo = owner
		}

		var createdBy *int64
		if rng.Intn(100) < 70 {
			createdBy = owner
		}

		t, err := q.CreateTicket(ctx, db.CreateTicketParams{
			TenantID:      tenantID,
			AccountID:     acc.ID,
			Subject:       subject,
			Description:   desc,
			Priority:      priority,
			AssignedTo:    assignedTo,
			SlaPolicyID:   slaPolicyID,
			SlaDeadlineAt: pgtype.Timestamptz{Time: deadline, Valid: true},
			CreatedBy:     createdBy,
		})
		if err != nil {
			return ids, fmt.Errorf("tiket akun #%d: %w", acc.ID, err)
		}

		// CreateTicket selalu mulai di status 'baru' (default kolom) — transisi
		// ke status akhir yang direncanakan via UpdateTicketStatus, meniru alur
		// nyata (support.go) alih-alih menulis status akhir langsung.
		if finalStatus != "baru" {
			finalAssigned := assignedTo
			if finalAssigned == nil {
				finalAssigned = owner
			}
			if _, err := q.UpdateTicketStatus(ctx, db.UpdateTicketStatusParams{
				Status:     finalStatus,
				AssignedTo: finalAssigned,
				UpdatedBy:  owner,
				ID:         t.ID,
			}); err != nil {
				return ids, fmt.Errorf("update status tiket #%d: %w", t.ID, err)
			}
		}

		ids = append(ids, t.ID)
	}
	return ids, nil
}
