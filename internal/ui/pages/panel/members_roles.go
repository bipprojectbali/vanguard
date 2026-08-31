package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// members_roles.go — badge & pemilih peran dua-sumbu (roleBadges, memberRoleSelect,
// crmRoleOpts/crmRoleOpt, memberIdent, roleOpts/roleOpt), dipisah dari members.go
// agar tiap file di bawah ambang tipe View/Component (300). Shell daftar anggota +
// form undang tetap di members.go — satu paket.

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
