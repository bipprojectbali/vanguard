package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// role_edit.go — view HALAMAN detail/edit SATU peran CRM
// (/w/{slug}/roles/{name}). Dipisah dari daftar (roles.go): daftar soal "peran
// apa saja ada", halaman ini soal "izin & cakupan satu peran". Murni-data:
// handler sudah menghitung level tiap sel & menandai peran sistem.

// RoleModulePerm = satu baris matriks (satu modul CRM) dalam sebuah peran.
// Level ∈ {none, read, write} — read & write dilipat jadi satu tingkat karena
// write MENCAKUP read (business.conf). Approve berdiri sendiri dan hanya
// bermakna bila CanApprove (Deals/Quotes/Renewal Management).
type RoleModulePerm struct {
	Obj        string
	Label      string
	CanApprove bool
	Level      string
	Approve    bool
}

// RoleCard = satu peran CRM workspace beserta matriksnya. Name = identitas mesin
// (subject Casbin, tak berubah), DisplayName = label layar. IsSystem (admin) →
// Modules nil dan dirender TERKUNCI: admin diwakili glob crm:* yang tak
// terpetakan ke kolom per-modul, jadi menampilkannya sebagai matriks akan keliru
// "semua None".
type RoleCard struct {
	Name        string
	DisplayName string
	Description string
	DataScope   string
	IsSystem    bool
	Modules     []RoleModulePerm
}

// levelOptions = pilihan tingkat izin per modul. Value = string mesin yang
// dibaca readRoleMatrix di handler ("read"/"write"; selain itu → dilewati = none).
var levelOptions = []ScopeOption{
	{Value: "none", Label: "Tak ada"},
	{Value: "read", Label: "Lihat"},
	{Value: "write", Label: "Kelola"},
}

// RoleEdit merender halaman satu peran: judul + link kembali + alert, lalu
// (peran sistem) keterangan terkunci ATAU (peran kustom) form sunting
// label/cakupan/matriks + form hapus. base = prefix URL workspace (dioper
// handler). canEdit=false → form terkunci & tombol simpan/hapus disembunyikan.
func RoleEdit(base string, rc RoleCard, scopes []ScopeOption, canEdit bool, errMsg, okMsg string) g.Node {
	body := []g.Node{
		h.A(h.Href(base+"/roles"), h.Class("link text-sm text-base-content/70"),
			g.Text("← Kembali ke daftar peran")),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.H1(h.Class("text-xl font-semibold"), g.Text(rc.DisplayName)),
			h.Span(h.Class("badge badge-neutral font-mono text-xs"), g.Text(rc.Name)),
			ui.When(rc.IsSystem, h.Span(h.Class("badge badge-warning"), g.Text("Sistem"))),
		),
	}
	if errMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "role-err", g.Text(errMsg)))
	}
	if okMsg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "role-ok", g.Text(okMsg)))
	}

	if rc.IsSystem {
		body = append(body,
			ui.When(rc.Description != "", h.P(h.Class("text-sm text-base-content/70"),
				g.Text(rc.Description))),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Peran bawaan dengan akses penuh ke semua modul CRM. "+
					"Tak bisa disunting atau dihapus.")))
		return roleEditShell(body)
	}

	body = append(body,
		h.FormEl(
			h.Method("post"), h.Action(base+"/roles/"+rc.Name),
			h.Class("grid gap-3 min-w-0"),
			h.Div(
				h.Class("grid gap-3 sm:grid-cols-2"),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Nama tampilan"),
					ui.Input(h.Name("display_name"), h.Type("text"),
						h.Value(rc.DisplayName), h.Required(), disabledIf(!canEdit)),
				),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Cakupan data"),
					scopeSelect("", "data_scope", rc.DataScope, scopes, !canEdit),
				),
			),
			h.Div(
				h.Class("grid gap-2"),
				ui.Label("Deskripsi (opsional)"),
				ui.Input(h.Name("description"), h.Type("text"),
					h.Value(rc.Description), h.MaxLength("200"),
					h.Placeholder("Ringkas peran ini dalam satu kalimat"),
					disabledIf(!canEdit)),
			),
			roleMatrix(rc, canEdit),
			quotesInheritNote(),
			ui.When(canEdit, h.Button(h.Type("submit"),
				h.Class("btn btn-primary min-h-11 justify-self-start"),
				g.Text("Simpan Perubahan"))),
		),
	)
	if canEdit {
		body = append(body, h.Div(
			h.Class("border-t border-base-300 pt-3 mt-1"),
			h.P(h.Class("text-sm text-base-content/70 mb-2"),
				g.Text("Menghapus peran akan mencabutnya dari semua anggota yang "+
					"memegangnya (mereka tak terhapus, hanya kehilangan peran ini).")),
			roleDeleteForm(base, rc.Name, "Hapus Peran"),
		))
	}
	return roleEditShell(body)
}

// quotesInheritNote = keterangan di bawah matriks bahwa Quotes TAK punya kolom
// tersendiri (BL-56): akses quote mewarisi izin Deals (quote nest di bawah deal),
// jadi hilangnya kolom bukan "fitur dicabut" melainkan kolom yang tak pernah
// ditegakkan. Teks statik → tetap murni-data.
func quotesInheritNote() g.Node {
	return h.P(h.Class("text-xs text-base-content/60"),
		g.Text("Quotes mengikuti izin Deals — peran yang boleh mengelola Deals "+
			"otomatis boleh membuat & menyunting Quote di bawahnya, jadi tak ada "+
			"kolom Quotes tersendiri."))
}

// roleEditShell = kartu tunggal berlebar terbatas (max-w-3xl) agar form editor
// tak melebar penuh di layar besar. min-w-0 agar tabel matriks di dalam bisa
// menyusut (konvensi mobile-first).
func roleEditShell(inner []g.Node) g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-3xl"),
		h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0 gap-3"), g.Group(inner)),
		),
	)
}

// roleMatrix = tabel izin per modul CRM. Dibungkus ui.TableScroll agar scroll
// terkurung, tak mendorong lebar halaman di mobile (konvensi mobile-first).
func roleMatrix(rc RoleCard, canEdit bool) g.Node {
	rows := make([]g.Node, 0, len(rc.Modules))
	for _, m := range rc.Modules {
		approveCell := g.Node(h.Span(h.Class("text-base-content/40"), g.Text("—")))
		if m.CanApprove {
			attrs := []g.Node{
				h.Type("checkbox"), h.Class("checkbox checkbox-sm"),
				h.Name("approve." + m.Obj), h.Value("1"),
			}
			if m.Approve {
				attrs = append(attrs, h.Checked())
			}
			if !canEdit {
				attrs = append(attrs, h.Disabled())
			}
			approveCell = h.Input(attrs...)
		}
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4"), g.Text(m.Label)),
			h.Td(h.Class("py-2 pr-4"), levelSelect(m.Obj, m.Level, !canEdit)),
			h.Td(h.Class("py-2 text-center"), approveCell),
		))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Modul")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Akses")),
			h.Th(h.Class("py-2 font-medium text-center"), g.Text("Setujui")),
		)),
		h.TBody(g.Group(rows)),
	))
}

// levelSelect = dropdown tingkat izin satu modul (name="level.<obj>").
func levelSelect(obj, current string, disabled bool) g.Node {
	attrs := []g.Node{h.Class("select select-sm"), h.Name("level." + obj)}
	if disabled {
		attrs = append(attrs, h.Disabled())
	}
	opts := make([]g.Node, 0, len(levelOptions))
	for _, o := range levelOptions {
		opts = append(opts, optionSel(o.Value, o.Label, current))
	}
	return h.Select(append(attrs, g.Group(opts))...)
}

// roleDeleteForm = hapus peran (form POST terpisah agar submit tak tertukar
// dengan sunting). Native POST → 303 (gotcha #16). label dibedakan pemanggil
// ("Hapus" ringkas di tabel, "Hapus Peran" di halaman detail).
func roleDeleteForm(base, name, label string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/roles/"+name+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text(label)),
	)
}
