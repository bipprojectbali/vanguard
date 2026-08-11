package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// roles.go — view DAFTAR peran CRM (/w/{slug}/roles): alert + form tambah peran +
// TABEL peran. Tiap baris menuju halaman detail/edit (role_edit.go) atau dihapus
// langsung dari baris. Murni-data: handler menyiapkan baris siap-render (label
// cakupan sudah diterjemahkan), view tak memanggil authz.

// RoleRow = satu baris tabel daftar peran (wireframe 9.2): identitas + deskripsi +
// jumlah pemegang + penanda sistem. Matriks izin & cakupan data TAK di sini — ada
// di halaman detail (RoleEdit), agar daftar tetap satu query ringan & tabelnya tak
// melebar. ScopeLabel dipertahankan handler (bisa dipakai kelak) tapi tak jadi
// kolom: wireframe menaruh cakupan di kartu Record Ownership Rules, bukan tabel.
type RoleRow struct {
	Name        string
	DisplayName string
	Description string
	ScopeLabel  string
	MemberCount int64
	IsSystem    bool
}

// ScopeOption = satu pilihan cakupan data F3 (value mesin + label Indonesia).
type ScopeOption struct {
	Value string
	Label string
}

// Roles merender panel daftar: alert, form tambah peran (bila canEdit), lalu
// tabel peran. base = prefix URL workspace (dioper handler — view tak merakit
// path sendiri, konvensi 0004). canEdit=false (workspace read-only/arsip) → form
// tambah & aksi hapus disembunyikan; daftar tetap bisa dibuka (Detail).
func Roles(base string, rows []RoleRow, scopes []ScopeOption, canEdit bool, errMsg, okMsg string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Peran CRM")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Atur peran bisnis workspace ini dan izin tiap modul CRM.")),
	}
	if errMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "roles-err", g.Text(errMsg)))
	}
	if okMsg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "roles-ok", g.Text(okMsg)))
	}
	if canEdit {
		body = append(body, roleCreateForm(base, scopes))
	}
	body = append(body, rolesTableCard(base, rows, canEdit))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// RolesForbidden = penolakan bagi anggota tanpa izin crm:roles yang membuka
// panel lewat URL langsung. Menyebut SIAPA yang bisa membantu, bukan sekadar
// "akses ditolak" (pola MembersForbidden).
func RolesForbidden() g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Peran CRM")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Pengaturan peran hanya bisa dibuka oleh admin CRM workspace.")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi mereka bila Anda perlu mengubah izin atau menambah peran.")),
	)
}

// rolesTableCard membungkus tabel dalam kartu. Kosong (mustahil di praktik —
// peran sistem "admin" selalu ter-seed) tetap ditangani agar tak render tabel
// tanpa baris.
func rolesTableCard(base string, rows []RoleRow, canEdit bool) g.Node {
	var inner g.Node
	if len(rows) == 0 {
		inner = h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Belum ada peran."))
	} else {
		inner = rolesTable(base, rows, canEdit)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Daftar Peran")),
			inner,
		),
	)
}

// rolesTable = daftar peran sebagai tabel. Dibungkus ui.TableScroll (WAJIB tiap
// <table>, konvensi mobile-first): overflow terkurung, tak mendorong lebar
// halaman di 375px. Aksi per baris: "Detail" (link GET — bookmarkable, lolos
// gotcha #16) selalu ada; "Hapus" hanya peran kustom saat workspace bisa ditulis.
// Peran sistem tak bisa dihapus → form-nya TAK dirender (bukan disembunyikan CSS).
func rolesTable(base string, rows []RoleRow, canEdit bool) g.Node {
	trs := make([]g.Node, 0, len(rows))
	for _, r := range rows {
		// Penanda Sistem/Kustom di SAMPING nama (seperti wireframe 9.2), bukan kolom
		// tersendiri — hemat lebar & sesuai rancangan.
		jenis := h.Span(h.Class("badge badge-ghost badge-sm"), g.Text("Kustom"))
		if r.IsSystem {
			jenis = h.Span(h.Class("badge badge-warning badge-sm"), g.Text("Sistem"))
		}
		desc := g.Node(h.Span(h.Class("text-base-content/40"), g.Text("—")))
		if r.Description != "" {
			desc = h.Span(h.Class("text-sm text-base-content/70"), g.Text(r.Description))
		}
		actions := []g.Node{
			h.A(h.Href(base+"/roles/"+r.Name),
				h.Class("btn btn-sm btn-ghost min-h-11"), g.Text("Detail")),
		}
		if canEdit && !r.IsSystem {
			actions = append(actions, roleDeleteForm(base, r.Name, "Hapus"))
		}
		trs = append(trs, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4"),
				h.Div(h.Class("flex flex-col gap-1"),
					h.Div(h.Class("flex items-center gap-2"),
						h.Span(h.Class("font-medium"), g.Text(r.DisplayName)),
						jenis,
					),
					h.Span(h.Class("font-mono text-xs text-base-content/60"), g.Text(r.Name)),
				)),
			h.Td(h.Class("py-2 pr-4 max-w-xs"), desc),
			h.Td(h.Class("py-2 pr-4 text-sm whitespace-nowrap"), g.Textf("%d user", r.MemberCount)),
			h.Td(h.Class("py-2"),
				h.Div(h.Class("flex flex-wrap gap-2 justify-end"), g.Group(actions))),
		))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Deskripsi")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jumlah User")),
			h.Th(h.Class("py-2 font-medium text-right"), g.Text("Aksi")),
		)),
		h.TBody(g.Group(trs)),
	))
}

// roleCreateForm = buat peran baru (matriks kosong; izinnya disunting di halaman
// detail). Native POST → 303 (gotcha #16). data_scope dipilih saat buat karena F3
// melekat pada peran, bukan pada izin per-modul.
func roleCreateForm(base string, scopes []ScopeOption) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(
			h.Class("card-body"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Tambah Peran")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/roles"),
				h.Class("grid gap-3 sm:grid-cols-3 sm:items-end"),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Nama (huruf kecil)", h.For("role-name")),
					ui.Input(h.ID("role-name"), h.Name("name"), h.Type("text"),
						h.Placeholder("mis. finance"),
						h.Pattern("[a-z][a-z0-9_]{1,31}"), h.Required()),
				),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Nama tampilan", h.For("role-display")),
					ui.Input(h.ID("role-display"), h.Name("display_name"), h.Type("text"),
						h.Placeholder("mis. Keuangan"), h.Required()),
				),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Cakupan data", h.For("role-scope")),
					scopeSelect("role-scope", "data_scope", "", scopes, false),
				),
				h.Div(
					h.Class("grid gap-2 sm:col-span-3"),
					ui.Label("Deskripsi (opsional)", h.For("role-desc")),
					ui.Input(h.ID("role-desc"), h.Name("description"), h.Type("text"),
						h.Placeholder("mis. Leads, deals, quotes — hanya desa yang ditugaskan"),
						h.MaxLength("200")),
				),
				h.Div(
					h.Class("sm:col-span-3"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Tambah Peran")),
				),
			),
			h.P(h.Class("text-xs text-base-content/60 mt-2"),
				g.Text("Nama dipakai sistem dan tak bisa diubah setelah dibuat. "+
					"Izin modul diatur di halaman detail peran.")),
		),
	)
}

// scopeSelect = dropdown cakupan data F3. id opsional (kosong = tak dipasang).
// Dipakai form tambah (roles.go) DAN editor peran (role_edit.go).
func scopeSelect(id, name, current string, scopes []ScopeOption, disabled bool) g.Node {
	attrs := []g.Node{h.Class("select"), h.Name(name)}
	if id != "" {
		attrs = append(attrs, h.ID(id))
	}
	if disabled {
		attrs = append(attrs, h.Disabled())
	}
	opts := make([]g.Node, 0, len(scopes))
	for _, o := range scopes {
		opts = append(opts, optionSel(o.Value, o.Label, current))
	}
	return h.Select(append(attrs, g.Group(opts))...)
}

// optionSel = <option> dengan selected bila value == current.
func optionSel(val, label, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(label))...)
}

// disabledIf mengembalikan atribut disabled bila cond, else node kosong (agar
// bisa disisipkan langsung ke daftar atribut tanpa cabang di pemanggil).
func disabledIf(cond bool) g.Node {
	if cond {
		return h.Disabled()
	}
	return g.Text("")
}
