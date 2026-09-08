package ui

import (
	"strings"
	"testing"

	"go_starter/internal/changelog"
)

// TestChangelogButtonRendersVersion memastikan tombol menanam versi aktif di
// data-app-version (dibaca changelog.js untuk keputusan badge) + penanda tombol.
func TestChangelogButtonRendersVersion(t *testing.T) {
	out := renderNode(t, ChangelogButton("1.2.3"))
	for _, want := range []string{
		`data-changelog-btn="true"`,
		`data-app-version="1.2.3"`,
		`data-changelog-badge="true"`,
		"Pembaruan",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ChangelogButton tak memuat %q\n%s", want, out)
		}
	}
}

// TestChangelogButtonEmptyVersion memastikan tanpa rilis (version "") tombol tak
// dirender — tak ada tombol menuju modal kosong.
func TestChangelogButtonEmptyVersion(t *testing.T) {
	if out := renderNode(t, ChangelogButton("")); strings.Contains(out, "data-changelog-btn") {
		t.Errorf("ChangelogButton(\"\") harus kosong, dapat: %s", out)
	}
}

// TestChangelogModalRendersReleases memastikan modal menampilkan tiap versi,
// tanggal, dan butir perubahan.
func TestChangelogModalRendersReleases(t *testing.T) {
	releases := []changelog.Release{
		{
			Version: "1.0.0",
			Date:    "2026-09-08",
			Sections: []changelog.Section{
				{Title: "Baru", Items: []string{"Fitur pertama", "Fitur kedua"}},
			},
		},
	}
	out := renderNode(t, ChangelogModal(releases))
	for _, want := range []string{"v1.0.0", "2026-09-08", "Baru", "Fitur pertama", "Fitur kedua"} {
		if !strings.Contains(out, want) {
			t.Errorf("ChangelogModal tak memuat %q", want)
		}
	}
}

// TestChangelogModalEmpty memastikan slice kosong → modal tak dirender.
func TestChangelogModalEmpty(t *testing.T) {
	if out := renderNode(t, ChangelogModal(nil)); strings.TrimSpace(out) != "" {
		t.Errorf("ChangelogModal(nil) harus kosong, dapat: %s", out)
	}
}
