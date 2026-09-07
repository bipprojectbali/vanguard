package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_kpi_test.go — unit murni (tanpa DB) untuk derivasi
// BL-94: label Jenis fallback (renewalTypeLabel), Status derivasi
// (renewalDerivedStatus) di sekitar ambang dueSoonDays, dan pemformatan 4 KPI
// (renewalKPIView). Menjaga aturan tampil agar tak diam-diam bergeser.

func TestRenewalTypeLabel(t *testing.T) {
	auto := "Auto-renew"
	empty := ""
	cases := []struct {
		name      string
		renewal   *string
		autoRenew bool
		want      string
	}{
		{"renewal_type terisi menang", &auto, false, "Auto-renew"},
		{"nil + auto_renew true → Auto", nil, true, "Auto"},
		{"nil + auto_renew false → Manual", nil, false, "Manual"},
		{"kosong + auto_renew true → Auto (fallback)", &empty, true, "Auto"},
		{"kosong + auto_renew false → Manual", &empty, false, "Manual"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renewalTypeLabel(c.renewal, c.autoRenew); got != c.want {
				t.Errorf("renewalTypeLabel = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRenewalDerivedStatus(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	renewed := "Renewed"
	other := "Pending"
	dateAfter := func(days int) pgtype.Date {
		return pgtype.Date{Time: now.AddDate(0, 0, days), Valid: true}
	}
	cases := []struct {
		name      string
		end       pgtype.Date
		status    *string
		wantLabel string
		wantClass string
	}{
		{"renewed menang tanpa peduli tanggal", dateAfter(-100), &renewed, "Diperpanjang", "badge badge-success"},
		{"end_date invalid → strip", pgtype.Date{}, nil, "—", "badge badge-ghost"},
		{"lewat tempo → Masa Tenggang", dateAfter(-1), nil, "Masa Tenggang", "badge badge-error"},
		{"hari ini (d=0) → Jatuh Tempo", dateAfter(0), nil, "Jatuh Tempo", "badge badge-warning"},
		{"tepat ambang 30 → Jatuh Tempo", dateAfter(dueSoonDays), nil, "Jatuh Tempo", "badge badge-warning"},
		{"di atas ambang → Aman", dateAfter(dueSoonDays + 1), nil, "Aman", "badge badge-success"},
		{"status non-Renewed diabaikan (pakai tanggal)", dateAfter(5), &other, "Jatuh Tempo", "badge badge-warning"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			label, cls := renewalDerivedStatus(now, c.end, c.status)
			if label != c.wantLabel {
				t.Errorf("label = %q, want %q", label, c.wantLabel)
			}
			if cls != c.wantClass {
				t.Errorf("class = %q, want %q", cls, c.wantClass)
			}
		})
	}
}

func TestRenewalKPIView(t *testing.T) {
	got := renewalKPIView(db.RenewalKPIsRow{
		Due30:          7,
		Grace:          3,
		AutoActive:     12,
		DuePast12m:     4,
		RenewedPast12m: 3,
	})
	if got.Due30 != "7" || got.Grace != "3" || got.AutoRenew != "12" {
		t.Errorf("count format salah: %+v", got)
	}
	if got.RenewalRate != "75,0%" { // 3/4 = 75%
		t.Errorf("RenewalRate = %q, want 75,0%%", got.RenewalRate)
	}

	// Kohort kosong (pembagi nol) → rate "—" (fail-soft ratePct), count nol string.
	empty := renewalKPIView(db.RenewalKPIsRow{})
	if empty.Due30 != "0" || empty.RenewalRate != "—" {
		t.Errorf("empty KPI salah: %+v", empty)
	}
}
