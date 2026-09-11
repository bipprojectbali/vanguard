package ui

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func renderNode(t *testing.T, n g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := n.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestConfirmModal(t *testing.T) {
	out := renderNode(t, ConfirmModal("logoutConfirm", "Keluar?", "Yakin?", "Keluar", "/logout"))

	// Tampil hanya saat signal true.
	if !strings.Contains(out, `data-show="$logoutConfirm"`) {
		t.Errorf("modal harus data-show pada signal:\n%s", out)
	}
	// Judul + pesan + tombol.
	for _, want := range []string{"Keluar?", "Yakin?", ">Batal<"} {
		if !strings.Contains(out, want) {
			t.Errorf("modal kurang %q:\n%s", want, out)
		}
	}
	// Tombol konfirmasi = NATIVE form POST (bukan @post — redirect SSE diblokir
	// CSP). Assert form method+action, BUKAN ekspresi Datastar.
	for _, want := range []string{`method="post"`, `action="/logout"`, `type="submit"`} {
		if !strings.Contains(out, want) {
			t.Errorf("tombol konfirmasi harus native form submit, kurang %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "@post") {
		t.Errorf("ConfirmModal TAK boleh pakai @post (navigasi = native form):\n%s", out)
	}
	// Batal & backdrop menutup via signal.
	if !strings.Contains(out, "$logoutConfirm = false") {
		t.Errorf("harus ada aksi tutup:\n%s", out)
	}
}

func TestConfirmTrigger(t *testing.T) {
	out := renderNode(t, ConfirmTrigger("logoutConfirm"))
	if !strings.Contains(out, "$logoutConfirm = true") {
		t.Errorf("trigger harus set signal true:\n%s", out)
	}
	// Trigger TIDAK boleh langsung @post (itu poin konfirmasi).
	if strings.Contains(out, "@post") {
		t.Errorf("trigger tak boleh langsung @post:\n%s", out)
	}
}

func TestToast(t *testing.T) {
	out := renderNode(t, Toast(VariantSuccess, "accounts-ok", g.Text("Desa ditambahkan.")))

	// Div luar: posisi mengambang + wajib pointer-events:none (gotcha #7 —
	// opacity:0 pun tetap menangkap klik).
	for _, want := range []string{`id="accounts-ok"`, "fixed", "bottom-4", "right-4", "z-50", "pointer-events:none"} {
		if !strings.Contains(out, want) {
			t.Errorf("Toast kurang %q di wadah luar:\n%s", want, out)
		}
	}
	// Div dalam: warna varian + animasi auto-fade CSS (bukan JS).
	for _, want := range []string{"alert-success", "toast-flash", "Desa ditambahkan."} {
		if !strings.Contains(out, want) {
			t.Errorf("Toast kurang %q di isi:\n%s", want, out)
		}
	}
}

func TestToastVariantError(t *testing.T) {
	out := renderNode(t, Toast(VariantDestructive, "accounts-err", g.Text("Gagal.")))
	if !strings.Contains(out, "alert-error") {
		t.Errorf("Toast VariantDestructive harus alert-error:\n%s", out)
	}
}

func TestToastSlot(t *testing.T) {
	out := renderNode(t, ToastSlot("flash"))

	if !strings.Contains(out, `id="flash"`) || !strings.Contains(out, "pointer-events:none") {
		t.Errorf("ToastSlot harus wadah posisi ber-id:\n%s", out)
	}
	// Slot kosong TAK BOLEH bawa .toast-flash — dipasang dari awal (bukan
	// diisi Toast()) akan memicu animasi fade kosong saat halaman dimuat.
	if strings.Contains(out, "toast-flash") || strings.Contains(out, "alert") {
		t.Errorf("ToastSlot harus kosong (tanpa .alert/.toast-flash):\n%s", out)
	}
}
