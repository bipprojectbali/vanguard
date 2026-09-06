package panel

import (
	"strings"
	"testing"
)

// engagements_search_test.go — regresi BL-6 slice 4: kotak cari + penerusan ?q=
// daftar Engagements. Kembaran tickets_search_test.go. KONTRAK URL saja.

func TestEngagementsList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, EngagementsList(EngagementsListView{
		Base:       "/w/desa",
		Tab:        "planned",
		Query:      "kali muara",
		Items:      []EngagementRow{{ID: 1, AccountName: "Desa Cocok", Subject: "Onboarding"}},
		NextCursor: "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/engagements", "kali muara")
	// BL-68: tab & kotak cari sebaris dalam satu wrapper justify-between.
	assertTabSearchRow(t, out, "/w/desa/engagements")
	if !strings.Contains(out, `<input type="hidden" name="tab" value="planned">`) {
		t.Errorf("form cari harus menjaga tab aktif via input tersembunyi tab=planned:\n%s", out)
	}
	if !strings.Contains(out, "after=99_9") || !strings.Contains(out, "tab=planned") {
		t.Errorf("pager harus membawa after & tab:\n%s", out)
	}
	assertEscapedQ(t, out, "kali muara")
}

func TestEngagementsList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, EngagementsList(EngagementsListView{
		Base: "/w/desa", Tab: "planned", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada engagement yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/engagements?tab=planned"`) {
		t.Errorf("tautan reset harus menjaga tab tapi membuang q:\n%s", out)
	}
}
