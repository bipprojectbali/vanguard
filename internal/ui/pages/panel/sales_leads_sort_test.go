package panel

import (
	"strings"
	"testing"
)

// sales_leads_sort_test.go — BL-157b: kontrak URL header sortable Leads, klon
// persis subscriptions_sort_test.go, digeneralisasi ke SEMUA 7 kolom tabel.
// Mekanisme href/toggle/indikator identik lintas kolom, jadi table-driven atas
// leadSortColumns. Murni-data (tanpa DB); penyaringan/urutan hasil sungguhan
// diuji di sisi handler (sales_leads_sort_test.go paket handler).

// leadSortColumns = pasangan slug+label PERSIS seperti dipakai leadsTable
// (lihat sales_leads.go) — satu sumber kebenaran utk test table-driven di bawah.
var leadSortColumns = []struct{ col, label string }{
	{"code", "Kode"},
	{"name", "Lead"},
	{"source", "Sumber"},
	{"status", "Status"},
	{"rating", "Rating"},
	{"value", "Estimasi"},
	{"owner", "Pemilik"},
}

// Header kolom non-aktif → tautan MENGAJAK sort=<col>&dir=asc (klik pertama
// selalu naik), tanpa arah panah — utk SEMUA kolom.
func TestLeadsList_SortHeader_Inactive(t *testing.T) {
	out := renderLeads(t, LeadsList(LeadsListView{
		Base: "/w/desa", Tab: "my",
		Items: []LeadRow{{ID: 1, LeadName: "Lead Cocok"}},
	}))
	if strings.Contains(out, "▲") || strings.Contains(out, "▼") {
		t.Errorf("tanpa sort aktif, tak boleh ada indikator arah pada kolom mana pun:\n%s", out)
	}
	for _, c := range leadSortColumns {
		t.Run(c.col, func(t *testing.T) {
			want := `href="/w/desa/leads?dir=asc&amp;sort=` + c.col + `&amp;tab=my"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q non-aktif harus mengajak sort=%s&dir=asc:\n%s", c.label, c.col, out)
			}
		})
	}
}

// Header kolom aktif ASC → indikator ▲ & klik berikutnya toggle ke desc.
func TestLeadsList_SortHeader_ActiveAscTogglesToDesc(t *testing.T) {
	for _, c := range leadSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, LeadsList(LeadsListView{
				Base: "/w/desa", Tab: "my", Sort: c.col, Dir: "asc",
				Items: []LeadRow{{ID: 1, LeadName: "Lead Cocok"}},
			}))
			if !strings.Contains(out, c.label+" ▲") {
				t.Errorf("header %q aktif-asc harus tampilkan indikator ▲:\n%s", c.label, out)
			}
			want := `href="/w/desa/leads?dir=desc&amp;sort=` + c.col + `&amp;tab=my"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-asc harus toggle ke dir=desc:\n%s", c.label, out)
			}
		})
	}
}

// Header kolom aktif DESC → indikator ▼ & klik berikutnya toggle balik ke asc.
func TestLeadsList_SortHeader_ActiveDescTogglesToAsc(t *testing.T) {
	for _, c := range leadSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, LeadsList(LeadsListView{
				Base: "/w/desa", Tab: "my", Sort: c.col, Dir: "desc",
				Items: []LeadRow{{ID: 1, LeadName: "Lead Cocok"}},
			}))
			if !strings.Contains(out, c.label+" ▼") {
				t.Errorf("header %q aktif-desc harus tampilkan indikator ▼:\n%s", c.label, out)
			}
			want := `href="/w/desa/leads?dir=asc&amp;sort=` + c.col + `&amp;tab=my"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-desc harus toggle ke dir=asc:\n%s", c.label, out)
			}
		})
	}
}

// Tautan sort header SENDIRI tak membawa after/trail (submit baru = reset ke
// hal 1), bahkan saat halaman ini sedang di halaman ke-2. Cukup diverifikasi
// lintas 2 kolom representatif (mekanisme href sama utk semua).
func TestLeadsList_SortHeader_NoAfterTrail(t *testing.T) {
	for _, c := range []struct{ col, label string }{{"name", "Lead"}, {"owner", "Pemilik"}} {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, LeadsList(LeadsListView{
				Base: "/w/desa", Sort: c.col, Dir: "asc",
				Items:      []LeadRow{{ID: 1, LeadName: "Lead Cocok"}},
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
// existing, bukan menggantikan) — mirror keputusan #2 BL-157a.
func TestLeadsList_SortThreadsThroughTabAndSearch(t *testing.T) {
	out := renderLeads(t, LeadsList(LeadsListView{
		Base:  "/w/desa",
		Tab:   "my",
		Sort:  "name",
		Dir:   "desc",
		Items: []LeadRow{{ID: 1, LeadName: "Lead Cocok"}},
	}))
	if !strings.Contains(out, `<input type="hidden" name="sort" value="name">`) {
		t.Errorf("form cari harus menjaga sort aktif via input tersembunyi:\n%s", out)
	}
	if !strings.Contains(out, `<input type="hidden" name="dir" value="desc">`) {
		t.Errorf("form cari harus menjaga dir aktif via input tersembunyi:\n%s", out)
	}
	// Tab "Unqualified" harus tetap membawa sort/dir aktif saat berpindah tab.
	if !strings.Contains(out, `href="/w/desa/leads?dir=desc&amp;sort=name&amp;tab=unqualified"`) {
		t.Errorf("tab lead harus mempertahankan sort=name&dir=desc:\n%s", out)
	}
}

// Pager membawa sort/dir aktif (bukan cuma tab/q) — tanpa ini pindah halaman
// diam-diam membuang sort aktif.
func TestLeadsList_SortThreadsThroughPager(t *testing.T) {
	out := renderLeads(t, LeadsList(LeadsListView{
		Base: "/w/desa", Sort: "name", Dir: "asc",
		Items:      []LeadRow{{ID: 1, LeadName: "Lead Cocok"}},
		NextCursor: "abcd12_2",
	}))
	for _, want := range []string{"sort=name", "dir=asc", "after=abcd12_2"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q:\n%s", want, out)
		}
	}
}
