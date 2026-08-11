package codes

import "testing"

// codes_test.go — perakitan & validasi kode entitas. Yang dijaga adalah sifat
// yang bila rusak membuat kode berhenti berguna sebagai PENANDA UNIK: padding
// yang MEMOTONG (dua entitas berbagi kode), atau format tak masuk akal yang lolos
// ke DB dan meledak sebagai galat constraint.

func TestRender(t *testing.T) {
	cases := []struct {
		f    Format
		seq  int64
		want string
		why  string
	}{
		{Format{"DESA", "-", 3}, 1, "DESA-001", "padding kiri sampai 3 digit"},
		{Format{"DESA", "-", 3}, 42, "DESA-042", "dua digit tetap dipad"},
		{Format{"DESA", "-", 3}, 999, "DESA-999", "pas selebar padding"},
		{Format{"DESA", "-", 3}, 1000, "DESA-1000", "MELEBIHI padding → melebar, TAK dipotong"},
		{Format{"DEAL", "-", 3}, 7, "DEAL-007", "prefix lain"},
		{Format{"SUB", "", 4}, 5, "SUB0005", "separator kosong sah"},
		{Format{"REC", "-", 0}, 12, "REC-12", "padding 0 = angka apa adanya"},
	}
	for _, c := range cases {
		if got := c.f.Render(c.seq); got != c.want {
			t.Errorf("Render(%+v, %d) = %q, want %q — %s", c.f, c.seq, got, c.want, c.why)
		}
	}
}

// TestRender_TakMemotong menegakkan sifat paling kritis secara terpisah: begitu
// nomor melewati lebar padding, kode HARUS melebar. Memotong = tabrakan kode.
func TestRender_TakMemotong(t *testing.T) {
	f := Format{"DESA", "-", 3}
	a := f.Render(1000)
	b := f.Render(10000)
	if a == b {
		t.Fatalf("1000 dan 10000 menghasilkan kode sama (%q) — padding memotong, kode tabrakan", a)
	}
	if a != "DESA-1000" || b != "DESA-10000" {
		t.Errorf("melebar salah: got %q & %q", a, b)
	}
}

func TestDefaultFormat(t *testing.T) {
	// Entitas dikenal → default spesifik.
	if got := DefaultFormat(EntityAccount); got.Prefix != "DESA" {
		t.Errorf("default account prefix = %q, want DESA", got.Prefix)
	}
	if got := DefaultFormat(EntityTicket); got.Padding != 4 {
		t.Errorf("default ticket padding = %d, want 4", got.Padding)
	}
	// Entitas tak dikenal → bentuk netral, BUKAN kosong/panik (create tak boleh
	// gagal total hanya karena default belum terdaftar).
	got := DefaultFormat(Entity("belum_ada"))
	if got.Prefix == "" {
		t.Error("entitas tak dikenal harus dapat prefix netral, bukan kosong")
	}
	if r := got.Render(1); r == "" {
		t.Error("format netral harus tetap merender kode")
	}
}

func TestValid(t *testing.T) {
	for _, e := range []Entity{EntityAccount, EntityLead, EntityDeal, EntityQuote, EntityTicket, EntitySubscription} {
		if !Valid(e) {
			t.Errorf("entity %q harus valid", e)
		}
	}
	for _, e := range []Entity{"", "account ", "Account", "user", "desa"} {
		if Valid(e) {
			t.Errorf("entity %q harus DITOLAK", e)
		}
	}
}

// TestValidFormat mencerminkan CHECK di migrasi 00006 — ditolak di Go dulu, DB
// jadi jaring terakhir. Prefix WAJIB (kode tanpa prefix cuma angka).
func TestValidFormat(t *testing.T) {
	ok := []Format{
		{"DESA", "-", 3},
		{"D", "", 0},                  // minimum: 1 huruf prefix, tanpa separator/padding
		{"ABCDEFGHIJKLMNOP", "-", 12}, // batas atas prefix (16) & padding (12)
	}
	for _, f := range ok {
		if !ValidFormat(f) {
			t.Errorf("format %+v harus valid", f)
		}
	}
	bad := []struct {
		f   Format
		why string
	}{
		{Format{"", "-", 3}, "prefix kosong"},
		{Format{"ABCDEFGHIJKLMNOPQ", "-", 3}, "prefix 17 > 16"},
		{Format{"DESA", "----", 3}, "separator 4 > 3"},
		{Format{"DESA", "-", -1}, "padding negatif"},
		{Format{"DESA", "-", 13}, "padding 13 > 12"},
	}
	for _, c := range bad {
		if ValidFormat(c.f) {
			t.Errorf("format %+v harus DITOLAK — %s", c.f, c.why)
		}
	}
}

func TestAllEntities_Salinan(t *testing.T) {
	a := AllEntities()
	if len(a) != 6 {
		t.Fatalf("harus 6 entitas, got %d", len(a))
	}
	// Mutasi hasil tak boleh merusak daftar internal.
	a[0] = "diretas"
	if AllEntities()[0] != EntityAccount {
		t.Error("AllEntities harus mengembalikan SALINAN, bukan slice internal")
	}
}
