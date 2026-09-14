package panel

import (
	"strings"
	"testing"
)

// dashboard_charts_test.go — BL-140: regresi grid chart per-domain Beranda.
// dashboardDomain harus merender N kartu chart-<id> + N script JSON sibling
// saat Charts terisi (membalik BL-98 khusus Beranda), dan TAK merender apa pun
// saat Charts kosong (TestDashboardBody_NoDanglingCells tak boleh rusak).

func TestDashboardDomain_RendersChartGrid(t *testing.T) {
	out := renderLeads(t, dashboardDomain(DashDomain{
		Title: "Sales",
		KPIs: []DashKPI{
			{Label: "Win Rate", Value: "72%"},
		},
		Charts: []DashChart{
			{Title: "Pipeline per Stage", ChartID: "chart-sales-pipeline", ChartJSON: `{"a":1}`},
			{Title: "Leads per Sumber", ChartID: "chart-sales-leads", ChartJSON: `{"b":2}`},
		},
	}))

	for _, id := range []string{"chart-sales-pipeline", "chart-sales-leads"} {
		if !strings.Contains(out, `id="`+id+`"`) {
			t.Errorf("harus ada kontainer id=%q:\n%s", id, out)
		}
		if !strings.Contains(out, `id="`+id+`-data"`) {
			t.Errorf("harus ada script JSON sibling id=%q-data:\n%s", id, out)
		}
	}
	// 2 chart → grid 2-kolom desktop (dashPanelCols(2)).
	if !strings.Contains(out, "md:grid-cols-2") {
		t.Errorf("2 chart harus pakai md:grid-cols-2:\n%s", out)
	}
}

func TestDashboardDomain_SingleChartFullWidth(t *testing.T) {
	out := renderLeads(t, dashboardDomain(DashDomain{
		Title: "Subscription",
		Charts: []DashChart{
			{Title: "Renewal per Bulan", ChartID: "chart-sub-renewal", ChartJSON: `{}`},
		},
	}))
	if !strings.Contains(out, `id="chart-sub-renewal"`) {
		t.Errorf("harus ada kontainer chart-sub-renewal:\n%s", out)
	}
	if strings.Contains(out, "md:grid-cols-2") {
		t.Errorf("1 chart harus penuh-lebar, tak boleh md:grid-cols-2:\n%s", out)
	}
}

// TestDashboardDomain_NoChartsNoGrid — regresi: domain tanpa Charts (nil/kosong,
// mis. sebelum BL-141..144 mengisi, atau domain yg semua chart-nya di-skip F4)
// tak boleh merender grid chart sama sekali.
func TestDashboardDomain_NoChartsNoGrid(t *testing.T) {
	out := renderLeads(t, dashboardDomain(DashDomain{
		Title: "Support",
		KPIs:  []DashKPI{{Label: "Tiket Terbuka", Value: "3"}},
	}))
	if strings.Contains(out, "chart-") {
		t.Errorf("domain tanpa Charts tak boleh menyisakan elemen chart-*:\n%s", out)
	}
}
