package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// crm_onboard.go — banner opt-in CRM di beranda ruang kerja.

// CRMOnboard = banner untuk pengelola (owner/admin) yang BELUM punya peran CRM.
// CanBusiness fail-closed: tanpa business_role, seluruh menu CRM (Desa, Kontak,
// Sales, Peran) tak dirakit sama sekali — owner baru bisa terkunci total dari CRM
// di workspace-nya sendiri. Tombol ini menugaskan dirinya sebagai Admin CRM sekali
// klik, membuka pintu itu (RefreshIdentity menyegarkan business_role per-request).
//
// Native POST → 303 (gotcha #16), sumbu CRM boleh menyentuh diri sendiri (opt-in).
// action = path self-assign & adminRole = nilai peran, KEDUANYA dioper handler:
// view tak merakit path sendiri dan tak boleh tahu nama peran sistem (murni-data).
func CRMOnboard(action, adminRole string) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-primary/40 min-w-0"),
		h.Div(
			h.Class("card-body gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Aktifkan CRM untuk Anda")),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Anda belum punya peran CRM, jadi menu CRM (Desa, Kontak, "+
					"Sales, Peran) belum muncul. Jadikan diri Anda Admin CRM untuk "+
					"membukanya — bisa diubah kapan saja di menu Anggota.")),
			h.FormEl(
				h.Method("post"), h.Action(action),
				h.Input(h.Type("hidden"), h.Name("business_role"), h.Value(adminRole)),
				h.Button(h.Type("submit"), h.Class("btn btn-primary btn-sm"),
					g.Text("Jadikan saya Admin CRM")),
			),
		),
	)
}
