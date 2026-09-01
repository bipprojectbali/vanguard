package panel

import (
	"strings"
	"testing"
)

// subscriptions_search_test.go — regresi BL-6 slice 3: kotak pencarian +
// penerusan ?q= pada daftar Active Subscriptions. Kembaran sales_search_test.go.
// Murni-data (tanpa DB): yang dijaga adalah KONTRAK URL — kotak cari native GET
// (bookmarkable, buang after), q diisi ulang, dan SEMUA jalur navigasi (tab
// status, pager "Berikutnya", tautan reset) membawa/menjaga q agar pencarian
// bertahan lintas halaman. Penyempitan hasil & batas F3 diuji di sisi handler
// (subscriptions_search_test.go paket handler). Memakai ulang assertSearchBox/
// assertEscapedQ/renderLeads dari sales_search_test.go (paket sama).

func TestSubList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base:         "/w/desa",
		StatusFilter: "Active",
		Statuses:     []string{"Active", "Trial"},
		Query:        "kali muara", // ada spasi → wajib ter-escape
		Items:        []SubRow{{ID: 1, Village: "Desa Cocok"}},
		NextCursor:   "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/subscriptions", "kali muara")
	// Status aktif dijaga lewat input tersembunyi saat submit.
	if !strings.Contains(out, `<input type="hidden" name="status" value="Active">`) {
		t.Errorf("form cari harus menjaga status aktif via input tersembunyi status=Active:\n%s", out)
	}
	// Pager membawa after + status + q ('&' dirender &amp; → dicek terpisah).
	for _, want := range []string{"after=99_9", "status=Active"} {
		if !strings.Contains(out, want) {
			t.Errorf("pager harus membawa %q:\n%s", want, out)
		}
	}
	// q muncul di pager DAN di tautan tab status (ganti status tak menjatuhkan q).
	assertEscapedQ(t, out, "kali muara")
}

func TestSubList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base: "/w/desa", StatusFilter: "Active",
		Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada langganan yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian, bukan 'belum ada langganan':\n%s", out)
	}
	// Tautan reset mempertahankan status aktif tapi mengosongkan q.
	if !strings.Contains(out, `href="/w/desa/subscriptions?status=Active"`) {
		t.Errorf("tautan reset harus tanpa q (status=Active saja):\n%s", out)
	}
}

// TestSubList_PrevLinkThreadsFilters — regresi BL-7: di halaman ke-2 (After +
// Trail terisi) tautan "« Sebelumnya" membawa status + q aktif TANPA after/trail
// (mundur ke halaman pertama hasil terfilter, bukan melebar ke semua langganan).
func TestSubList_PrevLinkThreadsFilters(t *testing.T) {
	out := renderLeads(t, SubList(SubListView{
		Base:         "/w/desa",
		StatusFilter: "Active",
		Statuses:     []string{"Active", "Trial"},
		Query:        "kali muara",
		Items:        []SubRow{{ID: 1, Village: "Desa Cocok"}},
		NextCursor:   "200_2",
		After:        "100_1",
		Trail:        "-",
	}))

	if !strings.Contains(out, "« Sebelumnya") {
		t.Errorf("halaman ke-2 harus punya tautan « Sebelumnya:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/subscriptions?status=Active&amp;q=kali+muara" class="btn btn-ghost min-h-11">« Sebelumnya`) {
		t.Errorf("« Sebelumnya harus membawa status=Active + q tanpa after/trail:\n%s", out)
	}
	if !strings.Contains(out, "Hal 2") {
		t.Errorf("harus menampilkan nomor halaman (Hal 2):\n%s", out)
	}
	if !strings.Contains(out, "after=200_2&amp;trail=-~100_1") {
		t.Errorf("Berikutnya harus push cursor halaman ini ke jejak:\n%s", out)
	}
}
