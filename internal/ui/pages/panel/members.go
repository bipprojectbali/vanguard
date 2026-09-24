package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// MemberRow = satu anggota workspace (view).
//
// Email & Name sudah DIPUTUSKAN handler, bukan dipilih di sini: siapa berhak
// melihat apa adalah kebijakan privasi, dan view yang ikut memutuskannya berarti
// alamat asli tetap harus dikirim ke sini — tempat satu pemakaian yang lupa
// menyamarkan akan mengirimnya ke browser. Keduanya BOLEH kosong:
// Name kosong = orangnya tak punya nama tersimpan; Email kosong = penglihatnya
// tak berhak melihatnya sama sekali.
type MemberRow struct {
	UserID    int64
	Email     string
	Name      string
	Role      string
	AvatarURL string
	Status    string
	// BusinessRole = peran CRM saat ini, KODE MESIN (sumbu bisnis, tegak lurus
	// Role tenant); "" = belum diberi peran CRM. Dipakai memilih nilai awal
	// dropdown "Peran CRM" (memberKindRoleForm/crmRoleOpts mencocokkan ke value
	// opsi select, yang juga kode mesin) — BUKAN untuk ditampilkan ke layar.
	BusinessRole string
	// BusinessRoleDisplay = label manusia dari BusinessRole di atas (diterjemahkan
	// handler via ListBusinessRoles), dipakai tampilan baca-saja (roleBadges).
	// "" = sama seperti BusinessRole, belum diberi peran CRM.
	BusinessRoleDisplay string
	// Kind = Jenis Anggota saat ini ("internal"/"external", BL-170) — sumbu KETIGA,
	// tegak lurus Role & BusinessRole. Menentukan daftar Peran CRM mana (internal/
	// eksternal) yang ditawarkan dropdown baris ini. Kind & BusinessRole kini
	// SATU form/tombol Simpan (memberKindRoleForm): ubah Kind menukar cascading
	// select Peran CRM di klien, backend tetap penjaga sesungguhnya.
	Kind string
}

// CRMRoleOption = satu peran CRM yang boleh ditugaskan (sumber ListBusinessRoles).
// Name = identitas mesin (disimpan di memberships.business_role); Display = label
// layar. Diputuskan handler, bukan view (view murni-data).
type CRMRoleOption struct {
	Name    string
	Display string
}

// InviteRow = satu undangan pending. Link ditampilkan untuk DISALIN manual —
// pengiriman email belum ada (task terbuka, lihat handler/invite.go).
//
// Role tenant TAK ditampilkan (BL-170: undangan selalu "member", promosi
// terjadi pasca-join) — yang bermakna bagi pengundang adalah sumbu yang
// benar-benar ia pilih di form: BusinessRole (label TAMPILAN peran CRM, "" =
// tak diberi, sudah diterjemahkan handler dari nama mesin) & Kind
// (internal/eksternal).
type InviteRow struct {
	ID           int64
	Email        string
	BusinessRole string
	Kind         string
	Link         string
	Expires      string
}

// Members merender panel anggota workspace: daftar anggota (ubah role/keluarkan),
// form undang, dan daftar undangan pending. canManage=false → hanya lihat.
// selfID dipakai menyembunyikan aksi terhadap diri sendiri.
//
// crmRolesInternal/crmRolesExternal = Peran CRM yang boleh ditugaskan, DIPECAH
// per Jenis Anggota (BL-170) — cascading select form Undang & dropdown baris
// anggota masing-masing hanya menawarkan subset yang cocok kind-nya. base =
// prefix URL workspace ini (mis. "/w/acme"), DIOPER dari handler — view
// tak boleh merakit path sendiri: sejak 0004 setiap aksi bergantung slug, dan
// path yang di-hardcode di view akan diam-diam menunjuk workspace yang salah.
//
// canEditKind (BL-171): kolom/field Jenis Anggota boleh DIUBAH hanya bila
// true — aktor bercakupan (actorKindScope) KEDUA jenis. Bila false, baris
// anggota menampilkan kindBadge read-only (business_role TETAP editable bila
// canManage — dua hal berbeda, lihat memberKindRoleForm) dan form Undang
// mengunci Jenis Anggota ke satu nilai lewat input hidden alih-alih select
// (kindLockedTo menentukan nilainya; "" = canEditKind true, tak dikunci).
//
// selfIsAdmin (BL-171): baris anggota MILIK AKTOR SENDIRI hanya menawarkan
// form ubah Peran CRM/Jenis Anggota bila true (peran CRM aktor saat ini
// "admin") — dihitung handler (members_page.go) dari data yang sama,
// bukan di sini (view murni-data). Non-admin yang membuka baris dirinya
// sendiri melihat roleBadges read-only, sama seperti penglihat tanpa
// canManage; anggota LAIN tetap mengikuti canManage seperti biasa.
func Members(base string, crmRolesInternal, crmRolesExternal []CRMRoleOption, members []MemberRow, invites []InviteRow, canManage, canEditKind, selfIsAdmin bool, selfID int64, errMsg, okMsg string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Anggota")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Kelola siapa saja yang punya akses di sini.")),
	}
	if errMsg != "" {
		body = append(body, ui.Toast(ui.VariantDestructive, "members-err", g.Text(errMsg)))
	}
	if okMsg != "" {
		body = append(body, ui.Toast(ui.VariantSuccess, "members-ok", g.Text(okMsg)))
	}
	if canManage {
		body = append(body, inviteForm(base, crmRolesInternal, crmRolesExternal, canEditKind))
	}
	body = append(body, memberList(base, crmRolesInternal, crmRolesExternal, members, canManage, canEditKind, selfIsAdmin, selfID))
	if canManage && len(invites) > 0 {
		body = append(body, inviteList(base, invites))
	}
	// min-w-0 di wadah grid: tanpa ini, tabel/input lebar di dalamnya memaksa
	// grid melebar → halaman overflow horizontal di mobile (konvensi mobile-first).
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// MembersForbidden = halaman penolakan bagi anggota biasa yang membuka daftar
// anggota lewat URL langsung.
//
// Menyebut SIAPA yang bisa membantu, bukan sekadar "akses ditolak": orang yang
// membaca ini tak bisa berbuat apa-apa dengan penolakan telanjang, dan
// pertanyaan berikutnya di kepalanya selalu "lalu saya harus bagaimana".
func MembersForbidden() g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Anggota")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Daftar anggota hanya bisa dibuka oleh owner & admin workspace, "+
				"atau peran dengan akses \"User Management\" di Kelola Peran.")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi mereka bila Anda perlu mengundang seseorang atau "+
				"mengubah akses.")),
	)
}

// memberList = tabel anggota. Tabel dibungkus ui.TableScroll agar scroll-nya
// terkurung, tak mendorong lebar halaman di mobile (konvensi mobile-first).
func memberList(base string, crmRolesInternal, crmRolesExternal []CRMRoleOption, members []MemberRow, canManage, canEditKind, selfIsAdmin bool, selfID int64) g.Node {
	rows := make([]g.Node, 0, len(members))
	for _, m := range members {
		rows = append(rows, memberRow(base, crmRolesInternal, crmRolesExternal, m, canManage, canEditKind, selfIsAdmin, selfID))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Anggota")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					// "Nama", bukan "Email": kolomnya berisi nama orang (+ email sebagai
					// baris pendamping bagi yang berhak melihatnya).
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama")),
					// "Peran" (bukan "Role"): kolom memuat TIGA sumbu — role tenant,
					// peran CRM, Jenis Anggota (BL-170) — disimpan bersama SATU
					// tombol (memberKindRoleForm), bukan lagi kolom+tombol sendiri.
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
					h.Th(h.Class("py-2 font-medium"), g.Text("")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func memberRow(base string, crmRolesInternal, crmRolesExternal []CRMRoleOption, m MemberRow, canManage, canEditKind, selfIsAdmin bool, selfID int64) g.Node {
	id := strconv.FormatInt(m.UserID, 10)
	roleCell := roleBadges(m)
	action := g.Node(g.Text(""))
	// editableRow (BL-171): baris MILIK AKTOR SENDIRI hanya boleh disunting bila
	// peran CRM-nya saat ini "admin" (selfIsAdmin, dihitung handler) — mencegah
	// non-admin (mis. Field Officer) mengubah/mengunci perannya sendiri lewat
	// dropdown yang sama dipakai mengelola anggota lain. Anggota LAIN tetap
	// mengikuti canManage seperti biasa, tak terpengaruh guard ini.
	editableRow := m.UserID != selfID || selfIsAdmin
	if canManage && editableRow {
		// SATU form Peran CRM + Jenis Anggota, SATU tombol Simpan
		// (memberKindRoleForm, members_roles.go) — dulu dua form/tombol
		// terpisah; digabung agar ubah Jenis Anggota & Peran CRM tersimpan
		// dalam satu POST (MemberSetKind menilai dua sumbu, pola sama
		// MemberSetRole lama utk role+business_role). Role TENANT tetap tak
		// disentuh di baris ini (diubah lewat /dev/users, BL-135); sumbu CRM
		// boleh menyentuh diri sendiri HANYA bila aktor admin (editableRow di
		// atas) — sebelumnya opt-in owner ke CRM tanpa syarat, kini dibatasi
		// agar non-admin tak mengubah perannya sendiri. canEditKind (BL-171):
		// aktor bercakupan satu jenis saja melihat Jenis Anggota read-only
		// (kindBadge) di sini, tapi Peran CRM TETAP bisa disunting — dua sumbu
		// berbeda, lihat memberKindRoleForm.
		roleCell = memberKindRoleForm(base, id, crmRolesInternal, crmRolesExternal, m, canEditKind)
	}
	if canManage {
		// Keluarkan tak pernah terhadap diri sendiri (cegah mengunci diri keluar).
		if m.UserID != selfID {
			action = h.FormEl(
				h.Method("post"), h.Action(base+"/members/"+id+"/remove"),
				h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline"),
					g.Text("Keluarkan")),
			)
		}
	}
	return h.Tr(
		h.Class("border-b border-base-300/50"),
		h.Td(h.Class("py-2 pr-4"), h.Div(
			h.Class("flex items-center gap-2 min-w-0"),
			ui.Avatar(m.AvatarURL, m.Name, m.Email, 32),
			memberIdent(m),
		)),
		h.Td(h.Class("py-2 pr-4"), roleCell),
		h.Td(h.Class("py-2"), action),
	)
}

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
