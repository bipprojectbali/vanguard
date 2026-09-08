package panel

import (
	"strings"
	"testing"
)

// pending_approval_test.go — halaman "menunggu approval admin" (Opsi A, BL-105).
// Jaga: heading + pesan approval + nama workspace ter-render; fallback saat nama
// kosong; kalimat lama "tergabung" (diminta user diganti) TAK muncul.

func TestPendingApproval_Text(t *testing.T) {
	out := renderLeads(t, PendingApproval("Desa Makmur"))
	wants := []string{
		"Menunggu approval admin",    // heading (kalimat baru per permintaan user)
		"menunggu persetujuan admin", // isi: menekankan approval, bukan "tergabung"
		"Desa Makmur",                // nama workspace dioper handler
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("PendingApproval harus memuat %q:\n%s", w, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "tergabung") {
		t.Errorf("kalimat lama 'tergabung' tak boleh muncul (diminta user diubah):\n%s", out)
	}
}

func TestPendingApproval_FallbackNamaKosong(t *testing.T) {
	out := renderLeads(t, PendingApproval(""))
	if !strings.Contains(out, "ruang kerja ini") {
		t.Errorf("workspaceName kosong harus fallback ke 'ruang kerja ini':\n%s", out)
	}
}
