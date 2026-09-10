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

// TestRenewalDerivedStatus — derivasi Status renewal (BL-94 + BL-127 + BL-152).
// Prioritas: Renewed DULU (BL-152 — sudah diperpanjang menang atas due-window),
// lalu due/grace mengikuti PERSIS predikat jendela ListRenewals: badge "Akan Jatuh
// Tempo"/"Masa Tenggang" HANYA muncul untuk baris yang benar-benar masuk tab
// 'due'/'grace' (status ∈ {Active,PendingApproval} untuk due; status=Active untuk
// grace). Status non-aktif ber-end_date dekat memakai label LIFECYCLE-nya, bukan
// badge urgensi (bug BL-127 yang diperbaiki).
func TestRenewalDerivedStatus(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	renewed := "Renewed"
	dateAfter := func(days int) pgtype.Date {
		return pgtype.Date{Time: now.AddDate(0, 0, days), Valid: true}
	}
	cases := []struct {
		name          string
		status        string
		end           pgtype.Date
		renewalStatus *string
		wantLabel     string
		wantClass     string
	}{
		// Active — jalur urgensi + aman.
		{"Active lewat tempo → Masa Tenggang", "Active", dateAfter(-1), nil, "Masa Tenggang", "badge badge-error"},
		{"Active hari ini (d=0) → Akan Jatuh Tempo", "Active", dateAfter(0), nil, "Akan Jatuh Tempo", "badge badge-warning"},
		{"Active tepat ambang 30 → Akan Jatuh Tempo", "Active", dateAfter(dueSoonDays), nil, "Akan Jatuh Tempo", "badge badge-warning"},
		{"Active di atas ambang → Aman", "Active", dateAfter(dueSoonDays + 1), nil, "Aman", "badge badge-success"},
		{"Active end_date invalid → strip", "Active", pgtype.Date{}, nil, "—", "badge badge-ghost"},
		// PendingApproval ikut jendela 'due' (upsell menunggu), TAPI tidak 'grace'.
		{"PendingApproval dekat → Akan Jatuh Tempo", "PendingApproval", dateAfter(5), nil, "Akan Jatuh Tempo", "badge badge-warning"},
		{"PendingApproval lewat tempo → label lifecycle (bukan grace)", "PendingApproval", dateAfter(-1), nil, "PendingApproval", "badge badge-warning"},
		// Renewed = tab 'renewed'; baris Active hasil renewal end-nya jauh (>30) → Diperpanjang.
		{"Active + Renewed jauh → Diperpanjang", "Active", dateAfter(365), &renewed, "Diperpanjang", "badge badge-success"},
		// BL-152: Renewed MENANG atas due-window "apa pun sisa hari" — end dekat (≤30)
		// & lewat tempo tetap "Diperpanjang", bukan "Akan Jatuh Tempo"/"Masa Tenggang".
		{"Active + Renewed dekat (≤30) → Diperpanjang (bukan due)", "Active", dateAfter(10), &renewed, "Diperpanjang", "badge badge-success"},
		{"Active + Renewed lewat tempo → Diperpanjang (bukan grace)", "Active", dateAfter(-3), &renewed, "Diperpanjang", "badge badge-success"},
		// BL-127 inti: status non-aktif ber-end dekat TIDAK boleh dapat badge urgensi.
		{"Expired end dekat → label lifecycle, bukan Jatuh Tempo", "Expired", dateAfter(5), nil, "Expired", "badge badge-error"},
		{"Expired lewat tempo → label lifecycle, bukan Masa Tenggang", "Expired", dateAfter(-10), nil, "Expired", "badge badge-error"},
		{"Churned end dekat → label lifecycle", "Churned", dateAfter(3), nil, "Churned", "badge badge-error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			label, cls := renewalDerivedStatus(now, c.status, c.end, c.renewalStatus)
			if label != c.wantLabel {
				t.Errorf("label = %q, want %q", label, c.wantLabel)
			}
			if cls != c.wantClass {
				t.Errorf("class = %q, want %q", cls, c.wantClass)
			}
		})
	}
}

// TestRenewMRR: parsing MRR baru form perpanjang. Kosong = ikut MRR lama; input
// terkelompok ribuan ("250.000") harus lolos (input pakai data-numgroup, backend
// cleanThousands sbg penjaga bila JS mati); negatif/non-angka → kode galat.
func TestRenewMRR(t *testing.T) {
	old := numFrom(t, "500000")
	cases := []struct {
		name     string
		raw      string
		wantVal  string // "" = harap = MRR lama
		wantCode string
	}{
		{"kosong → ikut lama", "", "500000", ""},
		{"angka polos", "750000", "750000", ""},
		{"terkelompok ribuan", "250.000", "250000", ""},
		{"spasi + titik", " 1.250.000 ", "1250000", ""},
		{"negatif ditolak", "-1", "", "new_mrr"},
		{"non-angka ditolak", "abc", "", "new_mrr"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, code := renewMRR(c.raw, old)
			if code != c.wantCode {
				t.Fatalf("code = %q, want %q", code, c.wantCode)
			}
			if code == "" && !numEq(n, c.wantVal) {
				t.Errorf("nilai tak cocok, want %s", c.wantVal)
			}
		})
	}
}

func TestRenewalKPIView(t *testing.T) {
	got := renewalKPIView(db.RenewalKPIsRow{
		Due30:          7,
		Grace:          3,
		RenewedCount:   12,
		DuePast12m:     4,
		RenewedPast12m: 3,
	})
	if got.Due30 != "7" || got.Grace != "3" || got.Renewed != "12" {
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
