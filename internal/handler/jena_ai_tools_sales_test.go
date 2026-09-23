package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/session"
)

// jena_ai_tools_sales_test.go — F2/F3/F4 tool search_deals/search_leads
// (BL-162 fase 3, jena_ai_tools_sales.go). Dipisah dari jena_ai_tools_test.go
// (file health, CLAUDE.md §8) — file itu sudah dekat batas 400 baris.
//
// Fokus BEDA dari list_my_deals/list_my_leads yang sudah diuji di
// jena_ai_tools_test.go: search_* MENGIKUTI data_scope penanya apa adanya,
// jadi yang paling penting dibuktikan di sini adalah (a) ScopeAll benar-benar
// bisa lintas-pemilik + owner_search menyaringnya, dan (b) ScopeOwn TETAP nol
// baris utk data milik orang lain walau owner_search cocok namanya — bukti
// fix ini tak meregresi isolasi F3.

// --- search_deals -------------------------------------------------------------

func TestJenaSearchDeals_F2DeniedForSupport(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Deal Cari", &uid, nil, nil)
	env.seedDealNamed(t, acc.ID, &uid, "Deal Uji")

	var out string
	env.runJena(uid, "support", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchDeals(ctx, json.RawMessage(`{"query":"","owner_search":""}`))
		if err != nil {
			t.Fatalf("jenaSearchDeals: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data deal") {
		t.Errorf("Support tak punya crm:deals (F2), got %s", out)
	}
}

// TestJenaSearchDeals_F3ScopeAllCrossOwnerAndOwnerSearchFiltersByName:
// Manager (ScopeAll) harus bisa melihat deal LINTAS-pemilik, dan owner_search
// mempersempitnya berdasarkan NAMA pemilik — inilah kasus asli bug report
// ("tampilkan data lead/deal yang dibuat oleh user jun").
func TestJenaSearchDeals_F3ScopeAllCrossOwnerAndOwnerSearchFiltersByName(t *testing.T) {
	env, uid := setupAccounts(t)
	jun := seedMemberNamed(t, env, "jun@local", "Jun Ahmad", "member")
	accMine := env.seedAccount(t, "Desa Deal Saya", &uid, nil, nil)
	accJun := env.seedAccount(t, "Desa Deal Jun", &jun, nil, nil)
	env.seedDealNamed(t, accMine.ID, &uid, "Deal Milik Saya")
	env.seedDealNamed(t, accJun.ID, &jun, "Deal Milik Jun")

	// Tanpa owner_search: manager (ScopeAll) melihat KEDUANYA.
	var outAll string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		outAll, err = env.h.jenaSearchDeals(ctx, json.RawMessage(`{"query":"","owner_search":""}`))
		if err != nil {
			t.Fatalf("jenaSearchDeals: %v", err)
		}
	})
	if !strings.Contains(outAll, "Deal Milik Saya") || !strings.Contains(outAll, "Deal Milik Jun") {
		t.Errorf("Manager (ScopeAll) harus melihat deal lintas-pemilik, got %s", outAll)
	}

	// owner_search="jun" → HANYA deal milik Jun, meniru pertanyaan asli bug
	// report ("deal yang dipegang oleh user jun").
	var outJun string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		outJun, err = env.h.jenaSearchDeals(ctx, json.RawMessage(`{"query":"","owner_search":"jun"}`))
		if err != nil {
			t.Fatalf("jenaSearchDeals: %v", err)
		}
	})
	if !strings.Contains(outJun, "Deal Milik Jun") {
		t.Errorf("owner_search=jun harus memuat deal milik Jun, got %s", outJun)
	}
	if strings.Contains(outJun, "Deal Milik Saya") {
		t.Errorf("owner_search=jun TAK BOLEH memuat deal milik user lain, got %s", outJun)
	}
	if !strings.Contains(outJun, `"owner_label":"Jun Ahmad"`) {
		t.Errorf("owner_label harus nama pemilik siap-tampil, got %s", outJun)
	}
}

// TestJenaSearchDeals_F3ScopeOwnStillZeroForOthersEvenWithMatchingOwnerSearch:
// REGRESI KRITIS — Sales (ScopeOwn) TIDAK BOLEH melihat deal milik user lain
// walau owner_search cocok persis nama pemiliknya. owner_search hanya boleh
// MEMPERSEMPIT, tak pernah MELEBARKAN cakupan F3 (lihat komentar
// ListDealsForJena, queries/deals.sql).
func TestJenaSearchDeals_F3ScopeOwnStillZeroForOthersEvenWithMatchingOwnerSearch(t *testing.T) {
	env, uid := setupAccounts(t)
	jun := seedMemberNamed(t, env, "jun-sales@local", "Jun Ahmad", "member")
	accJun := env.seedAccount(t, "Desa Deal Jun Sales", &jun, nil, nil)
	env.seedDealNamed(t, accJun.ID, &jun, "Deal Milik Jun Sales")

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchDeals(ctx, json.RawMessage(`{"query":"","owner_search":"jun"}`))
		if err != nil {
			t.Fatalf("jenaSearchDeals: %v", err)
		}
	})

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if len(results) != 0 {
		t.Errorf("Sales (ScopeOwn) HARUS nol baris utk deal milik Jun walau owner_search cocok (F3), got %d: %s", len(results), out)
	}
}

// TestJenaSearchDeals_F4AmountMaskedForRoleWithoutArrGrant: peran custom yang
// punya akses modul Deals (F2) tapi TIDAK punya grant crm:subscriptions/arr —
// Amount harus tersamar (satu kapabilitas arr, sama dgn canSeeARR).
func TestJenaSearchDeals_F4AmountMaskedForRoleWithoutArrGrant(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "jenanoarrsales", Obj: "crm:accounts", Act: "read"},
		authz.BusinessPerm{Role: "jenanoarrsales", Obj: "crm:deals", Act: "read"},
	)
	acc := env.seedAccount(t, "Desa Deal NoARR", &uid, nil, nil)
	d := env.seedDealNamed(t, acc.ID, &uid, "Deal NoARR")
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE deals SET amount=$1 WHERE id=$2`, "7000000", d.ID); err != nil {
		t.Fatalf("set amount: %v", err)
	}

	// Peran custom tak dikenal DefaultDataScope → DataScopeNone; set eksplisit
	// DataScopeAll agar F3 lolos & F4 (arr) murni diuji (pola sama
	// TestJenaGetAccountSummary_F4BudgetMaskedForSupport).
	var out string
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", "owner", false,
			env.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, "jenanoarrsales")
		session.SetBusinessDataScope(ctx, authz.DataScopeAll)
		var err error
		out, err = env.h.jenaSearchDeals(withQueries(ctx, env.q), json.RawMessage(`{"query":"","owner_search":""}`))
		if err != nil {
			t.Fatalf("jenaSearchDeals: %v", err)
		}
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(out, flsHidden) {
		t.Errorf("amount harus tersamar utk peran tanpa grant arr (F4), got %s", out)
	}
	if strings.Contains(out, "7.000.000") || strings.Contains(out, "7000000") {
		t.Errorf("amount TAK BOLEH bocor utk peran tanpa grant arr:\n%s", out)
	}
}

// --- search_leads ---------------------------------------------------------------

func TestJenaSearchLeads_F2DeniedForCSM(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLead(t, "Lead Uji Cari", &uid)

	var out string
	env.runJena(uid, "csm", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchLeads(ctx, json.RawMessage(`{"query":"","owner_search":""}`))
		if err != nil {
			t.Fatalf("jenaSearchLeads: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data lead") {
		t.Errorf("CSM tak punya crm:leads (BL-11, F2), got %s", out)
	}
}

// TestJenaSearchLeads_F3ScopeAllCrossOwnerAndOwnerSearchFiltersByEmail: kasus
// PERSIS bug report asli ("lead yang dibuat oleh user jun") — owner_search
// diuji lewat EMAIL (bukan nama) utk membuktikan fallback ILIKE email juga
// jalan, bukan cuma nama (mirror uji nama sudah dicakup search_deals).
func TestJenaSearchLeads_F3ScopeAllCrossOwnerAndOwnerSearchFiltersByEmail(t *testing.T) {
	env, uid := setupAccounts(t)
	jun := env.seedMember(t, "jun.lead@local", "member", 0).ID
	env.seedLead(t, "Lead Milik Saya", &uid)
	env.seedLead(t, "Lead Milik Jun", &jun)

	var outAll string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		outAll, err = env.h.jenaSearchLeads(ctx, json.RawMessage(`{"query":"","owner_search":""}`))
		if err != nil {
			t.Fatalf("jenaSearchLeads: %v", err)
		}
	})
	if !strings.Contains(outAll, "Lead Milik Saya") || !strings.Contains(outAll, "Lead Milik Jun") {
		t.Errorf("Manager (ScopeAll) harus melihat lead lintas-pemilik, got %s", outAll)
	}

	var outJun string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		outJun, err = env.h.jenaSearchLeads(ctx, json.RawMessage(`{"query":"","owner_search":"jun.lead@local"}`))
		if err != nil {
			t.Fatalf("jenaSearchLeads: %v", err)
		}
	})
	if !strings.Contains(outJun, "Lead Milik Jun") {
		t.Errorf("owner_search berbasis email harus memuat lead milik Jun, got %s", outJun)
	}
	if strings.Contains(outJun, "Lead Milik Saya") {
		t.Errorf("owner_search=jun.lead@local TAK BOLEH memuat lead milik user lain, got %s", outJun)
	}
}

// TestJenaSearchLeads_F3ScopeOwnStillZeroForOthersEvenWithMatchingOwnerSearch:
// REGRESI KRITIS, cermin search_deals — Sales (ScopeOwn) tetap nol baris utk
// lead milik user lain walau owner_search cocok.
func TestJenaSearchLeads_F3ScopeOwnStillZeroForOthersEvenWithMatchingOwnerSearch(t *testing.T) {
	env, uid := setupAccounts(t)
	jun := seedMemberNamed(t, env, "jun-lead-sales@local", "Jun Ahmad", "member")
	env.seedLead(t, "Lead Milik Jun Sales", &jun)

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchLeads(ctx, json.RawMessage(`{"query":"","owner_search":"jun"}`))
		if err != nil {
			t.Fatalf("jenaSearchLeads: %v", err)
		}
	})

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if len(results) != 0 {
		t.Errorf("Sales (ScopeOwn) HARUS nol baris utk lead milik Jun walau owner_search cocok (F3), got %d: %s", len(results), out)
	}
}

// TestJenaSearchLeads_F4EstValueMaskedForRoleWithoutArrGrant: cermin
// TestJenaSearchDeals_F4AmountMaskedForRoleWithoutArrGrant utk estimated_value.
func TestJenaSearchLeads_F4EstValueMaskedForRoleWithoutArrGrant(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "jenanoarrsales", Obj: "crm:leads", Act: "read"},
	)
	l := env.seedLead(t, "Lead NoARR", &uid)
	if _, err := env.h.Pool.Exec(t.Context(),
		`UPDATE leads SET estimated_value=$1 WHERE id=$2`, "3000000", l.ID); err != nil {
		t.Fatalf("set estimated_value: %v", err)
	}

	var out string
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", "owner", false,
			env.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, "jenanoarrsales")
		session.SetBusinessDataScope(ctx, authz.DataScopeAll)
		var err error
		out, err = env.h.jenaSearchLeads(withQueries(ctx, env.q), json.RawMessage(`{"query":"","owner_search":""}`))
		if err != nil {
			t.Fatalf("jenaSearchLeads: %v", err)
		}
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(out, flsHidden) {
		t.Errorf("estimated_value harus tersamar utk peran tanpa grant arr (F4), got %s", out)
	}
	if strings.Contains(out, "3.000.000") || strings.Contains(out, "3000000") {
		t.Errorf("estimated_value TAK BOLEH bocor utk peran tanpa grant arr:\n%s", out)
	}
}
