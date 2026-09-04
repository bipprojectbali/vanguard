package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// workspace.go — halaman yang tersisa di RUANG KERJA terkait workspace. Identitas
// workspace (ganti nama) & zona bahaya (arsip/hapus/pulihkan) SUDAH DIPINDAH ke
// ruang developer sebagai wewenang platform (BL-53) — view-nya di
// internal/ui/pages/dev/workspace_detail.go. Yang tersisa di sini hanyalah
// halaman 403 workspace yang ditangguhkan, yang memang menyapa anggotanya.

// WorkspaceSuspended = halaman 403 untuk anggota workspace yang ditangguhkan
// platform. SENGAJA menjelaskan (bukan 404 seperti slug asing): penerimanya
// sudah terbukti anggota, dan tanpa alasan ia akan mengira workspace-nya hilang
// lalu menghubungi support tanpa perlu (0005 §3).
func WorkspaceSuspended(name, reason string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Workspace Ditangguhkan")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Akses ke workspace "+name+" sedang ditangguhkan oleh pengelola platform.")),
	}
	if reason != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "suspend-reason", g.Text(reason)))
	}
	body = append(body, h.P(
		h.Class("text-sm text-base-content/60 mt-4"),
		g.Text("Data Anda tetap utuh. Hubungi pengelola platform untuk mengaktifkannya kembali."),
	))
	return h.Div(h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"), g.Group(body))
}
