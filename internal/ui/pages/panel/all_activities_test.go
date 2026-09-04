package panel

import (
	"strings"
	"testing"
)

// all_activities_test.go — regresi BL-42: halaman Activities global (/activity-log)
// memakai pola 1-form BL-19 — SATU tombol "Tambah Aktivitas" → /activities/new
// (jenis dipilih di dropdown form), BUKAN 3 tombol per-kind lama (Tugas/Panggilan/
// Catatan → ?kind=X) yang hanya menjangkau 3 dari 5 kind. Murni-data (tanpa DB):
// yang dijaga = kontrak markup tombol. Gate tulis (v.CanWrite) diuji ganda:
// tombol muncul saat true, hilang saat false. Memakai ulang renderLeads (paket sama).

// TestAllActivitiesList_SingleAddButton: satu tombol "Tambah Aktivitas" → tautan
// /activities/new tanpa ?kind=, dan TIDAK ada lagi tombol per-kind lama.
func TestAllActivitiesList_SingleAddButton(t *testing.T) {
	out := renderLeads(t, AllActivitiesList(AllActivitiesListView{
		Base:     "/w/desa",
		CanWrite: true,
	}))

	if !strings.Contains(out, "Tambah Aktivitas") {
		t.Errorf("harus ada satu tombol \"Tambah Aktivitas\":\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/activities/new"`) {
		t.Errorf("tombol harus menautkan ke /activities/new (jenis dipilih di form):\n%s", out)
	}
	// Pola LAMA 3-tombol per-kind harus HILANG (§17 dead code + inkonsistensi UX).
	if strings.Contains(out, "?kind=") {
		t.Errorf("tombol per-kind lama (?kind=) tak boleh ada lagi:\n%s", out)
	}
	for _, gone := range []string{">Tugas<", ">Panggilan<", ">Catatan<"} {
		if strings.Contains(out, gone) {
			t.Errorf("tombol per-kind lama %q tak boleh ada lagi:\n%s", gone, out)
		}
	}
}

// TestAllActivitiesList_AddButtonGatedByCanWrite: tanpa izin tulis, tombol tambah
// tak dirender (gate v.CanWrite).
func TestAllActivitiesList_AddButtonGatedByCanWrite(t *testing.T) {
	out := renderLeads(t, AllActivitiesList(AllActivitiesListView{
		Base:     "/w/desa",
		CanWrite: false,
	}))
	if strings.Contains(out, "Tambah Aktivitas") {
		t.Errorf("tombol tambah tak boleh muncul saat CanWrite=false:\n%s", out)
	}
}
