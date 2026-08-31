package panel

import (
	"strings"
	"testing"
)

// success_plans_search_test.go — regresi BL-6 slice 4: kotak cari + penerusan ?q=
// daftar Success Plans. KONTRAK URL saja (murni-data). Tab dari handler (v.Tabs).

func TestSuccessPlansList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, SuccessPlansList(SuccessPlansListView{
		Base:       "/w/desa",
		Tab:        "Active",
		Tabs:       []SuccessPlanTab{{Key: "", Label: "Semua"}, {Key: "Active", Label: "Aktif"}},
		Query:      "kali muara",
		Items:      []SuccessPlanRow{{ID: 1, AccountName: "Desa Cocok", PlanName: "Adopsi"}},
		NextCursor: "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/success-plans", "kali muara")
	if !strings.Contains(out, `<input type="hidden" name="tab" value="Active">`) {
		t.Errorf("form cari harus menjaga tab aktif via input tersembunyi tab=Active:\n%s", out)
	}
	if !strings.Contains(out, "after=99_9") || !strings.Contains(out, "tab=Active") {
		t.Errorf("pager harus membawa after & tab:\n%s", out)
	}
	assertEscapedQ(t, out, "kali muara")
}

func TestSuccessPlansList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, SuccessPlansList(SuccessPlansListView{
		Base: "/w/desa", Tab: "Active",
		Tabs:  []SuccessPlanTab{{Key: "Active", Label: "Aktif"}},
		Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada success plan yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/success-plans?tab=Active"`) {
		t.Errorf("tautan reset harus menjaga tab tapi membuang q:\n%s", out)
	}
}
