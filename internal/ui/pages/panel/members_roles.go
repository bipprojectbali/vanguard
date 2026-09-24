package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// members_roles.go — badge & pemilih peran anggota (roleBadges, memberRoleSelect,
// memberKindRoleForm, crmRoleOpts/crmRoleOpt, memberIdent), dipisah dari members.go
// agar tiap file di bawah ambang tipe View/Component (300). Shell daftar anggota +
// form undang tetap di members.go — satu paket.

// roleBadges = tampilan baca-saja (dipakai saat bukan pengelola): peran CRM
// sebagai TEKS BIASA (label, bukan kode mesin — pakai BusinessRoleDisplay,
// bukan BusinessRole) tanpa bingkai badge, hanya bila diberikan (kosong = tak
// diberi); Jenis Anggota (kindBadge) selalu tampil sebagai badge — bukan
// nullable. Role TENANT (m.Role) SENGAJA tak ditampilkan di sini: bagi
// penglihat yang sampai ke halaman ini (canViewMembers, BL-171) nilainya
// nyaris selalu "member" dan tak menambah informasi apa pun.
func roleBadges(m MemberRow) g.Node {
	nodes := make([]g.Node, 0, 2)
	if m.BusinessRoleDisplay != "" {
		nodes = append(nodes, h.Span(h.Class("text-sm"), g.Text(m.BusinessRoleDisplay)))
	}
	nodes = append(nodes, kindBadge(m.Kind))
	return h.Div(h.Class("flex flex-wrap items-center gap-1"), g.Group(nodes))
}

// inviteRoleBadges = tampilan Peran satu undangan pending: Peran CRM (bila
// dipilih saat mengundang, TEKS BIASA — bukan badge, sama pola roleBadges) +
// Jenis Anggota (tetap badge). Role tenant TAK disertakan di sini (beda dari
// roleBadges) — undangan BL-170 selalu "member", jadi menampilkannya tak
// menambah informasi apa pun bagi pengundang.
func inviteRoleBadges(i InviteRow) g.Node {
	badges := []g.Node{}
	if i.BusinessRole != "" {
		badges = append(badges, h.Span(h.Class("text-sm"), g.Text(i.BusinessRole)))
	}
	badges = append(badges, kindBadge(i.Kind))
	return h.Div(h.Class("flex flex-wrap items-center gap-1"), g.Group(badges))
}

// memberRoleSelect = label kecil + select mungil dgn opsi SIAP-RENDER, dipakai
// form di baris anggota. Beda dari selectField (accounts_form.go) yang merakit
// opsi dari []string (value==label): opsi CRM di sini punya value≠label (Name
// mesin vs Display layar), jadi opsinya dibangun pemanggil. Grid agar label
// menempel di atas select; min-w-0 supaya tak memaksa tabel melebar di mobile.
// extra = atribut tambahan pada <select> (mis. data.Attr("disabled", ...) utk
// cascading select memberKindRoleForm) — kosong utk select biasa.
func memberRoleSelect(caption, name string, opts []g.Node, extra ...g.Node) g.Node {
	sel := append([]g.Node{h.Class("select select-sm"), h.Name(name)}, extra...)
	sel = append(sel, g.Group(opts))
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		h.Span(h.Class("text-xs text-base-content/60"), g.Text(caption)),
		h.Select(sel...),
	)
}

// memberKindRoleForm = SATU form Jenis Anggota + Peran CRM per baris anggota,
// SATU tombol Simpan — dulu dua form/tombol terpisah. Ubah Jenis Anggota
// otomatis menukar daftar Peran CRM yang ditawarkan DAN mereset pilihannya ke
// "(tak ada)" (cascading select, persis pola inviteForm/members.go: dua
// <select name="business_role"> memakai nama sama, hanya yg AKTIF—cocok kind
// terpilih—yang benar-benar terkirim; yg lain dinonaktifkan via data-attr
// disabled). Opsi PERTAMA crmRoleOpts selalu "(tak ada)"; select non-aktif tak
// dapat Selected() dari BusinessRole kind lama → browser jatuh ke opsi pertama
// itu begitu ditampilkan — reset otomatis TANPA JS tambahan. Backend
// (applyMemberKind + applyBusinessRole lewat MemberSetKind) tetap penjaga
// sesungguhnya; toggle ini murni UX. Signal per-baris ("mk"+id) agar banyak
// baris di halaman yang sama tak bentrok satu sama lain.
//
// canEditKind=false (BL-171: aktor bercakupan satu jenis saja) → Jenis
// Anggota dirender kindBadge READ-ONLY + input hidden (agar tetap terkirim
// apa adanya, tak berubah), Peran CRM TETAP select yang bisa disunting — dua
// sumbu berbeda (plan BL-171: "business_role tetap editable jika manage").
// sig tetap diinisialisasi ke m.Kind & tak pernah berubah (tak ada elemen yang
// men-trigger data.On("change") kind), jadi showWhen di bawah tetap menampilkan
// cabang Peran CRM yang cocok dengan kind anggota saat ini.
func memberKindRoleForm(base, id string, crmRolesInternal, crmRolesExternal []CRMRoleOption, m MemberRow, canEditKind bool) g.Node {
	sig := "mk" + id
	// rsig: dibagikan KEDUA select business_role (internal & external, sama pola
	// invrole/inviteForm) — peran CRM kini WAJIB dipilih, tombol Simpan disable
	// selama "" ("(tak ada)"). Diinisialisasi ke BusinessRole saat ini: anggota
	// yang SUDAH punya peran CRM bisa langsung Simpan (mis. hanya ganti Kind)
	// tanpa dipaksa memilih ulang peran yang sama.
	rsig := "mkr" + id
	kindField := g.Node(memberRoleSelect("Jenis Anggota", "kind", kindOpts(m.Kind), data.Bind(sig),
		// Ganti Kind → reset peran CRM: daftar pilihan berubah total, peran dari
		// kind sebelumnya tak valid utk kind baru (lihat catatan sama di inviteForm).
		data.On("change", "$"+rsig+" = ''")))
	if !canEditKind {
		kindField = g.Group([]g.Node{
			h.Div(h.Class("grid gap-1 min-w-0"),
				h.Span(h.Class("text-xs text-base-content/60"), g.Text("Jenis Anggota")),
				kindBadge(m.Kind),
			),
			h.Input(h.Type("hidden"), h.Name("kind"), h.Value(m.Kind)),
		})
	}
	return h.FormEl(
		h.Method("post"), h.Action(base+"/members/"+id+"/kind"),
		data.Signals(map[string]any{sig: m.Kind, rsig: m.BusinessRole}),
		// Mobile-first: tumpuk 1 kolom di ponsel, sejajar+wrap mulai sm.
		h.Class("flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-end"),
		kindField,
		showWhen("$"+sig+" == 'internal'", "",
			memberRoleSelect("Peran CRM", "business_role", crmRoleOpts(crmRolesInternal, m.BusinessRole),
				data.Bind(rsig), data.Attr("disabled", "$"+sig+" != 'internal'")),
		),
		showWhen("$"+sig+" == 'external'", "",
			memberRoleSelect("Peran CRM", "business_role", crmRoleOpts(crmRolesExternal, m.BusinessRole),
				data.Bind(rsig), data.Attr("disabled", "$"+sig+" != 'external'")),
		),
		h.Button(h.Type("submit"), h.Class("btn btn-sm self-end"),
			data.Attr("disabled", "$"+rsig+" == ''"), g.Text("Simpan")),
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

// kindBadge = tampilan baca-saja Jenis Anggota (BL-170), dipakai saat bukan
// pengelola. Kind bukan nullable ("" tak pernah terjadi dari DB, tapi jatuh ke
// Internal bila memang kosong — sama seperti default kolom).
func kindBadge(kind string) g.Node {
	label := "Internal"
	if kind == "external" {
		label = "Eksternal"
	}
	return h.Span(h.Class("badge badge-outline"), g.Text(label))
}

// kindOpts = opsi Jenis Anggota. Persis dua, TANPA opsi "(tak ada)" —
// beda dari crmRoleOpts: kind bukan nullable, setiap anggota selalu punya satu.
func kindOpts(current string) []g.Node {
	return []g.Node{
		kindOpt("internal", "Internal", current),
		kindOpt("external", "Eksternal", current),
	}
}

func kindOpt(val, label, current string) g.Node {
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
