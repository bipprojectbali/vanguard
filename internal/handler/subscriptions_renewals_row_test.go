package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_row_test.go — BL-175: kolom "Sisa Hari" Renewals
// (renewalRowView) membawa DaysLeftTextCls (kelas WARNA TEKS, TANPA badge/label)
// REUSE subDerivedStatus via bandTextClass, MENDAMPINGI (bukan menggantikan)
// Status derivasi renewal (renewalDerivedStatus) yang sudah ada. PendingApproval
// dapat band bermakna sama seperti Active (bukan lagi passthrough mentah) —
// lihat subDerivedStatus (subscriptions_status.go).

func TestRenewalRowView_DaysLeftTextCls(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	dateAfter := func(days int) pgtype.Date {
		return pgtype.Date{Time: now.AddDate(0, 0, days), Valid: true}
	}
	cases := []struct {
		name          string
		status        string
		end           pgtype.Date
		wantTextCls   string
		wantStatusLbl string
	}{
		{"active d=5 → text-warning, Status tetap Akan Jatuh Tempo",
			"Active", dateAfter(5), "text-warning", "Akan Jatuh Tempo"},
		{"pending approval d=5 → band sama seperti Active, Status tetap Akan Jatuh Tempo",
			"PendingApproval", dateAfter(5), "text-warning", "Akan Jatuh Tempo"},
		{"active lewat tempo → text-error, Status juga Masa Tenggang",
			"Active", dateAfter(-1), "text-error", "Masa Tenggang"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row := renewalListRowFromDefault(db.ListRenewalsRow{
				VillageName: "Desa Uji", Status: c.status, EndDate: c.end,
			})
			v := renewalRowView(row, now, true)
			if v.DaysLeftTextCls != c.wantTextCls {
				t.Errorf("DaysLeftTextCls = %q, want %q", v.DaysLeftTextCls, c.wantTextCls)
			}
			if v.Status != c.wantStatusLbl {
				t.Errorf("Status (kolom Status, TAK boleh berubah oleh band) = %q, want %q", v.Status, c.wantStatusLbl)
			}
		})
	}
}
