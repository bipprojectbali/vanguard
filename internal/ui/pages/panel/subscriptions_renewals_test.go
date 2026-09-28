package panel

import (
	"strings"
	"testing"
)

// subscriptions_renewals_test.go — BL-175: kolom "Sisa Hari" Renewals merender
// DUA info sekaligus (angka hari + label band urgensi DaysLeftBand/DaysLeftBandCls
// dari handler, reuse subDerivedStatus) TANPA menyentuh kolom "Status" yang sudah
// ada (renewalDerivedStatus) — keduanya diverifikasi tampil independen.

func TestRenewalsList_DaysLeftBandRenders(t *testing.T) {
	out := renderLeads(t, RenewalsList(RenewalsView{
		Base:    "/w/desa",
		Window:  "due",
		Windows: []RenewalWindow{{Key: "due", Label: "Akan Jatuh Tempo"}},
		Items: []RenewalRow{{
			ID: 1, Village: "Desa Satu", Plan: "Paket A", RenewalDate: "2026-09-15",
			DaysLeft: "5 hari", DaysLeftBand: "Segera Jatuh Tempo",
			DaysLeftBandCls: "badge badge-warning badge-outline",
			Type:            "Auto", Status: "Akan Jatuh Tempo", StatusClass: "badge badge-warning",
			PrevValue: "Rp 1.000.000", CurrentMRR: "Rp 1.200.000",
		}},
	}))
	for _, want := range []string{
		"5 hari",
		"Segera Jatuh Tempo",
		"badge badge-warning badge-outline",
		"Akan Jatuh Tempo", // kolom Status derivasi renewal, tak berubah
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Sisa Hari/Status harus memuat %q:\n%s", want, out)
		}
	}
}

// TestRenewalsList_DaysLeftNoBandFallback: baris tanpa DaysLeftBand (mis. tak
// pernah diisi, seharusnya tak terjadi via renewalRowView tapi view fail-soft)
// merender angka hari saja tanpa badge kosong.
func TestRenewalsList_DaysLeftNoBandFallback(t *testing.T) {
	out := renderLeads(t, RenewalsList(RenewalsView{
		Base:    "/w/desa",
		Window:  "due",
		Windows: []RenewalWindow{{Key: "due", Label: "Akan Jatuh Tempo"}},
		Items: []RenewalRow{{
			ID: 1, Village: "Desa Dua", DaysLeft: "10 hari",
		}},
	}))
	if !strings.Contains(out, "10 hari") {
		t.Errorf("DaysLeft harus tetap tampil tanpa band:\n%s", out)
	}
	if strings.Contains(out, "badge-ghost\" ></span>") {
		t.Errorf("tak boleh render badge band kosong:\n%s", out)
	}
}
