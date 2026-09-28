package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_row_test.go — BL-175: kolom "Sisa Hari" Renewals
// (renewalRowView) membawa DaysLeftBand/DaysLeftBandCls REUSE subDerivedStatus,
// MENDAMPINGI (bukan menggantikan) Status derivasi renewal (renewalDerivedStatus)
// yang sudah ada. PendingApproval dapat band bermakna sama seperti Active (bukan
// lagi passthrough mentah) — lihat subDerivedStatus (subscriptions_status.go).

func TestRenewalRowView_DaysLeftBand(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	dateAfter := func(days int) pgtype.Date {
		return pgtype.Date{Time: now.AddDate(0, 0, days), Valid: true}
	}
	cases := []struct {
		name          string
		status        string
		end           pgtype.Date
		wantBand      string
		wantBandCls   string
		wantStatusLbl string
	}{
		{"active d=5 → Segera Jatuh Tempo band, Status tetap Akan Jatuh Tempo",
			"Active", dateAfter(5), "Segera Jatuh Tempo", "badge badge-warning badge-outline", "Akan Jatuh Tempo"},
		{"pending approval d=5 → band sama seperti Active, Status tetap Akan Jatuh Tempo",
			"PendingApproval", dateAfter(5), "Segera Jatuh Tempo", "badge badge-warning badge-outline", "Akan Jatuh Tempo"},
		{"active lewat tempo → Masa Tenggang band, Status juga Masa Tenggang",
			"Active", dateAfter(-1), "Masa Tenggang", "badge badge-error", "Masa Tenggang"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row := renewalListRowFromDefault(db.ListRenewalsRow{
				VillageName: "Desa Uji", Status: c.status, EndDate: c.end,
			})
			v := renewalRowView(row, now, true)
			if v.DaysLeftBand != c.wantBand {
				t.Errorf("DaysLeftBand = %q, want %q", v.DaysLeftBand, c.wantBand)
			}
			if v.DaysLeftBandCls != c.wantBandCls {
				t.Errorf("DaysLeftBandCls = %q, want %q", v.DaysLeftBandCls, c.wantBandCls)
			}
			if v.Status != c.wantStatusLbl {
				t.Errorf("Status (kolom Status, TAK boleh berubah oleh band) = %q, want %q", v.Status, c.wantStatusLbl)
			}
		})
	}
}
