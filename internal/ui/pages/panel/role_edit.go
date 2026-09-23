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
	CanARR     bool // true → sel "Lihat ARR" dirender (hanya Subscriptions, BL-58)
	// ARREligible: true → level modul INI ikut menentukan buka/tutup checkbox
	// "Lihat Nilai Kontrak" (authz.ARRGateObjects, disalin handler dari sana) —
	// checkbox itu sendiri TETAP satu, dirender hanya di baris CanARR.
	ARREligible bool
	// WriteEnforced: true → backend punya titik enforcement "write" nyata utk
	// modul ini, opsi "Kelola" dirender (levelOpts, role_edit_levels.go).
	// false → hanya "Tak ada"/"Lihat" (authz.ModuleWriteEnforced, disalin
	// handler dari sana; lihat rasional lengkap di role_edit_levels.go).
	WriteEnforced bool
	Level         string
	Approve       bool
	ARR           bool // tersetel → kotak "Lihat ARR" tercentang
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
	Kind        string
	IsSystem    bool
	Modules     []RoleModulePerm
}

// RoleEdit merender halaman satu peran: judul + link kembali + alert, lalu
// (peran sistem) keterangan terkunci ATAU (peran kustom) SATU form sunting
// label/cakupan/matriks/Pengaturan Tambahan (+ Field Security bila fsec != nil)
// dalam SATU submit (BL-145 subtask 7), plus kartu "Zona Berbahaya" (Hapus
// Peran) TERPISAH — form hapus sengaja tak pernah ikut form sunting supaya
// submit yang tak disengaja tak bisa memicu hapus. base = prefix URL workspace
// (dioper handler). canEdit=false → form terkunci & tombol simpan/hapus
// disembunyikan. fsec nil → peninjau tak berwenang crm:field_security, section
// Field Security tak dirender sama sekali.
func RoleEdit(base string, rc RoleCard, scopes []ScopeOption, canEdit bool, errMsg, okMsg string, fsec *FieldSecurityRoleView) g.Node {
	header := []g.Node{
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
		header = append(header, ui.Toast(ui.VariantDestructive, "role-err", g.Text(errMsg)))
	}
	if okMsg != "" {
		header = append(header, ui.Toast(ui.VariantSuccess, "role-ok", g.Text(okMsg)))
	}

	if rc.IsSystem {
		body := append(header,
			ui.When(rc.Description != "", h.P(h.Class("text-sm text-base-content/70"),
				g.Text(rc.Description))),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Peran bawaan dengan akses penuh ke semua modul CRM. "+
					"Tak bisa disunting atau dihapus.")))
		if fsec != nil {
			// Admin diwakili glob crm:* — akses penuh, statis, tanpa guard reaktif
			// (tak ada matriks Contacts/Leads untuk direaksikan). Peran sistem
			// tak punya matriks utk dilebur dgn Field Security, jadi bentuknya
			// TETAP form berdiri sendiri (satu-satunya form di halaman ini).
			body = append(body, fieldSecurityRoleSection(*fsec))
		}
		// Indikator Nilai Kontrak/MRR TAK bergantung gate crm:field_security
		// (§3b) — admin selalu "Terlihat" (akses penuh crm:*), statis.
		body = append(body, contractValueIndicator(true, false))
		return roleEditShell(body)
	}

	actx := buildARRCrossModuleCtx(rc)
	settingsForm := h.FormEl(
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
			h.Div(
				h.Class("grid gap-2"),
				ui.Label("Jenis Anggota"),
				kindSelect("", "kind", rc.Kind, !canEdit),
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
		roleMatrix(rc, canEdit, flsReactive(fsec), actx),
		additionalSettings(rc, canEdit, fsec, actx),
		ui.When(canEdit, h.Button(h.Type("submit"),
			h.Class("btn btn-primary min-h-11 justify-self-start"),
			g.Text("Simpan Perubahan"))),
	)

	cards := [][]g.Node{{settingsForm}}
	if canEdit {
		cards = append(cards, []g.Node{
			h.H2(h.Class("font-semibold text-error"), g.Text("Zona Berbahaya")),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Menghapus peran akan mencabutnya dari semua anggota yang "+
					"memegangnya (mereka tak terhapus, hanya kehilangan peran ini).")),
			roleDeleteForm(base, rc.Name, "Hapus Peran"),
		})
	}
	return roleEditShellCards(header, cards...)
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

// roleEditShellCards = varian roleEditShell utk peran KUSTOM (BL-145 subtask
// 7): header (link kembali/judul/badge/toast) dirender DI LUAR kartu apa pun,
// lalu tiap slice di cards dibungkus KARTUNYA SENDIRI. Dipakai memisahkan
// SECARA VISUAL form Pengaturan (identitas+matriks+Pengaturan Tambahan+Field
// Security, SATU submit) dari Zona Berbahaya (Hapus Peran, form POST
// TERPISAH) — dua form tak pernah berbagi satu card, walau satu halaman.
func roleEditShellCards(header []g.Node, cards ...[]g.Node) g.Node {
	all := make([]g.Node, 0, len(header)+len(cards))
	all = append(all, header...)
	for _, inner := range cards {
		all = append(all, h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0 gap-3"), g.Group(inner)),
		))
	}
	return h.Div(h.Class("grid gap-4 min-w-0 max-w-3xl"), g.Group(all))
}

// flsReactive menentukan apakah baris crm:contacts/crm:leads perlu klausa
// reset silang ke $fls_view/$fls_edit (BL-145 subtask 3) — hanya bila section
// Field Security DIRENDER reaktif di halaman yang sama (fsec != nil, peran
// kustom, boleh disunting); fsec nil (tak berwenang crm:field_security) atau
// checkbox-nya statis → tak ada signal fls_view/fls_edit untuk dirujuk.
func flsReactive(fsec *FieldSecurityRoleView) bool {
	return fsec != nil && fsec.Reactive && fsec.CanEdit
}

// roleMatrix/roleMatrixRow (tabel izin per modul) & arrCrossModuleCtx
// (konteks reaktivitas checkbox ARR lintas modul) di role_edit_matrix.go —
// dipisah krn ambang File Health (View/Component 300 baris).

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
