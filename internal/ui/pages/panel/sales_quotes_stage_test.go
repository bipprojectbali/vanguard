package panel

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

// sales_quotes_stage_test.go — regresi BL-13 (sisi view): kontrol MUTASI quote
// tak dirender saat deal di luar jendela quoting (Quotable=false), dan alasan
// muncul sebagai banner. Keputusan Quotable di-precompute HANDLER (murni-data);
// di sini kita jaga view menghormatinya. Backend tetap penjaga sesungguhnya —
// menyembunyikan tombol saja tak cukup, tapi tombol mati yang tampak menyesatkan.

func renderQuoteNode(t *testing.T, node g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := node.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// TestQuotesList_HidesCreateWhenNotQuotable: daftar quote menyembunyikan "Buat
// Quote" saat !Quotable dan menampilkan banner alasan; saat Quotable tombol muncul
// & banner tidak. Gate hanya untuk pemegang tulis (CanWrite).
func TestQuotesList_HidesCreateWhenNotQuotable(t *testing.T) {
	const lockMsg = "Deal masih di tahap Prospecting"

	locked := renderQuoteNode(t, QuotesList(QuotesListView{
		Base: "/w/desa", DealID: 7, DealName: "Deal A",
		CanWrite: true, Quotable: false, StageLockMsg: lockMsg,
	}))
	if strings.Contains(locked, "Buat Quote") {
		t.Errorf("!Quotable: tombol 'Buat Quote' tak boleh dirender:\n%s", locked)
	}
	if !strings.Contains(locked, lockMsg) {
		t.Errorf("!Quotable: banner alasan (%q) harus tampil:\n%s", lockMsg, locked)
	}

	open := renderQuoteNode(t, QuotesList(QuotesListView{
		Base: "/w/desa", DealID: 7, DealName: "Deal A",
		CanWrite: true, Quotable: true, StageLockMsg: "",
	}))
	if !strings.Contains(open, "Buat Quote") {
		t.Errorf("Quotable: tombol 'Buat Quote' harus dirender:\n%s", open)
	}
	if strings.Contains(open, lockMsg) {
		t.Errorf("Quotable: banner terkunci tak boleh tampil:\n%s", open)
	}
}

// TestQuoteDetail_HidesControlsWhenNotQuotable: builder menyembunyikan SEMUA
// kontrol mutasi (Sunting/Hapus header, Tambah Item, Ubah Status, kolom Aksi item)
// saat !Quotable, menampilkan banner; saat Quotable kontrol muncul.
func TestQuoteDetail_HidesControlsWhenNotQuotable(t *testing.T) {
	const lockMsg = "Deal sudah ditutup (Closed Won/Lost)"
	base := QuoteDetailView{
		Base: "/w/desa", DealID: 7, ID: 3, QuoteName: "Q-1",
		Status: "Draft", Statuses: []string{"Draft", "Sent"},
		Items: []QuoteItemRow{{ID: 9, PlanLabel: "Plan A", Quantity: "1", UnitPrice: "Rp 1", Subtotal: "Rp 1"}},
		Plans: []QuotePlanOption{{ID: 1, Label: "Plan A"}},
	}

	locked := base
	locked.CanWrite, locked.Quotable, locked.StageLockMsg = true, false, lockMsg
	out := renderQuoteNode(t, QuoteDetail(locked))
	for _, forbidden := range []string{"Tambah Item", "Ubah Status", "Simpan Status", ">Aksi<"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("!Quotable: kontrol %q tak boleh dirender:\n%s", forbidden, out)
		}
	}
	if !strings.Contains(out, lockMsg) {
		t.Errorf("!Quotable: banner alasan (%q) harus tampil", lockMsg)
	}

	open := base
	open.CanWrite, open.Quotable = true, true
	out = renderQuoteNode(t, QuoteDetail(open))
	for _, want := range []string{"Tambah Item", "Ubah Status", "Sunting", "Hapus"} {
		if !strings.Contains(out, want) {
			t.Errorf("Quotable: kontrol %q harus dirender:\n%s", want, out)
		}
	}
}
