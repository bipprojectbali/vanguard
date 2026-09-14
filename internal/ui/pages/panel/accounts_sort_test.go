package panel

import (
	"strings"
	"testing"
)

// accounts_sort_test.go — BL-157c: kontrak URL header sortable Accounts, klon
// persis sales_leads_sort_test.go, digeneralisasi ke 6 kolom tabel Desa.
// Mekanisme href/toggle/indikator identik lintas kolom, jadi table-driven atas
// accountSortColumns. Murni-data (tanpa DB); penyaringan/urutan hasil sungguhan
// diuji di sisi handler (accounts_sort_test.go paket handler).

// accountSortColumns = pasangan slug+label PERSIS seperti dipakai accountsTable
// (lihat accounts_table.go) — satu sumber kebenaran utk test table-driven di bawah.
var accountSortColumns = []struct{ col, label string }{
	{"village", "Desa"},
	{"type", "Tipe"},
	{"regency", "Kab/Kota"},
	{"province", "Provinsi"},
	{"owner", "Owner"},
	{"csm", "CS"},
}

// Header kolom non-aktif → tautan MENGAJAK sort=<col>&dir=asc (klik pertama
// selalu naik), tanpa arah panah — utk SEMUA kolom. accountsListHref merakit
// param urutan view→q→sort→dir (bukan alfabetis Encode()), jadi tautan yang
// dicek TAK memakai view (base saja).
func TestAccountsList_SortHeader_Inactive(t *testing.T) {
	out := renderLeads(t, AccountsList(AccountsListView{
		Base:  "/w/desa",
		Items: []AccountRow{{ID: 1, VillageName: "Desa Cocok"}},
	}))
	if strings.Contains(out, "▲") || strings.Contains(out, "▼") {
		t.Errorf("tanpa sort aktif, tak boleh ada indikator arah pada kolom mana pun:\n%s", out)
	}
	for _, c := range accountSortColumns {
		t.Run(c.col, func(t *testing.T) {
			want := `href="/w/desa/accounts?sort=` + c.col + `&amp;dir=asc"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q non-aktif harus mengajak sort=%s&dir=asc:\n%s", c.label, c.col, out)
			}
		})
	}
}

// Header kolom aktif ASC → indikator ▲ & klik berikutnya toggle ke desc.
func TestAccountsList_SortHeader_ActiveAscTogglesToDesc(t *testing.T) {
	for _, c := range accountSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, AccountsList(AccountsListView{
				Base: "/w/desa", Sort: c.col, Dir: "asc",
				Items: []AccountRow{{ID: 1, VillageName: "Desa Cocok"}},
			}))
			if !strings.Contains(out, c.label+" ▲") {
				t.Errorf("header %q aktif-asc harus tampilkan indikator ▲:\n%s", c.label, out)
			}
			want := `href="/w/desa/accounts?sort=` + c.col + `&amp;dir=desc"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-asc harus toggle ke dir=desc:\n%s", c.label, out)
			}
		})
	}
}

// Header kolom aktif DESC → indikator ▼ & klik berikutnya toggle balik ke asc.
func TestAccountsList_SortHeader_ActiveDescTogglesToAsc(t *testing.T) {
	for _, c := range accountSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, AccountsList(AccountsListView{
				Base: "/w/desa", Sort: c.col, Dir: "desc",
				Items: []AccountRow{{ID: 1, VillageName: "Desa Cocok"}},
			}))
			if !strings.Contains(out, c.label+" ▼") {
				t.Errorf("header %q aktif-desc harus tampilkan indikator ▼:\n%s", c.label, out)
			}
			want := `href="/w/desa/accounts?sort=` + c.col + `&amp;dir=asc"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-desc harus toggle ke dir=asc:\n%s", c.label, out)
			}
		})
	}
}

// Tautan sort header SENDIRI tak membawa after/trail (submit baru = reset ke
// hal 1), bahkan saat halaman ini sedang di halaman ke-2. Cukup diverifikasi
// lintas 2 kolom representatif (mekanisme href sama utk semua).
func TestAccountsList_SortHeader_NoAfterTrail(t *testing.T) {
	for _, c := range []struct{ col, label string }{{"village", "Desa"}, {"owner", "Owner"}} {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, AccountsList(AccountsListView{
				Base: "/w/desa", Sort: c.col, Dir: "asc",
				Items:      []AccountRow{{ID: 1, VillageName: "Desa Cocok"}},
				NextCursor: "abcd12_2", After: "1234ab_1", Trail: "-",
			}))
			marker := c.label + " ▲"
			i := strings.Index(out, marker)
			if i < 0 {
				t.Fatalf("header %q aktif tak ditemukan:\n%s", c.label, out)
			}
			headerHTML := out[:i]
			j := strings.LastIndex(headerHTML, `href="`)
			if j < 0 {
				t.Fatalf("href header %q tak ditemukan:\n%s", c.label, out)
			}
			href := headerHTML[j:]
			if strings.Contains(href, "after=") || strings.Contains(href, "trail=") {
				t.Errorf("tautan header sort tak boleh membawa after/trail:\n%s", href)
			}
		})
	}
}

// Tab & kotak cari mempertahankan sort/dir aktif (berdampingan dengan filter
// existing, bukan menggantikan) — mirror keputusan #2 BL-157a/157b.
func TestAccountsList_SortThreadsThroughTabAndSearch(t *testing.T) {
	out := renderLeads(t, AccountsList(AccountsListView{
		Base:       "/w/desa",
		ShowTabs:   true,
		ActiveView: AccViewMy,
		Sort:       "village",
		Dir:        "desc",
		Items:      []AccountRow{{ID: 1, VillageName: "Desa Cocok"}},
	}))
	if !strings.Contains(out, `<input type="hidden" name="sort" value="village">`) {
		t.Errorf("form cari harus menjaga sort aktif via input tersembunyi:\n%s", out)
	}
	if !strings.Contains(out, `<input type="hidden" name="dir" value="desc">`) {
		t.Errorf("form cari harus menjaga dir aktif via input tersembunyi:\n%s", out)
	}
	// Tab "Belum ada Owner" harus tetap membawa sort/dir aktif saat berpindah tab.
	if !strings.Contains(out, `href="/w/desa/accounts?view=unowned&amp;sort=village&amp;dir=desc"`) {
		t.Errorf("tab desa harus mempertahankan sort=village&dir=desc:\n%s", out)
	}
}

// Pager membawa sort/dir aktif (bukan cuma view/q) — tanpa ini pindah halaman
// diam-diam membuang sort aktif.
func TestAccountsList_SortThreadsThroughPager(t *testing.T) {
	out := renderLeads(t, AccountsList(AccountsListView{
		Base: "/w/desa", Sort: "village", Dir: "asc",
		Items:      []AccountRow{{ID: 1, VillageName: "Desa Cocok"}},
		NextCursor: "abcd12_2",
	}))
	for _, want := range []string{"sort=village", "dir=asc", "after=abcd12_2"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q:\n%s", want, out)
		}
	}
}

// Reset ("kembali ke awal" / "hapus pencarian") membuang sort/dir SEKALIGUS
// query — bukan cuma query (lihat emptyAccounts: "kembali ke awal" = benar-benar
// awal, mirror emptyLeads).
func TestAccountsList_SortDroppedOnReset(t *testing.T) {
	out := renderLeads(t, AccountsList(AccountsListView{
		Base: "/w/desa", ActiveView: AccViewAll, Sort: "village", Dir: "asc",
		Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, `href="/w/desa/accounts" class="btn btn-ghost btn-sm min-h-11">« Kembali ke awal`) {
		t.Errorf("tautan kembali ke awal harus membuang sort/dir (dan query):\n%s", out)
	}
}
