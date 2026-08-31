package panel

import (
	"strings"
	"testing"
)

// tickets_search_test.go — regresi BL-6 slice 4: kotak pencarian + penerusan ?q=
// pada daftar Tickets. Murni-data (tanpa DB): yang dijaga KONTRAK URL — kotak cari
// native GET (bookmarkable, buang after), q diisi ulang, dan SEMUA jalur navigasi
// (tab status, pager, reset) membawa/menjaga q. Penyempitan hasil & batas F3 diuji
// di sisi handler. Memakai ulang assertSearchBox/assertEscapedQ/renderLeads.

func TestTicketsList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, TicketsList(TicketsListView{
		Base:       "/w/desa",
		Tab:        "baru",
		Query:      "kali muara", // ada spasi → wajib ter-escape
		Items:      []TicketRow{{ID: 1, AccountName: "Desa Cocok", Subject: "Internet mati"}},
		NextCursor: "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/tickets", "kali muara")
	if !strings.Contains(out, `<input type="hidden" name="tab" value="baru">`) {
		t.Errorf("form cari harus menjaga tab aktif via input tersembunyi tab=baru:\n%s", out)
	}
	if !strings.Contains(out, "after=99_9") || !strings.Contains(out, "tab=baru") {
		t.Errorf("pager harus membawa after & tab:\n%s", out)
	}
	assertEscapedQ(t, out, "kali muara")
}

func TestTicketsList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, TicketsList(TicketsListView{
		Base: "/w/desa", Tab: "baru", Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada tiket yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/tickets?tab=baru"`) {
		t.Errorf("tautan reset harus menjaga tab tapi membuang q:\n%s", out)
	}
}
