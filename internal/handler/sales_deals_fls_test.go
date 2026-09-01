package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sales_deals_fls_test.go — F4 (field-level) Amount di modul Deals (4.2, inti
// pipeline penjualan): kebijakan umum canSeeARR/maskARR (semua role KECUALI
// Support). Sebelum M9-2 Deals tak punya file test SAMA SEKALI — bukan gap
// wiring (dealRowView/dealDetailView sudah memanggil maskARR sejak awal),
// melainkan gap CAKUPAN TEST (M9-2 gap 4, plan eager-doodling-sutherland.md).
//
// Support & CS (csm, sejak BL-11) TAK punya crm:deals sama sekali
// (business_policy.csv) → canViewDeals menolak SEBELUM dealDetailView/DealDetail
// pernah dipanggil lewat HTTP (F2-blocked total, sama pola
// sales_quotes_fls_test.go). Amount di baris list/pipeline diuji LANGSUNG atas
// dealRowView (murni data, tanpa HTTP): F4 (canSeeARR) TEGAK LURUS terhadap F2
// — csm tetap di sisi "boleh lihat Amount" seandainya bisa mencapai baris,
// jadi cabut-akses BL-11 murni soal gate modul, bukan masking field. Amount di
// detail diuji lewat HTTP nyata utk admin/manager/sales (reachable — write
// mencakup read, business.conf:35); csm & Support (F2-blocked) diuji sesuai
// jalurnya: csm → 403 nyata (bukti gate BL-11), Support → dealDetailView
// langsung utk masking.

// TestDealRowView_AmountMasked: Amount tersamar utk Support, tampil apa adanya
// utk role lain (admin/manager/sales/csm) di baris daftar/pipeline Deal. csm
// TETAP diuji di sini walau F2-blocked sejak BL-11: ini menguji F4 (canSeeARR,
// masking field) yang TEGAK LURUS terhadap F2 (gate modul) — csm ada di sisi
// "boleh lihat Amount" seandainya mencapai baris; cabut-akses BL-11 murni gate.
func TestDealRowView_AmountMasked(t *testing.T) {
	d := db.Deal{
		DealName: "Deal Baris",
		Stage:    "Prospecting",
		Amount:   numFrom(t, "7500000"),
	}
	const wantAmount = "Rp 7.500.000"

	v := dealRowView(d, nil, "support")
	if v.Amount != flsHidden {
		t.Errorf("support: Amount harus tersamar (%s), got %q", flsHidden, v.Amount)
	}

	for _, role := range []string{"admin", "manager", "sales", "csm"} {
		v := dealRowView(d, nil, role)
		if v.Amount != wantAmount {
			t.Errorf("role %q: Amount harus %q, got %q", role, wantAmount, v.Amount)
		}
	}
}

// TestDealDetailView_AmountMasked: Amount di halaman detail Deal — matriks
// 5-role. admin/manager/sales lewat HTTP nyata (DealDetail, Amount tampil);
// csm & Support F2-blocked (tak punya crm:deals): csm diuji 403 nyata (bukti
// gate BL-11), Support diuji langsung atas dealDetailView utk masking.
func TestDealDetailView_AmountMasked(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Deal", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "8200000")
	const wantAmount = "Rp 8.200.000"

	for _, role := range []string{"admin", "manager", "sales"} {
		t.Run("role="+role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/deals/"+itoa(deal.ID), nil, itoa(deal.ID))
			rec := env.runAccount(uid, "owner", role, req, env.h.DealDetail)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, wantAmount) {
				t.Errorf("role %q: Amount %q harus terlihat, body:\n%s", role, wantAmount, body)
			}
		})
	}

	// csm (BL-11): F2-blocked — DealDetail menolak 403 SEBELUM Amount pernah
	// dirakit. Bukti gate modul dicabut utk CS, bukan sekadar field tersamar.
	t.Run("role=csm", func(t *testing.T) {
		req := accountsReq(http.MethodGet, "/w/test/deals/"+itoa(deal.ID), nil, itoa(deal.ID))
		rec := env.runAccount(uid, "owner", "csm", req, env.h.DealDetail)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("csm harus 403 di DealDetail (BL-11), got %d\n%s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), wantAmount) {
			t.Errorf("csm: Amount %q BOCOR di body 403 — deal tak boleh terekspos sama sekali", wantAmount)
		}
	})

	t.Run("role=support", func(t *testing.T) {
		req := accountsReq(http.MethodGet, "/w/test/deals/"+itoa(deal.ID), nil, itoa(deal.ID))
		env.runAccount(uid, "owner", "support", req, func(w http.ResponseWriter, r *http.Request) {
			names, err := env.h.memberNameMap(r.Context())
			if err != nil {
				t.Fatalf("memberNameMap: %v", err)
			}
			v := env.h.dealDetailView(r.Context(), "", deal, names)
			if v.Amount != flsHidden {
				t.Errorf("support: Amount harus tersamar (%s), got %q", flsHidden, v.Amount)
			}
			if v.Amount == wantAmount {
				t.Errorf("support: Amount mentah %q BOCOR — nilai asli lolos ke yang tak berhak", wantAmount)
			}
		})
	})
}
