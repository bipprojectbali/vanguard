package handler

import (
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_row.go — mapping baris Subscription Lists (dipindah dari
// subscriptions_page.go krn ambang File Health; mirror health_score_row.go).

// subListRow = field YANG DIPAKAI subRowView, diekstrak dari DUA struct sqlc
// berbeda (db.ListSubscriptionsRow & db.ListSubscriptionsSortByVillageRow —
// satu query = satu struct meski SELECT sama persis) agar logika mapping
// (derivasi status, mask MRR F4, resolusi CSM) TAK diduplikasi per query.
type subListRow struct {
	ID                int64
	VillageName       string
	PlanName          *string
	ItemCount         int64
	Status            string
	EndDate           pgtype.Date
	Mrr               pgtype.Numeric
	SubscriptionOwner *int64
}

func subListRowFromDefault(s db.ListSubscriptionsRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromVillageSort(s db.ListSubscriptionsSortByVillageRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromStatusSort(s db.ListSubscriptionsSortByStatusRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromPlanSort(s db.ListSubscriptionsSortByPlanRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromMrrSort(s db.ListSubscriptionsSortByMrrRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromRenewalSort(s db.ListSubscriptionsSortByRenewalRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromCsmSort(s db.ListSubscriptionsSortByCsmRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

// subRowView memetakan satu baris daftar → baris tabel ramping (BL-95: 6 kolom
// Desa · Paket · MRR · Status · Renewal Date · CSM; ARR & Mulai dibuang). MRR:
// kebijakan umum maskARR (skema.md §9 — SEMUA role kecuali Support; diperbaiki
// audit FLS M9-1). Status = DERIVASI renewal (subDerivedStatus) HANYA untuk
// langganan Active; status daur hidup lain (Trial/Cancelled/…) tampil apa adanya
// (badge lifecycle) — derivasi berbasis end_date tak bermakna untuk status
// terminal. CSM (owner) diresolusi dari peta anggota. canARR dihitung SEKALI
// oleh pemanggil (canSeeARR(ctx)).
func subRowView(s subListRow, names map[int64]string, canARR bool, now time.Time) panel.SubRow {
	label, cls := subDerivedStatus(s.Status, s.EndDate, now)
	return panel.SubRow{
		ID:          s.ID,
		Village:     s.VillageName,
		Plan:        subPlanDisplay(s.PlanName, s.ItemCount),
		Status:      label,
		StatusClass: cls,
		MRR:         maskARR(formatRupiah(s.Mrr), canARR),
		Renewal:     dateStr(s.EndDate),
		CSM:         ownerName(s.SubscriptionOwner, names),
	}
}

// subPlanDisplay, subDerivedStatus, subStatusLifecycleClass di
// subscriptions_status.go.
