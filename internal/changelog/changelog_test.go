package changelog

import "testing"

// TestReleasesNotEmpty memastikan ada minimal satu rilis — tanpa ini modal &
// badge tak punya sumber (Current() "" → badge tak pernah menyala).
func TestReleasesNotEmpty(t *testing.T) {
	if len(Releases) == 0 {
		t.Fatal("Releases kosong: modal Pembaruan tak punya isi")
	}
}

// TestCurrentIsNewest mengunci kontrak Current() = versi di indeks 0 (terbaru),
// acuan badge. Salah urut → badge membandingkan versi keliru.
func TestCurrentIsNewest(t *testing.T) {
	if got, want := Current(), Releases[0].Version; got != want {
		t.Fatalf("Current() = %q, ingin %q (Releases[0])", got, want)
	}
}

// TestCurrentEmptyWhenNoReleases memastikan fungsi aman saat daftar kosong
// (tak panic, badge tak menyala) — dijaga lewat salinan lokal, bukan menyentuh var global.
func TestCurrentEmptyWhenNoReleases(t *testing.T) {
	saved := Releases
	t.Cleanup(func() { Releases = saved })
	Releases = nil
	if Current() != "" {
		t.Fatalf("Current() harus \"\" saat tak ada rilis, dapat %q", Current())
	}
}

// TestReleasesWellFormed menolak entri cacat: versi/tanggal kosong, atau rilis
// tanpa butir perubahan sama sekali (rilis hampa membingungkan pengguna).
func TestReleasesWellFormed(t *testing.T) {
	for i, r := range Releases {
		if r.Version == "" {
			t.Errorf("Releases[%d]: Version kosong", i)
		}
		if r.Date == "" {
			t.Errorf("Releases[%d] (v%s): Date kosong", i, r.Version)
		}
		items := 0
		for _, s := range r.Sections {
			items += len(s.Items)
		}
		if items == 0 {
			t.Errorf("Releases[%d] (v%s): tak ada butir perubahan", i, r.Version)
		}
	}
}
