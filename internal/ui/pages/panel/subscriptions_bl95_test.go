package panel

import (
	"strings"
	"testing"
)

// subscriptions_bl95_test.go — regresi BL-95 (murni-data): 4 kartu KPI header
// (delta "MRR baru bln ini" + denominator "dari N desa"), tabel diramping ke 6
// kolom (Desa · Paket · MRR · Status · Renewal Date · CSM — ARR & Mulai dibuang),
// dan kolom Status merender badge dari StatusClass derivasi handler.

func TestSubList_KPICardsRender(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base:         "/w/desa",
		StatusFilter: "Active",
		Statuses:     []string{"Active"},
		KPIs: SubKPIs{
			TotalMRR: "Rp 12.000.000", NewMRR: "Rp 2.000.000", ARR: "Rp 144.000.000",
			ActiveSubs: "8", Accounts: "20", ChurnRate: "20,0%",
		},
		Items: []SubRow{{ID: 1, Village: "Desa Satu"}},
	}))
	for _, want := range []string{
		"Total MRR", "Rp 12.000.000",
		"+Rp 2.000.000 bln ini", // delta
		"ARR", "Rp 144.000.000",
		"Langganan Aktif", "dari 20 desa", // denominator
		"Churn Rate", "20,0%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("KPI header harus memuat %q:\n%s", want, out)
		}
	}
}

func TestSubList_SlimColumns(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base:         "/w/desa",
		StatusFilter: "Active",
		Statuses:     []string{"Active"},
		Items: []SubRow{{
			ID: 1, Village: "Desa Satu", Plan: "Paket A", MRR: "Rp 5.000.000",
			Status: "Aman", StatusClass: "badge badge-success",
			Renewal: "2026-12-01", CSM: "Budi",
		}},
	}))
	// Header baru hadir (anchor </th> agar tak bentrok dgn label kartu KPI).
	for _, want := range []string{"Renewal Date</th>", "CSM</th>", "MRR</th>", "Status</th>"} {
		if !strings.Contains(out, want) {
			t.Errorf("header %q harus hadir:\n%s", want, out)
		}
	}
	// Kolom lama yang dibuang TAK muncul sebagai header tabel.
	for _, gone := range []string{"ARR</th>", "Mulai</th>", "Berakhir</th>", "Pemilik</th>"} {
		if strings.Contains(out, gone) {
			t.Errorf("header %q harus DIBUANG (BL-95):\n%s", gone, out)
		}
	}
	// Status derivasi memakai class dari handler + label.
	if !strings.Contains(out, "badge badge-success") || !strings.Contains(out, "Aman") {
		t.Errorf("kolom Status harus merender StatusClass+label derivasi:\n%s", out)
	}
	// Nilai baris: MRR/Renewal/CSM tampil; ARR (Rp 60jt) tak lagi dirender.
	for _, want := range []string{"Rp 5.000.000", "2026-12-01", "Budi"} {
		if !strings.Contains(out, want) {
			t.Errorf("baris harus memuat %q:\n%s", want, out)
		}
	}
}
