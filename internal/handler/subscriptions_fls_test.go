package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"
)

// subscriptions_fls_test.go — F4 (field-level) MRR di subscriptions/renewals/
// churn: kebijakan umum canSeeARR/maskARR (semua role KECUALI Support), beda
// dari ARR (admin+manager-only). Diperbaiki audit FLS M9-1 — sebelumnya MRR
// dirender mentah ke SEMUA viewer (skema.md §9 eksplisit sebut MRR).
//
// Diuji LANGSUNG atas subRowView/renewalRowView/churnRowView (bukan lewat
// HTTP): Support tak punya crm:subscriptions sama sekali (business_policy.csv)
// → 403 sebelum sampai row-view manapun (TestSubscriptions_GateRead), dan
// data_scope bawaan Support = DataScopeNone (business_defaults.go) → F3 selalu
// nol baris di /reports/subscriptions sekalipun Support lolos F2 di sana
// (crm:reports read). Jadi tak ada jalur nyata di mana baris MRR milik Support
// pernah dirender — pengujian HTTP-level "body memuat flsHidden" mustahil
// dirakit (tak pernah ada baris). Unit test langsung atas mapper murni-data
// ini memverifikasi WIRING pemanggilan maskARR di titik render (predikat
// canSeeARR/maskARR itu sendiri sudah diuji tuntas di fls_test.go) — pertahanan
// berlapis bila F2/F3 berubah di masa depan (pola sama dgn
// TestReportsSales_Export_NoLeak_Support).

// TestSubRowView_MRRMasked: MRR tersamar bila pemanggil TANPA kapabilitas
// crm:subscriptions/arr (canARR=false), tampil apa adanya bila true, di baris
// daftar Active Subscriptions. subRowView murni-data sejak BL-169 (menerima
// bool, bukan businessRole) — pemetaan role→canARR sendiri diuji di authz/
// business_defaults (default grant admin/manager/sales/csm, TANPA support).
func TestSubRowView_MRRMasked(t *testing.T) {
	row := db.ListSubscriptionsRow{
		VillageName: "Desa MRR",
		PlanName:    ptr("Paket MRR"),
		Mrr:         numFrom(t, "5000000"),
		Arr:         numFrom(t, "60000000"),
	}
	const wantMRR = "Rp 5.000.000"
	now := time.Now() // uji sumbu canARR (MRR), bukan derivasi status → now bebas

	v := subRowView(subListRowFromDefault(row), nil, false, now)
	if v.MRR != flsHidden {
		t.Errorf("canARR=false: MRR harus tersamar (%s), got %q", flsHidden, v.MRR)
	}

	v = subRowView(subListRowFromDefault(row), nil, true, now)
	if v.MRR != wantMRR {
		t.Errorf("canARR=true: MRR harus %q, got %q", wantMRR, v.MRR)
	}
}

// TestRenewalRowView_MRRMasked: Prev→Current (previous_value/MRR) tersamar bila
// canARR=false, tampil apa adanya bila true, di baris Renewals. renewalRowView
// murni-data sejak BL-169 — pemetaan role→canARR diuji terpisah.
func TestRenewalRowView_MRRMasked(t *testing.T) {
	row := db.ListRenewalsRow{
		VillageName:   "Desa Renewal",
		PlanName:      ptr("Paket Renewal"),
		Mrr:           numFrom(t, "5000000"),
		PreviousValue: numFrom(t, "4500000"),
	}
	now := time.Now()
	const wantMRR, wantPrev = "Rp 5.000.000", "Rp 4.500.000"

	v := renewalRowView(renewalListRowFromDefault(row), now, false)
	if v.CurrentMRR != flsHidden {
		t.Errorf("canARR=false: CurrentMRR harus tersamar (%s), got %q", flsHidden, v.CurrentMRR)
	}
	if v.PrevValue != flsHidden {
		t.Errorf("canARR=false: PrevValue harus tersamar (%s), got %q", flsHidden, v.PrevValue)
	}

	v = renewalRowView(renewalListRowFromDefault(row), now, true)
	if v.CurrentMRR != wantMRR {
		t.Errorf("canARR=true: CurrentMRR harus %q, got %q", wantMRR, v.CurrentMRR)
	}
	if v.PrevValue != wantPrev {
		t.Errorf("canARR=true: PrevValue harus %q, got %q", wantPrev, v.PrevValue)
	}
}

// TestChurnRowView_MRRMasked: MRR Hilang (lost_value_mrr) tersamar bila
// canARR=false, tampil apa adanya bila true, di baris Churn. churnRowView
// murni-data sejak BL-169 — pemetaan role→canARR diuji terpisah.
func TestChurnRowView_MRRMasked(t *testing.T) {
	row := db.ListChurnedRow{
		VillageName:  "Desa Churn",
		PlanName:     ptr("Paket Churn"),
		LostValueMrr: numFrom(t, "500000"),
	}
	const wantLostMRR = "Rp 500.000"

	v := churnRowView(row, nil, false)
	if v.LostMRR != flsHidden {
		t.Errorf("canARR=false: LostMRR harus tersamar (%s), got %q", flsHidden, v.LostMRR)
	}

	v = churnRowView(row, nil, true)
	if v.LostMRR != wantLostMRR {
		t.Errorf("canARR=true: LostMRR harus %q, got %q", wantLostMRR, v.LostMRR)
	}
}
