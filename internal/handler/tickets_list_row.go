package handler

import (
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_list_row.go — ticketListRow (tipe penyatu TUJUH struct baris sqlc
// hasil sort berbeda-beda, BL-157h) + tujuh ticketListRowFrom* converter.
// Dipisah dari tickets_format.go agar file itu di bawah ambang tipe
// Route/Handler (150). Satu paket; ticketRowView (tickets_format.go) tetap
// jadi satu-satunya pemakai ticketListRow.

// ticketListRow = field YANG DIPAKAI ticketRowView, diekstrak dari TUJUH
// struct sqlc berbeda (db.ListTicketsRow & 6× db.ListTicketsSortByXRow — satu
// query = satu struct meski SELECT sama persis, BL-157h) agar logika mapping
// (nomor tiket, label SLA derivasi) TAK diduplikasi per query. Pola sama
// subListRow (subscriptions_page.go) / renewalListRow (subscriptions_renewals_row.go).
type ticketListRow struct {
	ID             int64
	AccountName    string
	Subject        string
	Priority       string
	Status         string
	SlaDeadlineAt  pgtype.Timestamptz
	ResolvedAt     pgtype.Timestamptz
	AssignedToName *string
}

func ticketListRowFromDefault(r db.ListTicketsRow) ticketListRow {
	return ticketListRow{
		ID: r.ID, AccountName: r.AccountName, Subject: r.Subject,
		Priority: r.Priority, Status: r.Status,
		SlaDeadlineAt: r.SlaDeadlineAt, ResolvedAt: r.ResolvedAt,
		AssignedToName: r.AssignedToName,
	}
}

func ticketListRowFromVillageSort(r db.ListTicketsSortByVillageRow) ticketListRow {
	return ticketListRow{
		ID: r.ID, AccountName: r.AccountName, Subject: r.Subject,
		Priority: r.Priority, Status: r.Status,
		SlaDeadlineAt: r.SlaDeadlineAt, ResolvedAt: r.ResolvedAt,
		AssignedToName: r.AssignedToName,
	}
}

func ticketListRowFromSubjectSort(r db.ListTicketsSortBySubjectRow) ticketListRow {
	return ticketListRow{
		ID: r.ID, AccountName: r.AccountName, Subject: r.Subject,
		Priority: r.Priority, Status: r.Status,
		SlaDeadlineAt: r.SlaDeadlineAt, ResolvedAt: r.ResolvedAt,
		AssignedToName: r.AssignedToName,
	}
}

func ticketListRowFromPrioritySort(r db.ListTicketsSortByPriorityRow) ticketListRow {
	return ticketListRow{
		ID: r.ID, AccountName: r.AccountName, Subject: r.Subject,
		Priority: r.Priority, Status: r.Status,
		SlaDeadlineAt: r.SlaDeadlineAt, ResolvedAt: r.ResolvedAt,
		AssignedToName: r.AssignedToName,
	}
}

func ticketListRowFromStatusSort(r db.ListTicketsSortByStatusRow) ticketListRow {
	return ticketListRow{
		ID: r.ID, AccountName: r.AccountName, Subject: r.Subject,
		Priority: r.Priority, Status: r.Status,
		SlaDeadlineAt: r.SlaDeadlineAt, ResolvedAt: r.ResolvedAt,
		AssignedToName: r.AssignedToName,
	}
}

func ticketListRowFromAgentSort(r db.ListTicketsSortByAgentRow) ticketListRow {
	return ticketListRow{
		ID: r.ID, AccountName: r.AccountName, Subject: r.Subject,
		Priority: r.Priority, Status: r.Status,
		SlaDeadlineAt: r.SlaDeadlineAt, ResolvedAt: r.ResolvedAt,
		AssignedToName: r.AssignedToName,
	}
}

func ticketListRowFromSlaSort(r db.ListTicketsSortBySlaRow) ticketListRow {
	return ticketListRow{
		ID: r.ID, AccountName: r.AccountName, Subject: r.Subject,
		Priority: r.Priority, Status: r.Status,
		SlaDeadlineAt: r.SlaDeadlineAt, ResolvedAt: r.ResolvedAt,
		AssignedToName: r.AssignedToName,
	}
}
