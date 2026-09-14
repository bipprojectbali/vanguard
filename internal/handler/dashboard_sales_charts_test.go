package handler

import (
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// dashboard_sales_charts_test.go — BL-141: 3 chart inline section Sales
// (membalik BL-98 KHUSUS Beranda), semua di bawah crm:deals (gate SAMA dgn KPI
// Win Rate/Deal Tutup di atasnya). Setup reuse seedDeal/setDealStage
// (sales_quotes_test.go/sales_deals_stage_test.go) + seedLeadStatus
// (reports_sales_panels_test.go).

// seedLeadSource menaruh satu lead dgn lead_source tertentu — utk chart Leads
// per Sumber.
func (e *testEnv) seedLeadSource(t *testing.T, name string, owner int64, source string) db.Lead {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityLead)
	if err != nil {
		t.Fatalf("generate lead code: %v", err)
	}
	l, err := e.q.CreateLead(t.Context(), db.CreateLeadParams{
		TenantID:   e.tenantID,
		EntityCode: &code,
		LeadName:   name,
		LeadOwner:  &owner,
		LeadSource: &source,
		LeadStatus: "New",
		CreatedBy:  &owner,
	})
	if err != nil {
		t.Fatalf("seed lead %s: %v", name, err)
	}
	return l
}

// TestDashboardSalesCharts_RenderedForCanViewDeals: role ber-crm:deals melihat
// ketiga chart baru (pipeline, leads, win/loss) berdampingan dgn KPI yg sudah
// ada; role tanpa crm:deals (csm/support) TAK melihat satupun.
func TestDashboardSalesCharts_RenderedForCanViewDeals(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Chart Sales", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid) // Prospecting → masuk chart pipeline
	won := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, won.ID, "Closed Won")
	env.seedLeadSource(t, "Lead Web", uid, "Website")

	for _, role := range []string{"admin", "manager", "sales"} {
		t.Run(role+" melihat 3 chart Sales", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			for _, id := range []string{"chart-sales-pipeline", "chart-sales-leads", "chart-sales-winloss"} {
				if !strings.Contains(body, `id="`+id+`"`) {
					t.Errorf("role %q harus melihat %q, body:\n%s", role, id, body)
				}
			}
		})
	}

	for _, role := range []string{"csm", "support"} {
		t.Run(role+" tak melihat chart Sales", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			for _, id := range []string{"chart-sales-pipeline", "chart-sales-leads", "chart-sales-winloss"} {
				if strings.Contains(body, id) {
					t.Errorf("role %q (tanpa crm:deals) TAK boleh melihat %q", role, id)
				}
			}
		})
	}
}

// TestDashboardSalesCharts_WinLossSkippedWhenNoClosedDeals: chart Win/Loss
// dilewati SELURUHNYA (tak ada kontainer chart-sales-winloss) bila belum ada
// deal Closed Won/Lost sama sekali — donut 0/0 tak berguna. Pipeline & Leads
// tetap render (data lain memang ada).
func TestDashboardSalesCharts_WinLossSkippedWhenNoClosedDeals(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Belum Closed", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid) // Prospecting saja, belum ada won/lost

	body := env.dashboardBody(t, uid, "owner", "admin")
	if strings.Contains(body, "chart-sales-winloss") {
		t.Errorf("tanpa deal closed won/lost, chart-sales-winloss TAK boleh muncul, body:\n%s", body)
	}
	if !strings.Contains(body, "chart-sales-pipeline") {
		t.Errorf("chart-sales-pipeline harus tetap muncul (deal terbuka ada), body:\n%s", body)
	}
}

// TestDashboardSalesCharts_LeadsScopedByOwnership (F3): chart Leads per Sumber
// pakai LeadsListFilterFor — TERPISAH dari DealsListFilterFor milik pipeline/
// win-loss. Dibuktikan lewat JSON tertanam (nama sumber muncul/tak muncul
// sesuai kepemilikan), bukan cuma id kontainer. Assertion pakai pola JSON
// `"name":"<sumber>"` (bukan substring polos) — "Website" juga muncul apa
// adanya di copy changelog global setiap halaman ("Field \"Website\" desa..."),
// substring polos jadi false-positive.
func TestDashboardSalesCharts_LeadsScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherleadchart@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Lead Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Lead Other", &other, nil, nil)
	env.seedDeal(t, accMine.ID, &uid)
	env.seedDeal(t, accOther.ID, &other)
	env.seedLeadSource(t, "Lead Mine", uid, "Referral")
	env.seedLeadSource(t, "Lead Other", other, "Website")

	own := env.dashboardBody(t, uid, "owner", "sales")
	if !strings.Contains(own, `"name":"Referral"`) {
		t.Errorf("sales own-scope harus melihat sumber leadnya sendiri (Referral), body:\n%s", own)
	}
	if strings.Contains(own, `"name":"Website"`) {
		t.Errorf("sales own-scope TAK boleh melihat sumber lead milik orang lain (Website), body:\n%s", own)
	}

	all := env.dashboardBody(t, uid, "owner", "manager")
	if !strings.Contains(all, `"name":"Referral"`) || !strings.Contains(all, `"name":"Website"`) {
		t.Errorf("manager all-scope harus melihat kedua sumber lead, body:\n%s", all)
	}
}
