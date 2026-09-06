package panel

import (
	"strings"
	"testing"
)

// tab_search_row_test.go — regresi BL-68: di halaman list yang punya tab + kotak
// cari, keduanya harus sebaris dalam SATU wrapper justify-between (search
// terdorong ke pojok kanan), meniru pola acuan accounts.go — bukan dua baris
// terpisah. assertTabSearchRow dipakai ulang oleh test tiap halaman terdampak;
// test contacts (yang tak punya *_search_test.go sendiri) ada di sini.

// tabSearchWrapperClass = kelas wrapper baris tab+search (tabSearchRow di
// searchbox.go). Sengaja beda dari header (…gap-2 mb-2) & form cari
// (…gap-2 min-w-0 TANPA justify-between) → cocok hanya pada wrapper BL-68.
const tabSearchWrapperClass = "flex flex-wrap items-center justify-between gap-2 min-w-0"

// assertTabSearchRow memverifikasi bilah tab & kotak cari berada dalam satu
// wrapper justify-between, dgn urutan tab (kiri) → form cari (kanan). searchAction
// = action form cari halaman itu. Bila kembali dipisah ke dua baris, kelas wrapper
// hilang / urutan pecah → test gagal.
func assertTabSearchRow(t *testing.T, out, searchAction string) {
	t.Helper()
	wi := strings.Index(out, tabSearchWrapperClass)
	if wi < 0 {
		t.Fatalf("BL-68: wrapper tab+search %q tak ditemukan (search & tab tak sebaris?):\n%s",
			tabSearchWrapperClass, out)
	}
	fi := strings.Index(out, `action="`+searchAction+`"`)
	if fi < 0 {
		t.Fatalf("BL-68: form cari action=%q tak ditemukan:\n%s", searchAction, out)
	}
	// Tablist harus di dalam wrapper (setelah pembukanya) DAN sebelum form cari.
	rel := strings.Index(out[wi:], `class="tabs`)
	if rel < 0 {
		t.Fatalf("BL-68: bilah tab (class=\"tabs) tak ada setelah wrapper:\n%s", out)
	}
	ti := wi + rel
	if !(wi < ti && ti < fi) {
		t.Errorf("BL-68: urutan harus wrapper < tab < form cari (wrapper@%d tab@%d form@%d):\n%s",
			wi, ti, fi, out)
	}
}

// TestContactsAll_TabSearchRow — daftar kontak GLOBAL (ShowTabs) menyejajarkan
// tab dgn kotak cari (BL-68). Tanpa tab (peran cakupan 'own'), search berdiri
// sendiri full-width — diuji di cabang kedua.
func TestContactsAll_TabSearchRow(t *testing.T) {
	withTabs := renderLeads(t, ContactsAll(ContactsAllView{
		Base:       "/w/desa",
		ShowTabs:   true,
		ActiveView: ContactViewAll,
		Query:      "budi",
		Items:      []ContactRow{{ID: 1, Name: "Budi", AccountID: 7}},
	}))
	assertTabSearchRow(t, withTabs, "/w/desa/contacts")

	// Tanpa tab: form cari tetap ada, tapi TAK dibungkus wrapper tab+search.
	noTabs := renderLeads(t, ContactsAll(ContactsAllView{
		Base:     "/w/desa",
		ShowTabs: false,
		Query:    "budi",
		Items:    []ContactRow{{ID: 1, Name: "Budi", AccountID: 7}},
	}))
	if !strings.Contains(noTabs, `action="/w/desa/contacts"`) {
		t.Errorf("tanpa tab, form cari harus tetap dirender:\n%s", noTabs)
	}
	if strings.Contains(noTabs, tabSearchWrapperClass) {
		t.Errorf("tanpa tab, TAK boleh ada wrapper tab+search:\n%s", noTabs)
	}
}
