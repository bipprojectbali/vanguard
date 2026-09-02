package handler

import (
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_row.go — pemetaan baris Renewals ke view (renewalRowView)
// + label sisa hari (daysLeftLabel), dipisah dari subscriptions_renewals.go
// (handler + enum jendela) demi ambang tipe Route/Handler (150). Package sama;
// F4 maskARR identik.

// renewalRowView memetakan satu baris → baris tabel Renewals. Days Left dihitung
// relatif "hari ini" (zona waktu app). Prev→Current = previous_value → MRR,
// keduanya nilai komersial → maskARR (F4, diperbaiki audit FLS M9-1). Kolom
// mengikuti wireframe 5.2 (tanpa kolom pemilik).
func renewalRowView(s db.ListRenewalsRow, now time.Time, businessRole string) panel.RenewalRow {
	return panel.RenewalRow{
		ID:          s.ID,
		Village:     s.VillageName,
		Plan:        s.PlanName,
		RenewalDate: dateStr(s.EndDate),
		DaysLeft:    daysLeftLabel(now, s.EndDate),
		Type:        deref(s.RenewalType),
		Status:      s.Status,
		PrevValue:   maskARR(formatRupiah(s.PreviousValue), businessRole),
		CurrentMRR:  maskARR(formatRupiah(s.Mrr), businessRole),
	}
}

// daysLeftLabel = selisih hari (kalender) end_date terhadap hari ini, diformat.
// Keduanya dinormalkan ke tanggal sipil (UTC midnight) agar bebas jam/zona →
// selisih bulat hari. Query sudah menjamin end_date terisi, tapi tetap fail-soft.
func daysLeftLabel(now time.Time, end pgtype.Date) string {
	if !end.Valid {
		return "—"
	}
	a := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Time.Year(), end.Time.Month(), end.Time.Day(), 0, 0, 0, 0, time.UTC)
	d := int(b.Sub(a).Hours() / 24)
	switch {
	case d > 0:
		return strconv.Itoa(d) + " hari"
	case d == 0:
		return "Hari ini"
	default:
		return strconv.Itoa(-d) + " hari telat"
	}
}
