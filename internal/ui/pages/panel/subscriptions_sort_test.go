package panel

import (
	"strings"
	"testing"
)

// subscriptions_sort_test.go — BL-157a: kontrak URL header sortable, digeneralisasi
// ke SEMUA 6 kolom tabel (bukan cuma "Desa" fondasi) — mekanisme href/toggle/
// indikator identik lintas kolom, jadi table-driven atas subSortColumns. Murni-data
// (tanpa DB); penyaringan/urutan hasil sungguhan diuji di sisi handler
// (subscriptions_sort_test.go paket handler).

// subSortColumns = pasangan slug+label PERSIS seperti dipakai subsTable (lihat
// subscriptions.go) — satu sumber kebenaran utk test table-driven di bawah.
var subSortColumns = []struct{ col, label string }{
	{"village", "Desa"},
	{"plan", "Paket"},
	{"mrr", "MRR"},
	{"status", "Masa Berlaku"},
	{"renewal", "Renewal Date"},
	{"csm", "CSM"},
}

// Header kolom non-aktif → tautan MENGAJAK sort=<col>&dir=asc (klik pertama
// selalu naik), tanpa arah panah — utk SEMUA kolom (bukan cuma village).
func TestSubList_SortHeader_Inactive(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base: "/w/desa", StatusFilter: "Active",
		Items: []SubRow{{ID: 1, Village: "Desa Cocok"}},
	}))
	if strings.Contains(out, "▲") || strings.Contains(out, "▼") {
		t.Errorf("tanpa sort aktif, tak boleh ada indikator arah pada kolom mana pun:\n%s", out)
	}
	for _, c := range subSortColumns {
		t.Run(c.col, func(t *testing.T) {
			want := `href="/w/desa/subscriptions?dir=asc&amp;sort=` + c.col + `&amp;status=Active"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q non-aktif harus mengajak sort=%s&dir=asc:\n%s", c.label, c.col, out)
			}
		})
	}
}

// Header kolom aktif ASC → indikator ▲ & klik berikutnya toggle ke desc.
func TestSubList_SortHeader_ActiveAscTogglesToDesc(t *testing.T) {
	for _, c := range subSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, SubList(SubListView{
				Base: "/w/desa", StatusFilter: "Active", Sort: c.col, Dir: "asc",
				Items: []SubRow{{ID: 1, Village: "Desa Cocok"}},
			}))
			if !strings.Contains(out, c.label+" ▲") {
				t.Errorf("header %q aktif-asc harus tampilkan indikator ▲:\n%s", c.label, out)
			}
			want := `href="/w/desa/subscriptions?dir=desc&amp;sort=` + c.col + `&amp;status=Active"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-asc harus toggle ke dir=desc:\n%s", c.label, out)
			}
		})
	}
}

// Header kolom aktif DESC → indikator ▼ & klik berikutnya toggle balik ke asc.
func TestSubList_SortHeader_ActiveDescTogglesToAsc(t *testing.T) {
	for _, c := range subSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, SubList(SubListView{
				Base: "/w/desa", StatusFilter: "Active", Sort: c.col, Dir: "desc",
				Items: []SubRow{{ID: 1, Village: "Desa Cocok"}},
			}))
			if !strings.Contains(out, c.label+" ▼") {
				t.Errorf("header %q aktif-desc harus tampilkan indikator ▼:\n%s", c.label, out)
			}
			want := `href="/w/desa/subscriptions?dir=asc&amp;sort=` + c.col + `&amp;status=Active"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-desc harus toggle ke dir=asc:\n%s", c.label, out)
			}
		})
	}
}

// Tautan sort header SENDIRI tak membawa after/trail (submit baru = reset ke
// hal 1 — keputusan #3), bahkan saat halaman ini sedang di halaman ke-2. Cukup
// diverifikasi lintas 2 kolom representatif (mekanisme href sama utk semua).
func TestSubList_SortHeader_NoAfterTrail(t *testing.T) {
	for _, c := range []struct{ col, label string }{{"village", "Desa"}, {"csm", "CSM"}} {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, SubList(SubListView{
				Base: "/w/desa", StatusFilter: "Active", Sort: c.col, Dir: "asc",
				Items:      []SubRow{{ID: 1, Village: "Desa Cocok"}},
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

// Tab status & kotak cari mempertahankan sort/dir aktif (BL-157a keputusan #2:
// berdampingan dengan filter existing, bukan menggantikan).
func TestSubList_SortThreadsThroughStatusTabAndSearch(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base:         "/w/desa",
		StatusFilter: "Active",
		Statuses:     []string{"Active", "Trial"},
		Sort:         "village",
		Dir:          "desc",
		Items:        []SubRow{{ID: 1, Village: "Desa Cocok"}},
	}))
	if !strings.Contains(out, `<input type="hidden" name="sort" value="village">`) {
		t.Errorf("form cari harus menjaga sort aktif via input tersembunyi:\n%s", out)
	}
	if !strings.Contains(out, `<input type="hidden" name="dir" value="desc">`) {
		t.Errorf("form cari harus menjaga dir aktif via input tersembunyi:\n%s", out)
	}
	// Tab "Trial" harus tetap membawa sort/dir aktif saat berpindah status.
	if !strings.Contains(out, `href="/w/desa/subscriptions?dir=desc&amp;sort=village&amp;status=Trial"`) {
		t.Errorf("tab status harus mempertahankan sort=village&dir=desc:\n%s", out)
	}
}

// Pager membawa sort/dir aktif (bukan cuma status/q) — tanpa ini pindah
// halaman diam-diam membuang sort aktif.
func TestSubList_SortThreadsThroughPager(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base: "/w/desa", StatusFilter: "Active", Sort: "village", Dir: "asc",
		Items:      []SubRow{{ID: 1, Village: "Desa Cocok"}},
		NextCursor: "abcd12_2",
	}))
	for _, want := range []string{"sort=village", "dir=asc", "after=abcd12_2"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q:\n%s", want, out)
		}
	}
}
