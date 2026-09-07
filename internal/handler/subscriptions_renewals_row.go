package handler

import (
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_row.go — pemetaan baris Renewals ke view (renewalRowView)
// + label sisa hari (daysLeftLabel), Jenis fallback (renewalTypeLabel), dan Status
// DERIVASI renewal (renewalDerivedStatus). Dipisah dari subscriptions_renewals.go
// (handler + enum jendela) demi ambang tipe Route/Handler (150). Package sama;
// F4 maskARR identik.

// dueSoonDays = ambang "Jatuh Tempo segera" (BL-94, keputusan user ≤30 hari) —
// SAMA dgn window 'due' (queries) agar derivasi Status konsisten dgn tab & KPI.
const dueSoonDays = 30

// renewalRowView memetakan satu baris → baris tabel Renewals. Days Left dihitung
// relatif "hari ini" (zona waktu app). Jenis: renewal_type bila terisi, jika NULL
// fallback ke auto_renew (BL-94). Status = DERIVASI renewal (bukan lifecycle
// subscription.status) sesuai wireframe. Prev→Current = previous_value → MRR,
// keduanya nilai komersial → maskARR (F4, diperbaiki audit FLS M9-1).
func renewalRowView(s db.ListRenewalsRow, now time.Time, businessRole string) panel.RenewalRow {
	label, cls := renewalDerivedStatus(now, s.EndDate, s.RenewalStatus)
	return panel.RenewalRow{
		ID:          s.ID,
		Village:     s.VillageName,
		Plan:        s.PlanName,
		RenewalDate: dateStr(s.EndDate),
		DaysLeft:    daysLeftLabel(now, s.EndDate),
		Type:        renewalTypeLabel(s.RenewalType, s.AutoRenew),
		Status:      label,
		StatusClass: cls,
		PrevValue:   maskARR(formatRupiah(s.PreviousValue), businessRole),
		CurrentMRR:  maskARR(formatRupiah(s.Mrr), businessRole),
	}
}

// renewalTypeLabel = Jenis perpanjangan. renewal_type menang bila terisi; jika
// NULL/kosong (umum untuk langganan awal yang belum pernah diperpanjang) fallback
// ke auto_renew: true→"Auto", false→"Manual" (BL-94, wireframe kolom Type).
func renewalTypeLabel(renewalType *string, autoRenew bool) string {
	if renewalType != nil && *renewalType != "" {
		return *renewalType
	}
	if autoRenew {
		return "Auto"
	}
	return "Manual"
}

// renewalDerivedStatus = Status DERIVASI renewal (BL-94), berbeda dari
// subscription.status (lifecycle). Prioritas: sudah diperpanjang (renewal_status=
// 'Renewed') → "Diperpanjang" (agar baris renewed tak keliru "Masa Tenggang").
// Selebihnya dari selisih end_date vs hari ini: lewat tempo → "Masa Tenggang"
// (error), ≤ dueSoonDays → "Jatuh Tempo" (warning), selain itu → "Aman" (success).
// Mengembalikan label + class badge daisyUI (token semantik, bukan absolut).
func renewalDerivedStatus(now time.Time, end pgtype.Date, renewalStatus *string) (label, badgeClass string) {
	if renewalStatus != nil && *renewalStatus == "Renewed" {
		return "Diperpanjang", "badge badge-success"
	}
	if !end.Valid {
		return "—", "badge badge-ghost"
	}
	a := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Time.Year(), end.Time.Month(), end.Time.Day(), 0, 0, 0, 0, time.UTC)
	d := int(b.Sub(a).Hours() / 24)
	switch {
	case d < 0:
		return "Masa Tenggang", "badge badge-error"
	case d <= dueSoonDays:
		return "Jatuh Tempo", "badge badge-warning"
	default:
		return "Aman", "badge badge-success"
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
