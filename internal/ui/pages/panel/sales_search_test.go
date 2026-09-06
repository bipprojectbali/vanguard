package panel

import (
	"strings"
	"testing"
)

// sales_search_test.go — regresi BL-6 slice 2: kotak pencarian + penerusan ?q=
// pada daftar Sales (leads, deals-tabel, kontak global, quotes). Kembaran
// accounts_search_test.go untuk keempat daftar ini. Murni-data (tanpa DB): yang
// dijaga adalah KONTRAK URL view — input search selalu dirender & mengisi ulang
// kueri aktif, dan SEMUA jalur navigasi (tab/toggle, pager "Berikutnya", tautan
// kembali) membawa q agar pencarian bertahan lintas halaman. q di-QueryEscape
// (aman untuk spasi). Penyempitan hasil & batas F3 diuji di sisi handler
// (sales_search_test.go paket handler).

// assertSearchBox memeriksa properti kotak cari yang WAJIB sama di keempat daftar:
// field pencarian native, kueri aktif diisi ulang, form GET (bookmarkable + buang
// after), dan mobile-first (text-base ≥16px, tap target ≥44px).
func assertSearchBox(t *testing.T, out, action, q string) {
	t.Helper()
	for _, want := range []string{
		`name="q"`,
		`type="search"`,
		`value="` + q + `"`,
		`method="get"`,
		`action="` + action + `"`,
		"text-base",
		"min-h-11",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kotak cari harus memuat %q:\n%s", want, out)
		}
	}
}

// assertEscapedQ menerima kedua bentuk escape spasi (QueryEscape memakai +,
// Values.Encode juga +; %20 diterima untuk jaga-jaga).
func assertEscapedQ(t *testing.T, out, phrase string) {
	t.Helper()
	plus := "q=" + strings.ReplaceAll(phrase, " ", "+")
	pct := "q=" + strings.ReplaceAll(phrase, " ", "%20")
	if !strings.Contains(out, plus) && !strings.Contains(out, pct) {
		t.Errorf("harus memuat q ter-escape (%q):\n%s", phrase, out)
	}
}

// --- Leads -----------------------------------------------------------------

func TestLeadsList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, LeadsList(LeadsListView{
		Base:       "/w/desa",
		Items:      []LeadRow{{ID: 7, LeadName: "Lead Cocok"}},
		Tab:        "my",
		Query:      "kali muara", // ada spasi → wajib ter-escape
		NextCursor: "123_45",
	}))

	assertSearchBox(t, out, "/w/desa/leads", "kali muara")
	// Tab aktif dijaga lewat input tersembunyi saat submit.
	if !strings.Contains(out, `<input type="hidden" name="tab" value="my">`) {
		t.Errorf("form cari harus menjaga tab aktif via input tersembunyi tab=my:\n%s", out)
	}
	// Pager membawa cursor + tab + q agar halaman 2 tak melebar keluar pencarian.
	if !strings.Contains(out, "after=123_45") || !strings.Contains(out, "tab=my") {
		t.Errorf("pager harus membawa after & tab:\n%s", out)
	}
	// q muncul di pager DAN di tautan tab (pindah tab tak menjatuhkan pencarian).
	assertEscapedQ(t, out, "kali muara")
}

func TestLeadsList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, LeadsList(LeadsListView{
		Base: "/w/desa", Tab: "my", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada lead yang cocok") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian, bukan 'belum ada lead':\n%s", out)
	}
	// Jalan kembali mempertahankan tab tapi mengosongkan q.
	if !strings.Contains(out, `href="/w/desa/leads?tab=my"`) {
		t.Errorf("tautan kembali harus tanpa q (tab=my saja):\n%s", out)
	}
}

// --- Deals (tabel) ---------------------------------------------------------

func TestDealsTable_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, DealPipeline(DealPipelineView{
		Base:        "/w/desa",
		View:        "table",
		StageFilter: "Proposal",
		Query:       "sumur bor",
		Items:       []DealRow{{ID: 9, DealName: "Deal Cocok"}},
		NextCursor:  "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/deals", "sumur bor")
	// view=table & stage aktif dijaga lewat input tersembunyi.
	if !strings.Contains(out, `<input type="hidden" name="view" value="table">`) {
		t.Errorf("form cari harus menjaga view=table:\n%s", out)
	}
	if !strings.Contains(out, `<input type="hidden" name="stage" value="Proposal">`) {
		t.Errorf("form cari harus menjaga stage aktif:\n%s", out)
	}
	// Pager membawa view + after + stage + q (dicek terpisah — '&' dirender &amp;).
	for _, want := range []string{"view=table", "after=99_9", "stage=Proposal"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q:\n%s", want, out)
		}
	}
	assertEscapedQ(t, out, "sumur bor")
}

func TestDealsTable_SearchBoxHiddenOnKanban(t *testing.T) {
	// Papan Kanban (View "") DI LUAR lingkup slice — kotak cari tak dirender.
	out := renderLeads(t, DealPipeline(DealPipelineView{Base: "/w/desa", View: ""}))
	if strings.Contains(out, `name="q"`) {
		t.Errorf("kotak cari tak boleh muncul di papan Kanban (hanya view Tabel):\n%s", out)
	}
}

func TestDealsTable_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, DealPipeline(DealPipelineView{
		Base: "/w/desa", View: "table", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada deal yang cocok") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
}

// TestDealsTable_TabSearchRow — regresi BL-68 (keputusan user: align dgn toggle
// Semua/Deal Saya). Di view Tabel dgn ShowMineToggle, toggle Semua/Deal Saya &
// kotak cari harus sebaris dalam satu wrapper justify-between; tanpa toggle
// (cakupan 'own'), search berdiri sendiri full-width.
func TestDealsTable_TabSearchRow(t *testing.T) {
	withToggle := renderLeads(t, DealPipeline(DealPipelineView{
		Base: "/w/desa", View: "table", ShowMineToggle: true,
		Query: "sumur bor", Items: []DealRow{{ID: 9, DealName: "Deal Cocok"}},
	}))
	assertTabSearchRow(t, withToggle, "/w/desa/deals")

	// Tanpa toggle (cakupan 'own'): search tetap ada tapi TAK dibungkus wrapper.
	noToggle := renderLeads(t, DealPipeline(DealPipelineView{
		Base: "/w/desa", View: "table", ShowMineToggle: false,
		Query: "sumur bor", Items: []DealRow{{ID: 9, DealName: "Deal Cocok"}},
	}))
	if !strings.Contains(noToggle, `action="/w/desa/deals"`) {
		t.Errorf("tanpa toggle, form cari harus tetap dirender:\n%s", noToggle)
	}
	if strings.Contains(noToggle, tabSearchWrapperClass) {
		t.Errorf("tanpa toggle, TAK boleh ada wrapper tab+search:\n%s", noToggle)
	}
}

// --- Kontak (global) -------------------------------------------------------

func TestContactsAll_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, ContactsAll(ContactsAllView{
		Base:       "/w/desa",
		Items:      []ContactRow{{ID: 3, AccountID: 5, Name: "Budi"}},
		ShowTabs:   true,
		ActiveView: ContactViewMy,
		Query:      "pak lurah",
		NextCursor: "77_7",
	}))

	assertSearchBox(t, out, "/w/desa/contacts", "pak lurah")
	// view aktif ('my') dijaga lewat input tersembunyi.
	if !strings.Contains(out, `<input type="hidden" name="view" value="my">`) {
		t.Errorf("form cari harus menjaga view=my:\n%s", out)
	}
	// Pager membawa after + view + q.
	if !strings.Contains(out, "after=77_7") || !strings.Contains(out, "view=my") {
		t.Errorf("pager harus membawa after & view:\n%s", out)
	}
	assertEscapedQ(t, out, "pak lurah")
}

func TestContactsAll_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, ContactsAll(ContactsAllView{
		Base: "/w/desa", ShowTabs: true, ActiveView: ContactViewAll,
		Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada kontak yang cocok") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
}

// --- Quotes (global) -------------------------------------------------------

func TestQuotesIndex_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, QuotesIndex(QuotesIndexView{
		Base:       "/w/desa",
		Items:      []QuoteIndexRow{{QuoteID: 2, DealID: 4, QuoteName: "Quote Cocok"}},
		Query:      "paket hemat",
		NextCursor: "55_5",
	}))

	assertSearchBox(t, out, "/w/desa/quotes", "paket hemat")
	// Tanpa tab/keep field — pager cukup membawa after + q.
	if !strings.Contains(out, "after=55_5") {
		t.Errorf("pager harus membawa after:\n%s", out)
	}
	assertEscapedQ(t, out, "paket hemat")
}

func TestQuotesIndex_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, QuotesIndex(QuotesIndexView{
		Base: "/w/desa", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada quote yang cocok") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	// Truly-empty (tanpa q) harus pesan berbeda (arahan buat quote dari deal).
	empty := renderLeads(t, QuotesIndex(QuotesIndexView{Base: "/w/desa"}))
	if !strings.Contains(empty, "Buka sebuah deal") {
		t.Errorf("daftar benar-benar kosong harus mengarahkan buat quote dari deal:\n%s", empty)
	}
}
