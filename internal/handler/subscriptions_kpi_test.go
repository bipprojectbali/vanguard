package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_kpi_test.go — unit murni (tanpa DB) untuk BL-95: Status DERIVASI
// kolom daftar (subDerivedStatus) di sekitar ambang dueSoonDays + passthrough
// lifecycle non-Active, dan pemformatan 4 KPI header (subscriptionListKPIView)
// termasuk delta "MRR baru bln ini" + denominator "dari N desa" + churn rate.

func TestSubDerivedStatus(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	dateAfter := func(days int) pgtype.Date {
		return pgtype.Date{Time: now.AddDate(0, 0, days), Valid: true}
	}
	cases := []struct {
		name      string
		status    string
		end       pgtype.Date
		wantLabel string
		wantClass string
	}{
		// Active → DERIVASI dari end_date (reuse ambang BL-94).
		{"active lewat tempo → Masa Tenggang", "Active", dateAfter(-1), "Masa Tenggang", "badge badge-error"},
		{"active hari ini (d=0) → Jatuh Tempo", "Active", dateAfter(0), "Jatuh Tempo", "badge badge-warning"},
		{"active tepat ambang 30 → Jatuh Tempo", "Active", dateAfter(dueSoonDays), "Jatuh Tempo", "badge badge-warning"},
		{"active di atas ambang → Aman", "Active", dateAfter(dueSoonDays + 1), "Aman", "badge badge-success"},
		{"active end_date invalid → Aman (fail-soft)", "Active", pgtype.Date{}, "Aman", "badge badge-success"},
		// Non-Active → label lifecycle apa adanya (derivasi tak bermakna).
		{"trial → passthrough info", "Trial", dateAfter(-100), "Trial", "badge badge-info"},
		{"pending approval → warning", "PendingApproval", dateAfter(5), "PendingApproval", "badge badge-warning"},
		{"cancelled lewat tempo TETAP Cancelled", "Cancelled", dateAfter(-100), "Cancelled", "badge badge-error"},
		{"churned → error", "Churned", dateAfter(-1), "Churned", "badge badge-error"},
		{"status tak dikenal → ghost", "Weird", dateAfter(5), "Weird", "badge badge-ghost"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			label, cls := subDerivedStatus(c.status, c.end, now)
			if label != c.wantLabel {
				t.Errorf("label = %q, want %q", label, c.wantLabel)
			}
			if cls != c.wantClass {
				t.Errorf("class = %q, want %q", cls, c.wantClass)
			}
		})
	}
}

func TestSubscriptionListKPIView(t *testing.T) {
	got := subscriptionListKPIView(db.SubscriptionListKPIsRow{
		TotalMrr:      numFrom(t, "12000000"),
		TotalArr:      numFrom(t, "144000000"),
		NewMrr:        numFrom(t, "2000000"),
		ActiveCount:   8,
		Churned30:     2,
		TotalAccounts: 20,
	})
	if got.TotalMRR != "Rp 12.000.000" {
		t.Errorf("TotalMRR = %q", got.TotalMRR)
	}
	if got.ARR != "Rp 144.000.000" {
		t.Errorf("ARR = %q", got.ARR)
	}
	if got.NewMRR != "Rp 2.000.000" { // delta "+… bln ini"
		t.Errorf("NewMRR = %q", got.NewMRR)
	}
	if got.ActiveSubs != "8" {
		t.Errorf("ActiveSubs = %q", got.ActiveSubs)
	}
	if got.Accounts != "20" { // denominator "dari N desa"
		t.Errorf("Accounts = %q", got.Accounts)
	}
	if got.ChurnRate != "20,0%" { // 2 / (8+2) = 20%
		t.Errorf("ChurnRate = %q, want 20,0%%", got.ChurnRate)
	}

	// Nol langganan (pembagi churn nol) → rate "—" (fail-soft ratePct).
	empty := subscriptionListKPIView(db.SubscriptionListKPIsRow{})
	if empty.ActiveSubs != "0" || empty.ChurnRate != "—" {
		t.Errorf("empty KPI salah: %+v", empty)
	}
}
