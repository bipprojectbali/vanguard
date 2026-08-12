package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
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
	// BusinessRole = peran CRM saat ini (sumbu bisnis, tegak lurus Role tenant);
	// "" = belum diberi peran CRM. Dipakai memilih nilai awal dropdown "Peran CRM".
	BusinessRole string
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
type InviteRow struct {
	ID      int64
	Email   string
	Role    string
	Link    string
	Expires string
}

// Members merender panel anggota workspace: daftar anggota (ubah role/keluarkan),
// form undang, dan daftar undangan pending. canManage=false → hanya lihat.
// selfID dipakai menyembunyikan aksi terhadap diri sendiri.
//
// roles = role yang boleh dipilih (mode single tak memuat "owner"). base =
// prefix URL workspace ini (mis. "/w/acme"), DIOPER dari handler — view
// tak boleh merakit path sendiri: sejak 0004 setiap aksi bergantung slug, dan
// path yang di-hardcode di view akan diam-diam menunjuk workspace yang salah.
func Members(base string, roles []string, crmRoles []CRMRoleOption, members []MemberRow, invites []InviteRow, canManage bool, selfID int64, errMsg, okMsg string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Anggota Workspace")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Kelola siapa saja yang punya akses ke workspace ini.")),
	}
	if errMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "members-err", g.Text(errMsg)))
	}
	if okMsg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "members-ok", g.Text(okMsg)))
	}
	if canManage {
		body = append(body, inviteForm(base))
	}
	body = append(body, memberList(base, roles, crmRoles, members, canManage, selfID))
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
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Anggota Workspace")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Daftar anggota hanya bisa dibuka oleh owner & admin workspace.")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi mereka bila Anda perlu mengundang seseorang atau "+
				"mengubah akses.")),
	)
}

// memberList = tabel anggota. Tabel dibungkus ui.TableScroll agar scroll-nya
// terkurung, tak mendorong lebar halaman di mobile (konvensi mobile-first).
func memberList(base string, roles []string, crmRoles []CRMRoleOption, members []MemberRow, canManage bool, selfID int64) g.Node {
	rows := make([]g.Node, 0, len(members))
	for _, m := range members {
		rows = append(rows, memberRow(base, roles, crmRoles, m, canManage, selfID))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Anggota")),
			// Ketiadaan email WAJIB dijelaskan. Tanpa keterangan ini, baris tanpa
			// alamat (atau "mal•••@gmail.com" bagi yang tak punya nama) terbaca
			// seperti data rusak — dan orang akan melaporkannya sebagai bug alih-alih
			// memahaminya sebagai perlindungan.
			g.If(!canManage, h.P(
				h.Class("text-xs text-base-content/60 mb-2"),
				g.Text("Alamat email rekan disembunyikan. Hanya owner & admin "+
					"workspace yang melihatnya."),
			)),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					// "Anggota", bukan "Email": kolomnya kini berisi nama orang, dan
					// bagi anggota biasa email tak muncul di sana sama sekali.
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Anggota")),
					// "Peran" (bukan "Role"): kolom kini memuat DUA sumbu — role
					// tenant + peran CRM — yang disimpan bersama satu tombol.
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
					h.Th(h.Class("py-2 font-medium"), g.Text("")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func memberRow(base string, roles []string, crmRoles []CRMRoleOption, m MemberRow, canManage bool, selfID int64) g.Node {
	id := strconv.FormatInt(m.UserID, 10)
	roleCell := roleBadges(m)
	action := g.Node(g.Text(""))
	if canManage {
		// SATU form, DUA sumbu: role tenant + peran CRM disimpan sekali klik
		// (POST .../role menilai tiap sumbu dgn guard-nya sendiri). Sumbu tenant
		// DISEMBUNYIKAN untuk diri sendiri — cegah menurunkan/mengunci diri, jaga
		// owner terakhir. Sumbu CRM tetap ADA di baris sendiri: itu opt-in owner
		// ke CRM (penugasan business_role, gerbang = canManageMembers sumbu tenant).
		fields := make([]g.Node, 0, 3)
		if m.UserID != selfID {
			fields = append(fields, memberRoleSelect("Role", "role", roleOpts(roles, m.Role)))
		}
		fields = append(fields, memberRoleSelect("Peran CRM", "business_role", crmRoleOpts(crmRoles, m.BusinessRole)))
		fields = append(fields, h.Button(h.Type("submit"), h.Class("btn btn-sm self-end"), g.Text("Simpan")))
		roleCell = h.FormEl(
			h.Method("post"), h.Action(base+"/members/"+id+"/role"),
			// Mobile-first: tumpuk 1 kolom di ponsel, sejajar+wrap mulai sm.
			h.Class("flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-end"),
			g.Group(fields),
		)
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

// roleBadges = tampilan baca-saja dua sumbu (dipakai saat bukan pengelola): role
// tenant selalu tampil; peran CRM hanya bila diberikan (kosong = tak diberi).
func roleBadges(m MemberRow) g.Node {
	badges := []g.Node{h.Span(h.Class("badge badge-neutral"), g.Text(m.Role))}
	if m.BusinessRole != "" {
		badges = append(badges, h.Span(h.Class("badge badge-ghost"), g.Text(m.BusinessRole)))
	}
	return h.Div(h.Class("flex flex-wrap gap-1"), g.Group(badges))
}

// memberSelect = label kecil + select mungil dgn opsi SIAP-RENDER, dipakai form
// dua-sumbu di baris anggota. Beda dari selectField (accounts_form.go) yang
// merakit opsi dari []string (value==label): opsi CRM di sini punya value≠label
// (Name mesin vs Display layar), jadi opsinya dibangun pemanggil. Grid agar label
// menempel di atas select; min-w-0 supaya tak memaksa tabel melebar di mobile.
func memberRoleSelect(caption, name string, opts []g.Node) g.Node {
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		h.Span(h.Class("text-xs text-base-content/60"), g.Text(caption)),
		h.Select(h.Class("select select-sm"), h.Name(name), g.Group(opts)),
	)
}

// crmRoleOpts = opsi peran CRM. Opsi PERTAMA value="" = "(tak ada)": memilihnya
// mencabut peran CRM (business_role → NULL di handler). Sisanya dari daftar peran
// workspace (ListBusinessRoles), Display sebagai label, Name sebagai nilai.
func crmRoleOpts(crmRoles []CRMRoleOption, current string) []g.Node {
	out := make([]g.Node, 0, len(crmRoles)+1)
	out = append(out, crmRoleOpt("", "(tak ada)", current))
	for _, c := range crmRoles {
		out = append(out, crmRoleOpt(c.Name, c.Display, current))
	}
	return out
}

func crmRoleOpt(val, label, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(label))...)
}

// memberIdent merender penanda orang: nama sebagai baris utama, email sebagai
// baris pendamping yang lebih kecil.
//
// Keduanya bisa kosong, dan itu bukan kasus tepi yang jarang: akun password dev
// tak punya nama, sementara anggota biasa tak menerima email rekannya sama
// sekali. Jadi bentuknya diturunkan dari apa yang ADA, bukan dari satu tata
// letak yang mengasumsikan keduanya hadir — dua baris hanya bila memang ada dua
// hal untuk ditampilkan, agar tak ada baris kosong yang menggantung.
//
// `truncate` + `min-w-0` di keduanya: nama & email sama-sama bisa panjang, dan
// tanpa ini ia mendorong lebar tabel di mobile (konvensi mobile-first).
func memberIdent(m MemberRow) g.Node {
	switch {
	case m.Name == "":
		return h.Span(h.Class("truncate"), g.Text(m.Email))
	case m.Email == "":
		return h.Span(h.Class("truncate"), g.Text(m.Name))
	}
	return h.Div(
		h.Class("min-w-0"),
		h.Div(h.Class("truncate"), g.Text(m.Name)),
		h.Div(h.Class("truncate text-xs text-base-content/60"), g.Text(m.Email)),
	)
}

// roleOpts merender opsi role yang BOLEH diberikan — daftarnya dioper handler,
// bukan diputuskan di sini: mode single tak mengenal `owner` (0006 §7), dan view
// tak boleh menanyakan mode aplikasi (konvensi view murni-data).
func roleOpts(roles []string, current string) []g.Node {
	out := make([]g.Node, 0, len(roles))
	for _, r := range roles {
		out = append(out, roleOpt(r, current))
	}
	return out
}

func roleOpt(val, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(val))...)
}

// inviteForm = undang lewat email. Form NATIVE POST → 303 (gotcha #16).
func inviteForm(base string) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(
			h.Class("card-body"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Undang Anggota")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/members/invite"),
				h.Class("flex flex-col gap-2 sm:flex-row sm:items-end"),
				h.Div(
					h.Class("grid gap-2 flex-1"),
					ui.Label("Email", h.For("invite-email")),
					ui.Input(h.ID("invite-email"), h.Name("email"), h.Type("email"),
						h.Placeholder("nama@contoh.com")),
				),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Role", h.For("invite-role")),
					h.Select(h.ID("invite-role"), h.Class("select"), h.Name("role"),
						h.Option(h.Value("member"), g.Text("member")),
						h.Option(h.Value("admin"), g.Text("admin")),
					),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-primary"), g.Text("Undang")),
			),
			h.P(h.Class("text-xs text-base-content/60 mt-2"),
				g.Text("Email belum dikirim otomatis — salin tautan undangan di bawah dan kirim manual.")),
		),
	)
}

// inviteList = undangan pending + tautannya (untuk disalin manual).
func inviteList(base string, invites []InviteRow) g.Node {
	rows := make([]g.Node, 0, len(invites))
	for _, i := range invites {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4"), g.Text(i.Email)),
			h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-neutral"), g.Text(i.Role))),
			h.Td(h.Class("py-2 pr-4"), ui.Input(
				h.Type("text"), h.Value(i.Link), h.ReadOnly(),
				h.Class("input input-sm w-full min-w-[16rem] font-mono text-xs"),
			)),
			h.Td(h.Class("py-2 pr-4 text-xs text-base-content/60"), g.Text(i.Expires)),
			h.Td(h.Class("py-2"), h.FormEl(
				h.Method("post"),
				h.Action(base+"/members/invite/"+strconv.FormatInt(i.ID, 10)+"/delete"),
				h.Button(h.Type("submit"), h.Class("btn btn-sm btn-ghost"), g.Text("Batal")),
			)),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Undangan Menunggu")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Email")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Role")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tautan")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Berlaku s/d")),
					h.Th(h.Class("py-2 font-medium"), g.Text("")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}
