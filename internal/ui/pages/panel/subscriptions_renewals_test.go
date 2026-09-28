package panel

import (
	"strings"
	"testing"
)

// subscriptions_renewals_test.go — BL-175: kolom "Sisa Hari" Renewals mewarnai
// angka hari via DaysLeftTextCls (kelas warna teks polos dari handler, reuse
// subDerivedStatus) TANPA badge/label tambahan, dan TANPA menyentuh kolom
// "Status" yang sudah ada (renewalDerivedStatus) — keduanya diverifikasi
// tampil independen.

func TestRenewalsList_DaysLeftTextClsRenders(t *testing.T) {
	out := renderLeads(t, RenewalsList(RenewalsView{
		Base:    "/w/desa",
		Window:  "due",
		Windows: []RenewalWindow{{Key: "due", Label: "Akan Jatuh Tempo"}},
		Items: []RenewalRow{{
			ID: 1, Village: "Desa Satu", Plan: "Paket A", RenewalDate: "2026-09-15",
			DaysLeft: "5 hari", DaysLeftTextCls: "text-warning",
			Type: "Auto", Status: "Akan Jatuh Tempo", StatusClass: "badge badge-warning",
			PrevValue: "Rp 1.000.000", CurrentMRR: "Rp 1.200.000",
		}},
	}))
	for _, want := range []string{
		"5 hari",
		"text-warning",
		"Akan Jatuh Tempo", // kolom Status derivasi renewal, tak berubah
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Sisa Hari/Status harus memuat %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "badge badge-warning badge-outline") {
		t.Errorf("Sisa Hari TAK boleh render badge — warna teks saja:\n%s", out)
	}
}

// TestRenewalsList_DaysLeftNoTextClsFallback: baris tanpa DaysLeftTextCls (mis.
// tak pernah diisi, seharusnya tak terjadi via renewalRowView tapi view
// fail-soft) merender angka hari saja tanpa kelas warna tambahan.
func TestRenewalsList_DaysLeftNoTextClsFallback(t *testing.T) {
	out := renderLeads(t, RenewalsList(RenewalsView{
		Base:    "/w/desa",
		Window:  "due",
		Windows: []RenewalWindow{{Key: "due", Label: "Akan Jatuh Tempo"}},
		Items: []RenewalRow{{
			ID: 1, Village: "Desa Dua", DaysLeft: "10 hari",
		}},
	}))
	if !strings.Contains(out, "10 hari") {
		t.Errorf("DaysLeft harus tetap tampil tanpa kelas warna:\n%s", out)
	}
}
