package panel

import (
	"strings"
	"testing"
)

// sales_deals_sort_test.go — BL-157e: kontrak URL header sortable Deals, klon
// persis sales_leads_sort_test.go, digeneralisasi ke 7 kolom tabel Deal.
// Mekanisme href/toggle/indikator identik lintas kolom, jadi table-driven atas
// dealSortColumns. Murni-data (tanpa DB); penyaringan/urutan hasil sungguhan
// diuji di sisi handler (sales_deals_sort_test.go paket handler).

// dealSortColumns = pasangan slug+label PERSIS seperti dipakai dealsTable
// (lihat sales_deals_body.go) — satu sumber kebenaran utk test table-driven
// di bawah.
var dealSortColumns = []struct{ col, label string }{
	{"code", "Kode"},
	{"name", "Deal"},
	{"stage", "Tahap"},
	{"amount", "Nilai"},
	{"probability", "Peluang"},
	{"close", "Perkiraan Tutup"},
	{"owner", "Pemilik"},
}

// dealTableView = DealPipelineView minimal yang merender tampilan Tabel (View
// harus "table" & Items tak kosong, lihat DealPipeline).
func dealTableView(v DealPipelineView) DealPipelineView {
	v.View = "table"
	if v.Items == nil {
		v.Items = []DealRow{{ID: 1, DealName: "Deal Cocok"}}
	}
	return v
}

// Header kolom non-aktif → tautan MENGAJAK sort=<col>&dir=asc (klik pertama
// selalu naik), tanpa arah panah — utk SEMUA kolom.
func TestDealsList_SortHeader_Inactive(t *testing.T) {
	out := renderLeads(t, DealPipeline(dealTableView(DealPipelineView{Base: "/w/desa"})))
	if strings.Contains(out, "▲") || strings.Contains(out, "▼") {
		t.Errorf("tanpa sort aktif, tak boleh ada indikator arah pada kolom mana pun:\n%s", out)
	}
	for _, c := range dealSortColumns {
		t.Run(c.col, func(t *testing.T) {
			want := `href="/w/desa/deals?dir=asc&amp;sort=` + c.col + `&amp;view=table"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q non-aktif harus mengajak sort=%s&dir=asc:\n%s", c.label, c.col, out)
			}
		})
	}
}

// Header kolom aktif ASC → indikator ▲ & klik berikutnya toggle ke desc.
func TestDealsList_SortHeader_ActiveAscTogglesToDesc(t *testing.T) {
	for _, c := range dealSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, DealPipeline(dealTableView(DealPipelineView{
				Base: "/w/desa", Sort: c.col, Dir: "asc",
			})))
			if !strings.Contains(out, c.label+" ▲") {
				t.Errorf("header %q aktif-asc harus tampilkan indikator ▲:\n%s", c.label, out)
			}
			want := `href="/w/desa/deals?dir=desc&amp;sort=` + c.col + `&amp;view=table"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-asc harus toggle ke dir=desc:\n%s", c.label, out)
			}
		})
	}
}

// Header kolom aktif DESC → indikator ▼ & klik berikutnya toggle balik ke asc.
func TestDealsList_SortHeader_ActiveDescTogglesToAsc(t *testing.T) {
	for _, c := range dealSortColumns {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, DealPipeline(dealTableView(DealPipelineView{
				Base: "/w/desa", Sort: c.col, Dir: "desc",
			})))
			if !strings.Contains(out, c.label+" ▼") {
				t.Errorf("header %q aktif-desc harus tampilkan indikator ▼:\n%s", c.label, out)
			}
			want := `href="/w/desa/deals?dir=asc&amp;sort=` + c.col + `&amp;view=table"`
			if !strings.Contains(out, want) {
				t.Errorf("header %q aktif-desc harus toggle ke dir=asc:\n%s", c.label, out)
			}
		})
	}
}

// Tautan sort header SENDIRI tak membawa after/trail (submit baru = reset ke
// hal 1), bahkan saat halaman ini sedang di halaman ke-2. Cukup diverifikasi
// lintas 2 kolom representatif (mekanisme href sama utk semua).
func TestDealsList_SortHeader_NoAfterTrail(t *testing.T) {
	for _, c := range []struct{ col, label string }{{"name", "Deal"}, {"owner", "Pemilik"}} {
		t.Run(c.col, func(t *testing.T) {
			out := renderLeads(t, DealPipeline(dealTableView(DealPipelineView{
				Base: "/w/desa", Sort: c.col, Dir: "asc",
				NextCursor: "abcd12_2", After: "1234ab_1", Trail: "-",
			})))
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

// Toggle Semua/Deal Saya & kotak cari mempertahankan sort/dir aktif
// (berdampingan dengan filter existing, bukan menggantikan) — mirror
// keputusan #2 BL-157a/b/c/d.
func TestDealsList_SortThreadsThroughMineToggleAndSearch(t *testing.T) {
	out := renderLeads(t, DealPipeline(dealTableView(DealPipelineView{
		Base: "/w/desa", ShowMineToggle: true, Sort: "name", Dir: "desc",
	})))
	if !strings.Contains(out, `<input type="hidden" name="sort" value="name">`) {
		t.Errorf("form cari harus menjaga sort aktif via input tersembunyi:\n%s", out)
	}
	if !strings.Contains(out, `<input type="hidden" name="dir" value="desc">`) {
		t.Errorf("form cari harus menjaga dir aktif via input tersembunyi:\n%s", out)
	}
	// Toggle "Deal Saya" harus tetap membawa sort/dir aktif saat berpindah sumbu.
	if !strings.Contains(out, `href="/w/desa/deals?dir=desc&amp;mine=1&amp;sort=name&amp;view=table"`) {
		t.Errorf("toggle Deal Saya harus mempertahankan sort=name&dir=desc:\n%s", out)
	}
}

// Pager membawa sort/dir aktif (bukan cuma stage/mine/q) — tanpa ini pindah
// halaman diam-diam membuang sort aktif.
func TestDealsList_SortThreadsThroughPager(t *testing.T) {
	out := renderLeads(t, DealPipeline(dealTableView(DealPipelineView{
		Base: "/w/desa", Sort: "name", Dir: "asc",
		NextCursor: "abcd12_2",
	})))
	for _, want := range []string{"sort=name", "dir=asc", "after=abcd12_2"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q:\n%s", want, out)
		}
	}
}

// Reset ("kembali ke awal") membuang sort/dir SEKALIGUS query — bukan cuma
// query (mirror emptyLeads/emptyAccounts: "kembali ke awal" = benar-benar awal).
func TestDealsList_SortDroppedOnReset(t *testing.T) {
	out := renderLeads(t, DealPipeline(DealPipelineView{
		Base: "/w/desa", View: "table", Query: "cari", Sort: "name", Dir: "asc",
		Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, `href="/w/desa/deals?view=table" class="btn btn-ghost btn-sm min-h-11">« Kembali ke awal`) {
		t.Errorf("tautan kembali ke awal harus membuang sort/dir (dan query):\n%s", out)
	}
}
