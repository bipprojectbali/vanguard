package panel

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// pending_approval.go — halaman "menunggu approval admin" (Opsi A, BL-105).
//
// Ditampilkan untuk anggota BARU yang masuk ruang kerja tetapi belum diberi peran
// CRM (business_role kosong). Tanpa peran, seluruh menu CRM fail-closed
// (CanBusiness) → Beranda tampak "menu mati" tanpa penjelasan, seolah aplikasi
// rusak. Panel ini menggantikan Beranda itu: user tahu ia harus MENUNGGU admin
// menetapkan peran, bukan menyangka ada bug. Bukan kebocoran data — user pending
// data_scope=none (nol baris) + RLS; ini murni kejelasan UX.
//
// Murni-data (konvensi view): teks statis; workspaceName dioper handler.
func PendingApproval(workspaceName string) g.Node {
	where := "ruang kerja ini"
	if workspaceName != "" {
		where = workspaceName
	}
	return h.Div(
		h.Class("card bg-base-100 border border-warning/40 min-w-0 max-w-xl mx-auto"),
		h.Div(
			h.Class("card-body gap-3 items-center text-center"),
			lucide.Hourglass(h.Class("size-10 text-warning")),
			h.H2(h.Class("card-title"), g.Text("Menunggu approval admin")),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Akun Anda sedang menunggu persetujuan admin. Anda akan "+
					"mendapat akses ke "+where+" setelah admin menetapkan peran "+
					"untuk Anda. Silakan hubungi admin ruang kerja bila belum juga "+
					"diberi akses.")),
		),
	)
}
