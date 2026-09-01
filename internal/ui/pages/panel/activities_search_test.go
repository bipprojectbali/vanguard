package panel

import (
	"strings"
	"testing"
)

// activities_search_test.go — regresi BL-6 slice 5: kotak pencarian + penerusan
// ?q= pada KEDUA daftar aktivitas polimorfik — ActivitiesList (/activities) &
// AllActivitiesList (/activity-log). Murni-data (tanpa DB): yang dijaga KONTRAK
// URL — kotak cari native GET (bookmarkable, buang after), q diisi ulang, dan
// SEMUA jalur navigasi (pager "Berikutnya", tautan reset) membawa/menjaga q agar
// pencarian bertahan lintas halaman keyset. Penyempitan hasil & batas F3 diuji di
// sisi handler (activities_search_test.go paket handler). Memakai ulang
// assertSearchBox/assertEscapedQ/renderLeads (paket sama).

func TestActivitiesList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, ActivitiesList(ActivitiesListView{
		Base:         "/w/desa",
		Query:        "kali muara", // ada spasi → wajib ter-escape
		TargetFilter: "account:42",
		Items:        []ActivityRow{{ID: 1, Subject: "Aktivitas Cocok"}},
		NextCursor:   "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/activities", "kali muara")
	// TargetFilter aktif dijaga lewat input tersembunyi saat submit (mode per-entitas).
	if !strings.Contains(out, `<input type="hidden" name="target" value="account:42">`) {
		t.Errorf("form cari harus menjaga target aktif via input tersembunyi target=account:42:\n%s", out)
	}
	// Pager membawa after + target + q agar halaman 2 tak keluar dari pencarian/filter.
	for _, want := range []string{"after=99_9", "target=account:42"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q:\n%s", want, out)
		}
	}
	assertEscapedQ(t, out, "kali muara")
}

func TestActivitiesList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, ActivitiesList(ActivitiesListView{
		Base: "/w/desa", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Tak ada aktivitas yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	// Tautan reset mengosongkan q (kembali ke daftar tanpa pencarian).
	if !strings.Contains(out, `href="/w/desa/activities"`) {
		t.Errorf("tautan reset harus tanpa q:\n%s", out)
	}
}

func TestAllActivitiesList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, AllActivitiesList(AllActivitiesListView{
		Base:       "/w/desa",
		Query:      "kali muara",
		Items:      []ActivityRow{{ID: 1, Subject: "Aktivitas Cocok"}},
		NextCursor: "77_7",
	}))

	assertSearchBox(t, out, "/w/desa/activity-log", "kali muara")
	if !strings.Contains(out, "after=77_7") {
		t.Errorf("pager harus membawa after:\n%s", out)
	}
	assertEscapedQ(t, out, "kali muara")
}

func TestAllActivitiesList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, AllActivitiesList(AllActivitiesListView{
		Base: "/w/desa", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Tak ada aktivitas yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/activity-log"`) {
		t.Errorf("tautan reset harus menaut daftar tanpa q:\n%s", out)
	}
}
