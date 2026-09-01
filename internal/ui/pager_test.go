package ui

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func renderPager(t *testing.T, node g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := node.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// Cursor asli berbentuk `<nano>_<id>` (digit + underscore); token jejak
// divalidasi mengikuti bentuk itu, jadi fixture memakai cursor realistis.
const (
	c1 = "100_1"
	c2 = "200_2"
	c3 = "300_3"
)

// Halaman 1 (belum ada jejak) dengan halaman berikutnya: TAK ada "Sebelumnya",
// ada "Hal 1", dan "Berikutnya" mendorong sentinel halaman-pertama ke jejak.
// Catatan: gomponents meng-escape & di atribut href jadi &amp;.
func TestKeysetPager_FirstPage(t *testing.T) {
	out := renderPager(t, KeysetPager("/w/desa/accounts", "", "", c1))

	if strings.Contains(out, "Sebelumnya") {
		t.Errorf("halaman pertama tak boleh punya tombol Sebelumnya:\n%s", out)
	}
	if !strings.Contains(out, "Hal 1") {
		t.Errorf("harus menampilkan Hal 1:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/accounts?after=100_1&amp;trail=-"`) {
		t.Errorf("Berikutnya harus mendorong sentinel ke jejak:\n%s", out)
	}
	if !strings.Contains(out, "Berikutnya »") {
		t.Errorf("harus ada tombol Berikutnya:\n%s", out)
	}
}

// Halaman 2 (jejak = sentinel): "Sebelumnya" kembali ke halaman pertama (buang
// after & trail), "Hal 2", "Berikutnya" mendorong after halaman-2 ke jejak.
func TestKeysetPager_SecondPage(t *testing.T) {
	out := renderPager(t, KeysetPager("/w/desa/accounts", c1, "-", c2))

	if !strings.Contains(out, "« Sebelumnya") {
		t.Errorf("halaman 2 harus punya Sebelumnya:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/accounts" class="btn btn-ghost min-h-11">« Sebelumnya`) {
		t.Errorf("Sebelumnya dari hal 2 harus menuju daftar kanonik (tanpa after/trail):\n%s", out)
	}
	if !strings.Contains(out, "Hal 2") {
		t.Errorf("harus menampilkan Hal 2:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/accounts?after=200_2&amp;trail=-~100_1"`) {
		t.Errorf("Berikutnya dari hal 2 harus push c1 ke jejak:\n%s", out)
	}
}

// Halaman 3 (jejak = -~c1): "Sebelumnya" pop ke halaman 2 (after=c1, trail=-).
func TestKeysetPager_ThirdPage_PrevPops(t *testing.T) {
	out := renderPager(t, KeysetPager("/w/desa/accounts", c2, "-~"+c1, c3))

	if !strings.Contains(out, `href="/w/desa/accounts?after=100_1&amp;trail=-"`) {
		t.Errorf("Sebelumnya dari hal 3 harus pop kembali ke hal 2 (after=c1, trail=-):\n%s", out)
	}
	if !strings.Contains(out, "Hal 3") {
		t.Errorf("harus menampilkan Hal 3:\n%s", out)
	}
	if !strings.Contains(out, `href="/w/desa/accounts?after=300_3&amp;trail=-~100_1~200_2"`) {
		t.Errorf("Berikutnya dari hal 3 harus push c2:\n%s", out)
	}
}

// Halaman terakhir (ada jejak, next kosong): ada Sebelumnya + "Ujung daftar.",
// TAK ada tombol Berikutnya.
func TestKeysetPager_LastPage(t *testing.T) {
	out := renderPager(t, KeysetPager("/w/desa/accounts", c2, "-~"+c1, ""))

	if !strings.Contains(out, "« Sebelumnya") {
		t.Errorf("halaman terakhir (bukan hal 1) harus tetap punya Sebelumnya:\n%s", out)
	}
	if strings.Contains(out, "Berikutnya") {
		t.Errorf("halaman terakhir tak boleh punya Berikutnya:\n%s", out)
	}
	if !strings.Contains(out, "Ujung daftar.") {
		t.Errorf("halaman terakhir harus menyatakan ujung daftar:\n%s", out)
	}
	if !strings.Contains(out, "Hal 3") {
		t.Errorf("halaman terakhir harus menampilkan nomor halaman:\n%s", out)
	}
}

// Daftar satu halaman MURNI (tanpa after, tanpa jejak, tanpa next): tak ada apa
// pun untuk dinavigasi → KeysetPager kembalikan nil (bukan span "Ujung daftar.").
// nil aman disisipkan di view (gomponents melewati anak nil).
func TestKeysetPager_SinglePage(t *testing.T) {
	if node := KeysetPager("/w/desa/accounts", "", "", ""); node != nil {
		out := renderPager(t, node)
		t.Errorf("daftar satu halaman murni harus nil, bukan merender:\n%s", out)
	}
}

// Deep-link ke halaman akhir TANPA jejak (mis. bookmark ?after=… yang kebetulan
// halaman terakhir): after terisi tapi trail & next kosong. Tak bisa maju maupun
// mundur (jejak tak ada) → nyatakan "Ujung daftar." saja, tanpa nomor "Hal N"
// palsu dan tanpa tombol navigasi.
func TestKeysetPager_DeepLinkLastPage(t *testing.T) {
	out := renderPager(t, KeysetPager("/w/desa/accounts", c2, "", ""))

	if strings.Contains(out, "Sebelumnya") || strings.Contains(out, "Berikutnya") {
		t.Errorf("deep-link tanpa jejak tak boleh punya tombol navigasi:\n%s", out)
	}
	if strings.Contains(out, "Hal ") {
		t.Errorf("deep-link tanpa jejak tak boleh mengklaim nomor halaman:\n%s", out)
	}
	if !strings.Contains(out, "Ujung daftar.") {
		t.Errorf("deep-link ke halaman akhir harus menyatakan ujung:\n%s", out)
	}
}

// baseHref yang SUDAH berparam (filter/q) → param after/trail disambung dengan &,
// bukan ? kedua.
func TestKeysetPager_BaseHrefWithParams(t *testing.T) {
	base := "/w/desa/subscriptions?status=active&q=budi"
	out := renderPager(t, KeysetPager(base, "", "", c1))

	if !strings.Contains(out, `href="/w/desa/subscriptions?status=active&amp;q=budi&amp;after=100_1&amp;trail=-"`) {
		t.Errorf("param after/trail harus disambung dgn & bila baseHref sudah berparam:\n%s", out)
	}
	// Setelah tanda tanya pertama, sisanya harus & — tak ada ? kedua.
	if i := strings.Index(out, "?"); i >= 0 && strings.Contains(out[i+1:], "?") {
		t.Errorf("tak boleh ada tanda tanya kedua di href:\n%s", out)
	}
}

// Jejak sampah/kelewat panjang → diperlakukan kosong (lenient): halaman jadi
// Hal 1 tanpa Sebelumnya, maju tetap jalan.
func TestKeysetPager_GarbageTrailLenient(t *testing.T) {
	for _, bad := range []string{"not_a_cursor", "abc~def", strings.Repeat("1_2~", 2000)} {
		out := renderPager(t, KeysetPager("/list", c3, bad, c3))
		if strings.Contains(out, "Sebelumnya") {
			t.Errorf("jejak sampah %q harus di-reset (tanpa Sebelumnya):\n%s", bad, out)
		}
		if !strings.Contains(out, "Hal 1") {
			t.Errorf("jejak sampah %q harus jatuh ke Hal 1:\n%s", bad, out)
		}
	}
}

// splitTrail: token valid dipertahankan, satu token rusak membuang seluruh jejak.
func TestSplitTrail(t *testing.T) {
	if got := splitTrail("-~123_45~678_9"); len(got) != 3 {
		t.Errorf("jejak valid 3 entri, dapat %v", got)
	}
	if got := splitTrail("-~bad"); got != nil {
		t.Errorf("satu token rusak → seluruh jejak dibuang, dapat %v", got)
	}
	if got := splitTrail(""); got != nil {
		t.Errorf("jejak kosong → nil, dapat %v", got)
	}
}
