package panel

import (
	"strings"
	"testing"
)

// cs_trainings_search_test.go — regresi BL-6 slice 4: kotak cari + penerusan ?q=
// daftar Trainings. Kembaran tickets_search_test.go. KONTRAK URL saja.

func TestCSTrainingsList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, CSTrainingsList(CSTrainingsListView{
		Base:       "/w/desa",
		Tab:        "scheduled",
		Query:      "kali muara",
		Items:      []CSTrainingRow{{ID: 1, AccountName: "Desa Cocok"}},
		NextCursor: "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/trainings", "kali muara")
	if !strings.Contains(out, `<input type="hidden" name="tab" value="scheduled">`) {
		t.Errorf("form cari harus menjaga tab aktif via input tersembunyi tab=scheduled:\n%s", out)
	}
	if !strings.Contains(out, "after=99_9") || !strings.Contains(out, "tab=scheduled") {
		t.Errorf("pager harus membawa after & tab:\n%s", out)
	}
	assertEscapedQ(t, out, "kali muara")
}

func TestCSTrainingsList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, CSTrainingsList(CSTrainingsListView{
		Base: "/w/desa", Tab: "scheduled", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada training yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/trainings?tab=scheduled"`) {
		t.Errorf("tautan reset harus menjaga tab tapi membuang q:\n%s", out)
	}
}
