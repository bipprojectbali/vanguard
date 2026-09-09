package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// field_security.go — SECTION "Field Security" di dalam halaman Peran CBM (/roles,
// ADR 0012 opsi B): matriks business_role × {lihat nomor penuh, boleh sunting} untuk
// HP & WhatsApp. SATU form replace-all (pola RoleUpdate, bukan satu-form-per-baris
// seperti code_formats): kebijakan semua peran disimpan sebagai satu transaksi
// ganti-total, jadi wajar dikirim sekali. Native POST ke /field-security → 303
// (gotcha #16). Murni-data: handler menyiapkan state kotak centang (dari kebijakan
// yang BERLAKU), view tak memanggil authz/fls.
//
// Digabung ke halaman /roles (bukan halaman Settings tersendiri) demi satu pintu
// pengaturan per-peran; tapi FORM, POST, penyimpanan (field_security_policies), &
// gerbang (crm:field_security) tetap TERPISAH dari matriks Casbin /roles — sumbu F4
// & F2 tak dicampur di satu transaksi (fls.go:11-32). fieldSecuritySection dipanggil
// dari view roles.go, bukan dirender sebagai halaman sendiri.
//
// Dua kolom = dua sumbu terpisah (edit⇒view): peran bisa "lihat" tanpa "sunting"
// (mempertahankan realita Admin=lihat-saja). Backend meng-coerce view=view||edit,
// jadi "sunting tanpa lihat" tak pernah tersimpan; enhancement JS opsional bisa
// auto-centang view saat edit dicentang, tapi kebenaran tetap di backend.

// FieldSecurityRoleRow = satu baris peran siap-render: identitas + state awal dua
// kotak centang. IsSystem hanya penanda tampilan (peran "admin" terkunci) — admin
// tetap bisa disimpan barisnya (defaultnya lihat+—), tak diperlakukan khusus di form.
type FieldSecurityRoleRow struct {
	Name         string // nilai mesin business_role — dikirim di nama field (view.<Name>)
	DisplayName  string // label layar
	IsSystem     bool
	CanViewPhone bool
	CanEditPhone bool
}

// FieldSecurityView = data siap-render section. Tanpa Msg/Err: alert dirender oleh
// halaman /roles yang menampungnya (satu region alert bersama), bukan section ini.
type FieldSecurityView struct {
	Base    string // prefix URL workspace ("/w/acme") — dari handler, view tak merakit path
	CanEdit bool   // read-only/arsip → matriks tampil, tombol simpan disembunyikan
	Roles   []FieldSecurityRoleRow
}

// fieldSecuritySection merender BLOK Field Security untuk ditanam di halaman /roles
// (di bawah tabel peran). Judul section (h2) + penjelasan + kartu form matriks; TANPA
// H1 & TANPA alert — halaman /roles sudah punya satu region alert bersama (sukses
// fsec_saved & galat lewat okMsg/errMsg-nya), jadi menaruh alert kedua di sini akan
// menggandakannya. Dipanggil dari Roles() bila handler menyertakan *FieldSecurityView.
func fieldSecuritySection(v FieldSecurityView) g.Node {
	return h.Div(
		h.Class("grid gap-2 min-w-0 mt-2"),
		h.H2(h.Class("text-lg font-semibold"), g.Text("Field Security")),
		h.P(h.Class("text-base-content/70 mb-1"),
			g.Text("Atur peran bisnis mana yang boleh melihat nomor HP & WhatsApp secara "+
				"penuh, dan mana yang boleh menyuntingnya. Berlaku untuk Kontak, Prospek "+
				"(Lead), dan formulir Konversi Lead.")),
		h.P(h.Class("text-xs text-base-content/60 mb-2"),
			g.Text("Peran yang tak boleh melihat nomor akan menerima tampilan tersamar "+
				"(•••). \"Boleh sunting\" otomatis mengikutkan \"lihat nomor penuh\" — nomor "+
				"yang tak terlihat mustahil disunting.")),
		fieldSecurityCard(v),
	)
}

// fieldSecurityCard = kartu berisi tabel matriks. canEdit=false → tabel tampil
// (kotak disabled), tanpa <form>/tombol simpan (pola codeFormatCard hanya-lihat).
func fieldSecurityCard(v FieldSecurityView) g.Node {
	table := fieldSecurityMatrix(v)
	inner := []g.Node{
		h.H2(h.Class("font-semibold"), g.Text("Matriks Peran")),
		table,
	}
	if v.CanEdit {
		inner = append(inner,
			h.Div(h.Class("flex flex-wrap items-center gap-2 mt-1 min-w-0"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary btn-sm min-h-11"),
					g.Text("Simpan")),
			),
		)
		return h.FormEl(
			h.Method("post"), h.Action(v.Base+"/field-security"),
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(append([]g.Node{h.Class("card-body min-w-0 gap-3")}, inner...)...),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(append([]g.Node{h.Class("card-body min-w-0 gap-3")}, inner...)...),
	)
}

// fieldSecurityMatrix = tabel peran × {lihat, sunting}. Dibungkus ui.TableScroll
// (WAJIB tiap <table>, konvensi mobile-first) agar tak mendorong lebar di 375px.
func fieldSecurityMatrix(v FieldSecurityView) g.Node {
	rows := make([]g.Node, 0, len(v.Roles))
	for _, r := range v.Roles {
		jenis := g.Node(g.Text(""))
		if r.IsSystem {
			jenis = h.Span(h.Class("badge badge-warning badge-sm ml-2"), g.Text("Sistem"))
		}
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4"),
				h.Div(h.Class("flex items-center"),
					h.Span(h.Class("font-medium"), g.Text(r.DisplayName)),
					jenis,
				),
				h.Div(h.Class("font-mono text-xs text-base-content/60"), g.Text(r.Name)),
			),
			h.Td(h.Class("py-2 pr-4 text-center"),
				fieldSecurityCheck("view."+r.Name, r.CanViewPhone, v.CanEdit)),
			h.Td(h.Class("py-2 text-center"),
				fieldSecurityCheck("edit."+r.Name, r.CanEditPhone, v.CanEdit)),
		))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
			h.Th(h.Class("py-2 pr-4 font-medium text-center"), g.Text("Lihat nomor penuh")),
			h.Th(h.Class("py-2 font-medium text-center"), g.Text("Boleh sunting")),
		)),
		h.TBody(g.Group(rows)),
	))
}

// fieldSecurityCheck = satu kotak-centang matriks (name=view.<role>/edit.<role>,
// value=1). Terkunci saat !canEdit. Tap target ≥44px lewat padding baris + ukuran
// checkbox (konvensi mobile-first).
func fieldSecurityCheck(name string, checked, canEdit bool) g.Node {
	attrs := []g.Node{
		h.Type("checkbox"), h.Class("checkbox checkbox-sm"),
		h.Name(name), h.Value("1"),
	}
	if checked {
		attrs = append(attrs, h.Checked())
	}
	if !canEdit {
		attrs = append(attrs, h.Disabled())
	}
	return h.Input(attrs...)
}
