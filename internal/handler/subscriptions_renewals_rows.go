package handler

import (
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_rows.go — renewalListRow (bentuk antara) + converter
// dari keenam struct sqlc Renewals. Dipisah dari subscriptions_renewals_row.go
// krn ambang File Health yang sama.

// renewalListRow = field YANG DIPAKAI renewalRowView, diekstrak dari ENAM
// struct sqlc berbeda (db.ListRenewalsRow & lima db.ListRenewalsSortByXRow —
// satu query = satu struct meski SELECT sama persis, BL-157g) agar logika
// mapping (derivasi status, Jenis fallback, mask F4) TAK diduplikasi per
// query — mirror subListRow (subscriptions_page.go).
type renewalListRow struct {
	ID            int64
	VillageName   string
	PlanName      *string
	ItemCount     int64
	Status        string
	EndDate       pgtype.Date
	AutoRenew     bool
	RenewalType   *string
	RenewalStatus *string
	PreviousValue pgtype.Numeric
	Mrr           pgtype.Numeric
}

func renewalListRowFromDefault(s db.ListRenewalsRow) renewalListRow {
	return renewalListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, AutoRenew: s.AutoRenew, RenewalType: s.RenewalType,
		RenewalStatus: s.RenewalStatus, PreviousValue: s.PreviousValue, Mrr: s.Mrr,
	}
}

func renewalListRowFromVillageSort(s db.ListRenewalsSortByVillageRow) renewalListRow {
	return renewalListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, AutoRenew: s.AutoRenew, RenewalType: s.RenewalType,
		RenewalStatus: s.RenewalStatus, PreviousValue: s.PreviousValue, Mrr: s.Mrr,
	}
}

func renewalListRowFromPlanSort(s db.ListRenewalsSortByPlanRow) renewalListRow {
	return renewalListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, AutoRenew: s.AutoRenew, RenewalType: s.RenewalType,
		RenewalStatus: s.RenewalStatus, PreviousValue: s.PreviousValue, Mrr: s.Mrr,
	}
}

func renewalListRowFromDateSort(s db.ListRenewalsSortByDateRow) renewalListRow {
	return renewalListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, AutoRenew: s.AutoRenew, RenewalType: s.RenewalType,
		RenewalStatus: s.RenewalStatus, PreviousValue: s.PreviousValue, Mrr: s.Mrr,
	}
}

func renewalListRowFromTypeSort(s db.ListRenewalsSortByTypeRow) renewalListRow {
	return renewalListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, AutoRenew: s.AutoRenew, RenewalType: s.RenewalType,
		RenewalStatus: s.RenewalStatus, PreviousValue: s.PreviousValue, Mrr: s.Mrr,
	}
}

func renewalListRowFromMrrSort(s db.ListRenewalsSortByMrrRow) renewalListRow {
	return renewalListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, AutoRenew: s.AutoRenew, RenewalType: s.RenewalType,
		RenewalStatus: s.RenewalStatus, PreviousValue: s.PreviousValue, Mrr: s.Mrr,
	}
}
