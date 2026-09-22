package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// subscriptions_detail_test.go — halaman Detail Langganan (M5-3b) + redesign
// kartu 2-kolom (BL-154). Dipindah dari subscriptions_test.go (ukuran file);
// seedSubscription tetap di sana (dipakai kedua file). Tiga blok di sini:
//
//   - Detail dasar (found/out-of-scope) + F4 masking ARR — TEST YANG SUDAH ADA,
//     dipindah apa adanya.
//   - BL-154: kartu baru (Status & Lifecycle, Renewal, System & Audit), grid
//     2-kolom, lookup lintas-modul customer_success (dgn & tanpa baris), source
//     deal (terisi & kosong), field ❌ yang di-drop TAK muncul.

// TestSubscriptions_DetailFound: pemilik (sales own-scope) membuka detail
// langganannya → 200, body memuat nama desa & paket.
func TestSubscriptions_DetailFound(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Detail", "PLAN-DET", "1000000")
	acc := env.seedAccount(t, "Desa Detail", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Detail") {
		t.Error("detail harus memuat nama desa")
	}
	if !strings.Contains(body, "Paket Detail") {
		t.Error("detail harus memuat nama paket")
	}
}

// TestSubscriptions_DetailNotFoundWhenOutOfScope: sales membuka detail langganan
// milik anggota lain → 404 (di luar cakupan; keberadaan baris tak diungkap sbg 403).
func TestSubscriptions_DetailNotFoundWhenOutOfScope(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	planID := env.seedPlan(t, "Paket Orang", "PLAN-OTH", "1000000")
	acc := env.seedAccount(t, "Desa Orang", &other, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &other, "Active", "500000", "6000000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
	if rec.Code != http.StatusNotFound {
		t.Errorf("langganan di luar cakupan harus 404, got %d", rec.Code)
	}
}

// --- F4: masking ARR -------------------------------------------------------

// TestSubscriptions_ARRVisibleForDefaultRoles: MRR & ARR tampil apa adanya
// untuk KEEMPAT peran bawaan (admin/manager/sales/csm) — DEFAULT grant
// kapabilitas crm:subscriptions/arr (BL-58 admin+manager, BL-169 menambah
// sales+csm agar tenant existing tak regresi dari perilaku lama, business_
// defaults.go). Support tak diuji di sini — F2 (crm:subscriptions) memblokirnya
// total sebelum halaman ini terbuka (lihat TestSubscriptions_GateRead di
// subscriptions_test.go); ia tersamar via TestSubRowView_MRRMasked dkk. Peran
// CUSTOM ber-grant/tanpa-grant diuji di TestSubscriptions_ARRCustomRoleCapability
// (satu-satunya jalur nyata untuk melihat ARR tersamar via HTTP, sejak BL-169
// menyamakan sumbu MRR & ARR untuk keempat peran bawaan).
func TestSubscriptions_ARRVisibleForDefaultRoles(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Nilai", "PLAN-VAL", "1000000")
	acc := env.seedAccount(t, "Desa Nilai", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "5000000", "60000000")

	const wantMRR = "Rp 5.000.000"
	const wantARR = "Rp 60.000.000"

	for _, role := range []string{"sales", "csm", "admin", "manager"} {
		t.Run("role="+role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
			rec := env.runAccount(uid, "owner", role, req, env.h.SubscriptionDetail)
			if rec.Code != http.StatusOK {
				t.Fatalf("role %q detail status = %d, want 200\n%s", role, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, wantMRR) {
				t.Errorf("role %q: MRR %q harus terlihat", role, wantMRR)
			}
			if !strings.Contains(body, wantARR) {
				t.Errorf("role %q: ARR %q harus terlihat", role, wantARR)
			}
		})
	}
}

// TestSubscriptions_ARRCustomRoleCapability — inti BL-58: visibilitas ARR
// mengikuti KAPABILITAS ter-matriks (crm:subscriptions/arr), bukan cek nama role
// hardcode. Sebelum BL-58, `role == admin || == manager` mengunci ARR ke nama
// peran sistem sehingga peran custom (mis. "Direktur"/"Finance") TAK PERNAH bisa
// melihat ARR walau diberi cakupan penuh — kontra desain role-aware. Dua peran
// custom bercakupan 'all' diuji berdampingan: "direktur" DIBERI grant arr →
// melihat ARR; "finance" TANPA grant → tersamar (flsHidden). Keduanya punya read
// (agar F2 lolos) & scope 'all' (agar F3 tak menyaring baris), jadi satu-satunya
// pembeda adalah grant arr. Sejak BL-169, MRR (canSeeARR) & ARR (canSeeSubscriptionARR)
// adalah KAPABILITAS YANG SAMA — beda dari sebelum BL-169 saat MRR berbasis nama
// role & ortogonal (custom role selalu melihatnya tersamar terlepas grant arr).
// Jadi di sini MRR diuji BERSAMA ARR: keduanya ikut grant arr yang sama untuk
// "direktur" (lihat) maupun "finance" (tersamar).
func TestSubscriptions_ARRCustomRoleCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	// Peran custom: keduanya read (lolos F2). "direktur" + arr, "finance" tanpa.
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "direktur", Obj: "crm:subscriptions", Act: "read"},
		authz.BusinessPerm{Role: "direktur", Obj: "crm:subscriptions", Act: "arr"},
		authz.BusinessPerm{Role: "finance", Obj: "crm:subscriptions", Act: "read"},
	)
	planID := env.seedPlan(t, "Paket Custom", "PLAN-CST", "1000000")
	acc := env.seedAccount(t, "Desa Custom", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "5000000", "60000000")

	const wantMRR = "Rp 5.000.000"
	const wantARR = "Rp 60.000.000"

	open := func(t *testing.T, role string) string {
		t.Helper()
		req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
		rec := env.runAccountScope(uid, "member", role, authz.DataScopeAll, req, env.h.SubscriptionDetail)
		if rec.Code != http.StatusOK {
			t.Fatalf("role %q detail status = %d, want 200\n%s", role, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	t.Run("custom role WITH arr grant sees MRR & ARR", func(t *testing.T) {
		body := open(t, "direktur")
		if !strings.Contains(body, wantMRR) {
			t.Errorf("direktur: MRR %q harus terlihat (grant crm:subscriptions/arr, BL-169)", wantMRR)
		}
		if !strings.Contains(body, wantARR) {
			t.Errorf("direktur: ARR %q harus terlihat (grant crm:subscriptions/arr)", wantARR)
		}
	})

	t.Run("custom role WITHOUT arr grant masks MRR & ARR", func(t *testing.T) {
		body := open(t, "finance")
		if !strings.Contains(body, flsHidden) {
			t.Errorf("finance: MRR/ARR harus tersamar (%s) — tanpa grant arr", flsHidden)
		}
		// wantMRR SATU kali wajar: harga satuan item (UnitPrice) sengaja TAK
		// disamarkan (subItemRows — kuantitas & harga satuan bukan sensitif),
		// kebetulan sama angka dgn MRR krn seedSubscription mem-seed 1 item cermin
		// (qty=1, unit_price=mrr). Field MRR yg SEHARUSNYA tersamar (header,
		// PrevToCurrent, item MRR/Subtotal, chain) tak boleh menambah kemunculan.
		if n := strings.Count(body, wantMRR); n > 1 {
			t.Errorf("finance: MRR mentah %q bocor di luar harga satuan item (muncul %dx, want 1)", wantMRR, n)
		}
		if strings.Contains(body, wantARR) {
			t.Errorf("finance: ARR mentah %q tak boleh bocor tanpa grant", wantARR)
		}
	})
}

// --- BL-154: kartu redesign -------------------------------------------------

// seedSubscriptionFromDeal = varian seedSubscription dgn source_deal_id terisi
// (kartu System & Audit, "Sumber"). Helper TERPISAH (bukan menambah parameter
// ke seedSubscription bersama) agar signature yang dipakai banyak test lain
// tak berubah.
func (e *testEnv) seedSubscriptionFromDeal(
	t *testing.T, accountID, planID, dealID int64, owner *int64, mrr, arr string,
) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            &planID,
		SourceDealID:      &dealID,
		Status:            "Active",
		AutoRenew:         false,
		Mrr:               numFrom(t, mrr),
		Arr:               numFrom(t, arr),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed subscription from deal: %v", err)
	}
	lineNo := int16(1)
	if _, err := e.q.AddSubscriptionItem(t.Context(), db.AddSubscriptionItemParams{
		SubscriptionID: s.ID,
		TenantID:       e.tenantID,
		PlanID:         &planID,
		Quantity:       1,
		UnitPrice:      numFrom(t, mrr),
		Subtotal:       numFrom(t, mrr),
		Mrr:            numFrom(t, mrr),
		Arr:            numFrom(t, arr),
		LineNo:         &lineNo,
	}); err != nil {
		t.Fatalf("seed subscription item: %v", err)
	}
	return s
}

// setSubscriptionUpdatedBy mengubah updated_by (kartu System & Audit, "Diubah
// Oleh") lewat UpdateSubscription — field lain dipertahankan (baca dulu via
// GetSubscription) krn query ini REPLACE profil penuh, bukan partial.
func (e *testEnv) setSubscriptionUpdatedBy(t *testing.T, subID, updatedBy int64) {
	t.Helper()
	s, err := e.q.GetSubscription(t.Context(), subID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if _, err := e.q.UpdateSubscription(t.Context(), db.UpdateSubscriptionParams{
		SubscriptionOwner:  s.SubscriptionOwner,
		PlanID:             s.PlanID,
		StartDate:          s.StartDate,
		EndDate:            s.EndDate,
		BillingCycle:       s.BillingCycle,
		AutoRenew:          s.AutoRenew,
		ContractTermMonths: s.ContractTermMonths,
		Mrr:                s.Mrr,
		Arr:                s.Arr,
		QuantitySeats:      s.QuantitySeats,
		DiscountPct:        s.DiscountPct,
		PaymentStatus:      s.PaymentStatus,
		RenewalStatus:      s.RenewalStatus,
		RenewalType:        s.RenewalType,
		RenewalOwner:       s.RenewalOwner,
		RenewalQuoteID:     s.RenewalQuoteID,
		UpdatedBy:          &updatedBy,
		ID:                 subID,
	}); err != nil {
		t.Fatalf("update subscription updated_by: %v", err)
	}
}

// TestSubscriptionDetail_CardsGrid: grid 2-kolom (mobile-first) + ketiga kartu
// baru BL-154 render judulnya; ketiga field ❌ yang DIPUTUSKAN drop (Setup Fee,
// Payment Method, Last Invoice — docs/crm/tasks.md BL-154) TAK muncul sama
// sekali di v1.
func TestSubscriptionDetail_CardsGrid(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Grid", "PLAN-GRD", "1000000")
	acc := env.seedAccount(t, "Desa Grid", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "md:grid-cols-2") {
		t.Error("kartu detail harus grid mobile-first (1 kolom < md, 2 kolom md+)")
	}
	// "&" di-escape g.Text jadi "&amp;" di HTML (cardRows judul via g.Text) —
	// assert bentuk yang benar-benar dirender, bukan literal "&".
	for _, title := range []string{"Status &amp; Lifecycle", "Renewal", "System &amp; Audit"} {
		if !strings.Contains(body, title) {
			t.Errorf("kartu %q harus render", title)
		}
	}
	for _, dropped := range []string{"Setup Fee", "Payment Method", "Metode Pembayaran", "Last Invoice", "Invoice Terakhir"} {
		if strings.Contains(body, dropped) {
			t.Errorf("field %q sudah DIPUTUSKAN drop dari v1 (BL-154), tak boleh muncul", dropped)
		}
	}
}

// TestSubscriptionDetail_HealthOnboarding: lookup lintas-modul customer_success
// (BL-114). Dengan baris CS → label Health & status Onboarding muncul. TANPA
// baris CS (pgx.ErrNoRows) → tetap 200 (best-effort, bukan gagal request),
// field kosong (tak ada label Health apa pun yang bocor).
func TestSubscriptionDetail_HealthOnboarding(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket CS", "PLAN-CS1", "1000000")

	t.Run("dengan baris customer_success", func(t *testing.T) {
		acc := env.seedAccount(t, "Desa Sehat", &uid, nil, nil)
		env.seedCustomerSuccess(t, acc.ID) // Healthy / In Progress / progress 50
		sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

		req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
		rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Sehat") {
			t.Error("Health 'Sehat' (dari customer_success.health_status=Healthy) harus muncul")
		}
		if !strings.Contains(body, "In Progress") {
			t.Error("status Onboarding harus muncul")
		}
	})

	t.Run("tanpa baris customer_success", func(t *testing.T) {
		acc := env.seedAccount(t, "Desa Belum CS", &uid, nil, nil)
		sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

		req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
		rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
		if rec.Code != http.StatusOK {
			t.Fatalf("desa tanpa baris customer_success harus tetap 200 (best-effort), got %d\n%s",
				rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, label := range []string{"Sehat", "Berisiko", "Kritis"} {
			if strings.Contains(body, label) {
				t.Errorf("tanpa baris customer_success, label Health %q tak boleh muncul", label)
			}
		}
	})
}

// TestSubscriptionDetail_SourceDeal: "Sumber" pada kartu System & Audit.
// source_deal_id terisi → kode/nama deal + tautan /deals/{id} muncul;
// source_deal_id NULL (langganan bukan dari deal) → tautan itu TIDAK muncul,
// tapi halaman tetap 200 (field tampil "—", bukan error).
func TestSubscriptionDetail_SourceDeal(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Deal", "PLAN-DL1", "1000000")

	t.Run("source_deal_id terisi", func(t *testing.T) {
		acc := env.seedAccount(t, "Desa Konversi", &uid, nil, nil)
		deal := env.seedDeal(t, acc.ID, &uid)
		sub := env.seedSubscriptionFromDeal(t, acc.ID, planID, deal.ID, &uid, "500000", "6000000")

		req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
		rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "/deals/"+itoa(deal.ID)) {
			t.Error("Sumber harus menaut ke deal asal (/deals/{id})")
		}
		if !strings.Contains(body, "Deal Uji") {
			t.Error("Sumber harus memuat nama deal asal")
		}
	})

	t.Run("source_deal_id NULL", func(t *testing.T) {
		acc := env.seedAccount(t, "Desa Langsung", &uid, nil, nil)
		sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

		req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
		rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "/deals/") {
			t.Error("langganan tanpa source_deal_id tak boleh menaut ke deal apa pun")
		}
	})
}

// TestSubscriptionDetail_SystemAuditNames: kartu System & Audit menampilkan
// NAMA (via memberNameMap, jatuh ke email bila tanpa nama tampilan) untuk
// pembuat & pengubah — bukan ID mentah.
func TestSubscriptionDetail_SystemAuditNames(t *testing.T) {
	env, uid := setupAccounts(t) // uid = test@local
	other := env.seedMember(t, "editor@local", "member", 0).ID
	planID := env.seedPlan(t, "Paket Audit", "PLAN-AUD", "1000000")
	acc := env.seedAccount(t, "Desa Audit", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")
	env.setSubscriptionUpdatedBy(t, sub.ID, other)

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/"+itoa(sub.ID), nil, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "test@local") {
		t.Error("'Dibuat Oleh' harus memuat nama/email pembuat (test@local)")
	}
	if !strings.Contains(body, "editor@local") {
		t.Error("'Diubah Oleh' harus memuat nama/email pengubah (editor@local)")
	}
}
