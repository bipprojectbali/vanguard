package panel

import (
	"strings"
	"testing"
)

// accounts_search_test.go — regresi BL-6: kotak pencarian daftar desa + penerusan
// param ?q= ke navigasi. Yang dijaga di sini adalah kontrak URL view (murni-data,
// tanpa DB): input search selalu dirender & mengisi ulang kueri aktif, dan SEMUA
// jalur navigasi (tab, pager "Berikutnya", tautan reset/kembali) membawa q agar
// pencarian bertahan lintas halaman/tab. q di-QueryEscape (aman untuk spasi &
// karakter khusus). Penyempitan hasil & batas F3 diuji di sisi handler.

// accountsSearchView = daftar desa dengan pencarian aktif + halaman berikutnya,
// cukup untuk memeriksa kotak cari & penerusan q ke tab/pager.
func accountsSearchView() AccountsListView {
	return AccountsListView{
		Base:       "/w/desa",
		Items:      []AccountRow{{ID: 7, VillageName: "Desa Sukamaju"}},
		CanWrite:   true,
		ShowTabs:   true,
		ActiveView: AccViewMy,
		Query:      "kali muara", // ada spasi → wajib ter-escape di URL
		NextCursor: "123_45",
	}
}

// TestAccountsList_SearchBoxRendered — kotak cari selalu ada; input mengisi ulang
// kueri aktif (agar tak hilang saat reload) dan mobile-first (text-base + tap 44px).
func TestAccountsList_SearchBoxRendered(t *testing.T) {
	out := renderLeads(t, AccountsList(accountsSearchView()))

	for _, want := range []string{
		`name="q"`,           // field pencarian
		`type="search"`,      // semantik native (tombol clear)
		`value="kali muara"`, // kueri aktif diisi ulang
		`method="get"`,       // form GET → bookmarkable, buang after (reset paging)
		`action="/w/desa/accounts"`,
		"text-base", // ≥16px → iOS tak auto-zoom
		"min-h-11",  // tap target ≥44px
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kotak cari harus memuat %q:\n%s", want, out)
		}
	}

	// Tab aktif dipertahankan lewat input tersembunyi saat submit.
	if !strings.Contains(out, `<input type="hidden" name="view" value="my">`) {
		t.Errorf("form cari harus menjaga tab aktif via input tersembunyi view=my:\n%s", out)
	}
}

// TestAccountsList_SearchThreadsIntoPager — tombol "Berikutnya" (keyset) wajib
// membawa q (ter-escape) DAN after; tanpa itu, halaman 2 melebar kembali ke
// seluruh desa alih-alih melanjutkan hasil pencarian.
func TestAccountsList_SearchThreadsIntoPager(t *testing.T) {
	out := renderLeads(t, AccountsList(accountsSearchView()))

	// Spasi → %20 atau + (QueryEscape memakai +); terima keduanya.
	if !strings.Contains(out, "after=123_45") {
		t.Errorf("pager harus membawa cursor after:\n%s", out)
	}
	if !strings.Contains(out, "q=kali+muara") && !strings.Contains(out, "q=kali%20muara") {
		t.Errorf("pager harus membawa q ter-escape (kali muara):\n%s", out)
	}
	if !strings.Contains(out, "view=my") {
		t.Errorf("pager harus membawa tab aktif view=my:\n%s", out)
	}
}

// TestAccountsList_SearchThreadsIntoTabs — pindah tab tak boleh menjatuhkan
// pencarian: tiap tautan tab membawa q. (Param after SENGAJA tak ikut — tab mulai
// dari halaman pertama.)
func TestAccountsList_SearchThreadsIntoTabs(t *testing.T) {
	out := renderLeads(t, AccountsList(accountsSearchView()))

	if !strings.Contains(out, "q=kali+muara") && !strings.Contains(out, "q=kali%20muara") {
		t.Errorf("tautan tab harus membawa q ter-escape:\n%s", out)
	}
	// Tautan "Semua Desa" (AccViewAll) → tanpa view= tapi tetap dengan q=.
	if strings.Contains(out, "view=all") {
		t.Errorf("tab Semua Desa tak boleh menulis view=all (kanonik tanpa param view):\n%s", out)
	}
}

// TestAccountsList_SearchEmptyState — pencarian tanpa hasil menampilkan pesan
// "tak cocok" (bukan "belum ada desa" yang menyesatkan) + jalan keluar menghapus
// pencarian yang mengosongkan q.
func TestAccountsList_SearchEmptyState(t *testing.T) {
	v := accountsSearchView()
	v.Items = nil     // tak ada hasil
	v.NextCursor = "" // halaman pertama pun kosong
	out := renderLeads(t, AccountsList(v))

	if !strings.Contains(out, "Tak ada desa yang cocok") {
		t.Errorf("empty-state pencarian harus berkata tak ada yang cocok:\n%s", out)
	}
	if !strings.Contains(out, "Hapus pencarian") {
		t.Errorf("empty-state pencarian harus menawarkan hapus pencarian:\n%s", out)
	}
	// Tautan "Hapus pencarian" mengosongkan q (tab aktif view=my dipertahankan,
	// q hilang). Diperiksa pada tautan ITU SENDIRI — tautan tab di atas halaman
	// memang tetap membawa q, jadi absennya q tak bisa dicek global.
	if !strings.Contains(out, `href="/w/desa/accounts?view=my" class="btn btn-ghost btn-sm min-h-11">« Hapus pencarian`) {
		t.Errorf("tautan hapus pencarian harus menuju daftar tanpa q (view=my saja):\n%s", out)
	}
}
