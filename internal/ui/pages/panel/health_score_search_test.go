package panel

import (
	"strings"
	"testing"
)

// health_score_search_test.go — regresi BL-6 slice 4: kotak cari + penerusan ?q=
// daftar Health Score. Satu-satunya kolom teks yang TAMPIL = nama desa (kunci
// cari). KONTRAK URL saja (murni-data). Tab aktif = ActiveTab.

func TestHealthScoreList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, HealthScoreList(HealthScoreListView{
		Base:       "/w/desa",
		ActiveTab:  "sehat",
		Query:      "kali muara",
		Rows:       []HealthScoreRowView{{ID: 1, AccountName: "Desa Cocok", Score: "85"}},
		NextCursor: "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/health-scores", "kali muara")
	if !strings.Contains(out, `<input type="hidden" name="tab" value="sehat">`) {
		t.Errorf("form cari harus menjaga tab aktif via input tersembunyi tab=sehat:\n%s", out)
	}
	if !strings.Contains(out, "after=99_9") || !strings.Contains(out, "tab=sehat") {
		t.Errorf("pager harus membawa after & tab:\n%s", out)
	}
	assertEscapedQ(t, out, "kali muara")
}

func TestHealthScoreList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, HealthScoreList(HealthScoreListView{
		Base: "/w/desa", ActiveTab: "sehat", Query: "zzz", Rows: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada desa yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/health-scores?tab=sehat"`) {
		t.Errorf("tautan reset harus menjaga tab tapi membuang q:\n%s", out)
	}
}
