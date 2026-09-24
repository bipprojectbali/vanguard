package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// members_invite.go — form undang (inviteForm) & daftar undangan pending
// (inviteList), dipisah dari members.go agar tiap file di bawah ambang tipe
// View/Component (300). Shell daftar anggota tetap di members.go — satu paket.

// inviteForm = undang lewat email. Form NATIVE POST → 303 (gotcha #16).
//
// BL-170: role tenant tak lagi dipilih di sini (selalu "member" server-side,
// promosi admin terjadi PASCA-join lewat baris anggota) — diganti Jenis
// Anggota (kind, wajib) + Peran CRM (business_role, opsional), sepasang
// cascading select. $invkind (data.Bind, diinisialisasi via data.Signals ke
// "internal" → no-FOUC, pola sales_leads_status_control.go) menentukan select
// business_role mana yang TAMPIL (showWhen) DAN AKTIF (data.Attr disabled):
// dua <select name="business_role"> memakai nama sama, jadi yang dinonaktifkan
// tak ikut terkirim — hanya satu yang benar-benar sampai ke server (pola
// reactiveCheckbox, role_edit_additional.go). Guard sesungguhnya tetap di
// backend (InviteCreate: kind divalidasi ValidKind, business_role dicocokkan
// ulang ke kind via GetBusinessRole) — toggle ini murni UX.
//
// canEditKind=false (BL-171: aktor bercakupan satu jenis saja) → Jenis
// Anggota TAK dipilih di sini sama sekali: dikunci lewat input hidden ke
// satu-satunya jenis yang crmRolesInternal/crmRolesExternal-nya terisi
// (handler sudah memfilter dua daftar itu sesuai cakupan aktor sebelum
// sampai ke sini, members_page.go), select "Internal"/"Eksternal" & toggle
// showWhen-nya tak dirender. Backend (InviteCreate) TETAP penjaga
// sesungguhnya — ini murni mencegah aktor mencoba mengundang jenis di luar
// cakupannya lewat UI, bukan satu-satunya lapisan.
func inviteForm(base string, crmRolesInternal, crmRolesExternal []CRMRoleOption, canEditKind bool) g.Node {
	lockedKind := ""
	if !canEditKind {
		lockedKind = "internal"
		if len(crmRolesExternal) > 0 && len(crmRolesInternal) == 0 {
			lockedKind = "external"
		}
	}
	kindField := g.Node(h.Div(
		h.Class("grid gap-2"),
		ui.Label("Jenis Anggota", h.For("invite-kind")),
		h.Select(h.ID("invite-kind"), h.Class("select"), h.Name("kind"),
			data.Bind("invkind"),
			// Ganti Jenis Anggota → reset peran CRM ke "(tak ada)": daftar
			// pilihan berubah total (internal↔eksternal), pilihan lama dari
			// daftar sebelumnya tak lagi berarti apa-apa untuk kind baru.
			// Tanpa reset ini, invrole (dibagikan kedua select business_role)
			// bisa tetap berisi nama peran dari kind SEBELUMNYA → tombol
			// Daftar terlihat aktif padahal peran itu tak valid utk kind baru.
			data.On("change", "$invrole = ''"),
			h.Option(h.Value("internal"), g.Text("Internal")),
			h.Option(h.Value("external"), g.Text("Eksternal")),
		),
	))
	if lockedKind != "" {
		kindField = h.Input(h.Type("hidden"), h.Name("kind"), h.Value(lockedKind))
	}
	initKind := "internal"
	if lockedKind != "" {
		initKind = lockedKind
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(
			h.Class("card-body"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Tambah Anggota")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/members/invite"),
				// invrole: dibagikan KEDUA select business_role (internal & external) —
				// hanya salah satu tampil (showWhen ikut invkind), tapi keduanya menulis
				// signal yang sama sehingga tombol submit tahu peran CRM sudah dipilih
				// atau belum, siapa pun yang sedang aktif. "" (opsi "(tak ada)") = belum
				// memilih; peran CRM WAJIB dipilih (keputusan produk) — tombol Daftar
				// disable selama itu, bukan divalidasi setelah submit gagal.
				data.Signals(map[string]any{"invkind": initKind, "invrole": ""}),
				h.Class("flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-end"),
				h.Div(
					h.Class("grid gap-2 flex-1 min-w-0"),
					ui.Label("Email", h.For("invite-email")),
					ui.Input(h.ID("invite-email"), h.Name("email"), h.Type("email"),
						h.Placeholder("nama@contoh.com")),
				),
				kindField,
				showWhen("$invkind == 'internal'", "grid gap-2 min-w-0",
					g.Group([]g.Node{
						ui.Label("Peran CRM", h.For("invite-role-internal")),
						h.Select(h.ID("invite-role-internal"), h.Class("select"), h.Name("business_role"),
							data.Bind("invrole"),
							data.Attr("disabled", "$invkind != 'internal'"),
							g.Group(crmRoleOpts(crmRolesInternal, "")),
						),
					}),
				),
				showWhen("$invkind == 'external'", "grid gap-2 min-w-0",
					g.Group([]g.Node{
						ui.Label("Peran CRM", h.For("invite-role-external")),
						h.Select(h.ID("invite-role-external"), h.Class("select"), h.Name("business_role"),
							data.Bind("invrole"),
							data.Attr("disabled", "$invkind != 'external'"),
							g.Group(crmRoleOpts(crmRolesExternal, "")),
						),
					}),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-primary"),
					data.Attr("disabled", "$invrole == ''"), g.Text("Daftar")),
			),
			h.P(h.Class("text-xs text-base-content/60 mt-2"),
				g.Text("Orang yang diundang otomatis bergabung saat login/daftar dengan email ini.")),
		),
	)
}

// inviteList = undangan pending. BL-170: kolom "Tautan" DIHAPUS dari UI — jalur
// utama kini auto-join saat login/daftar dengan email yang sama, bukan lagi
// klik-link. Link/token TETAP hidup di backend (inviteLink, /invite/{token})
// untuk kompatibilitas; InviteRow.Link masih diisi handler tapi tak dirender
// di sini. Kolom "Peran": role tenant tak lagi ditampilkan (selalu "member",
// BL-170) — diganti Peran CRM + Jenis Anggota, sumbu yang benar-benar dipilih
// pengundang, sama seperti roleBadges di tabel anggota.
func inviteList(base string, invites []InviteRow) g.Node {
	rows := make([]g.Node, 0, len(invites))
	for _, i := range invites {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4"), g.Text(i.Email)),
			h.Td(h.Class("py-2 pr-4"), inviteRoleBadges(i)),
			h.Td(h.Class("py-2 pr-4 text-xs text-base-content/60"), g.Text(i.Expires)),
			h.Td(h.Class("py-2"), h.FormEl(
				h.Method("post"),
				h.Action(base+"/members/invite/"+strconv.FormatInt(i.ID, 10)+"/delete"),
				h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline"), g.Text("Batal")),
			)),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Daftar Undangan")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Email")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Berlaku s/d")),
					h.Th(h.Class("py-2 font-medium"), g.Text("")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}
