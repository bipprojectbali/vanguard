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

// TestQuoteDetail_AddItemAndTaxAsModals — regresi BL-70: form "Tambah Item" &
// "Pajak" tak lagi kartu selalu-tampil, melainkan modal CSP-safe (daisyUI
// checkbox-toggle) yang dibuka tombol pemicu DI DALAM section Line Items &
// Amounts. Submit form tetap native POST → 303 (action & field utuh). Gate
// CanMutate tetap: modal hanya dirender saat boleh mutasi.
func TestQuoteDetail_AddItemAndTaxAsModals(t *testing.T) {
	v := QuoteDetailView{
		Base: "/w/desa", DealID: 7, ID: 3, QuoteName: "Q-1",
		Status: "Draft", Statuses: []string{"Draft", "Sent"},
		TaxMode: "percent", TaxRateInput: "11",
		Items:    []QuoteItemRow{{ID: 9, PlanLabel: "Plan A", Quantity: "1", UnitPrice: "Rp 1", Subtotal: "Rp 1"}},
		Plans:    []QuotePlanOption{{ID: 1, Label: "Plan A"}},
		CanWrite: true, Quotable: true,
	}
	out := renderQuoteNode(t, QuoteDetail(v))

	// (a) Pemicu + wadah modal ADA (label for=id → checkbox-toggle, nol JS/CSP-safe).
	for _, want := range []string{
		`for="quote-additem"`, `for="quote-tax"`,
		"Edit Pajak", // tombol pemicu modal pajak
		`id="quote-additem" class="modal-toggle"`,
		`id="quote-tax" class="modal-toggle"`,
		`class="modal"`, "modal-box", "modal-backdrop",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BL-70: markup modal quote harus memuat %q:\n%s", want, out)
		}
	}

	// (b) Aksi POST form TETAP utuh di dalam modal (native POST → 303; gotcha #16).
	for _, want := range []string{
		`action="/w/desa/deals/7/quotes/3/items"`,
		`action="/w/desa/deals/7/quotes/3/tax"`,
		`name="plan_id"`, `name="quantity"`, `name="discount_pct"`,
		`name="tax_mode"`, `name="tax_rate"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BL-70: form POST harus tetap utuh %q:\n%s", want, out)
		}
	}

	// (c) Struktur: form Tambah Item berada DI DALAM modal-box (setelah pembukanya),
	// bukan kartu selalu-tampil sebelum totals. iAdd = action add-item EKSAK (bukan
	// .../items/9 milik sunting per-baris).
	iModalBox := strings.Index(out, "modal-box")
	iAdd := strings.Index(out, `action="/w/desa/deals/7/quotes/3/items"`)
	if iModalBox < 0 || iAdd < 0 || iAdd < iModalBox {
		t.Errorf("BL-70: form Tambah Item harus di dalam modal-box "+
			"(iModalBox=%d iAdd=%d):\n%s", iModalBox, iAdd, out)
	}
}

// TestQuoteDetail_AddItemModalEmptyCatalog — cabang katalog plan kosong: tombol
// pemicu tetap ada, tapi isi modal beri keterangan jujur (bukan form mati).
func TestQuoteDetail_AddItemModalEmptyCatalog(t *testing.T) {
	v := QuoteDetailView{
		Base: "/w/desa", DealID: 7, ID: 3, QuoteName: "Q-1",
		Status: "Draft", Statuses: []string{"Draft"},
		Plans:    nil, // katalog kosong
		CanWrite: true, Quotable: true,
	}
	out := renderQuoteNode(t, QuoteDetail(v))
	if !strings.Contains(out, `for="quote-additem"`) {
		t.Errorf("katalog kosong: tombol pemicu Tambah Item harus tetap ada:\n%s", out)
	}
	if !strings.Contains(out, "Belum ada plan aktif di katalog") {
		t.Errorf("katalog kosong: modal harus beri keterangan jujur:\n%s", out)
	}
	if strings.Contains(out, `action="/w/desa/deals/7/quotes/3/items"`) {
		t.Errorf("katalog kosong: tak boleh render form add-item mati:\n%s", out)
	}
}
