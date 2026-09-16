package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// permission_sets.go — blok B (Permission Sets) & blok C (Record Ownership
// Rules) dari halaman /roles (BL-145 subtask 6). Keduanya READ-ONLY:
// menyunting matriks Casbin sesungguhnya TETAP di halaman detail (RoleEdit,
// role_edit.go) — blok ini cuma memberi admin GAMBARAN UTUH satu peran tanpa
// masuk ke editor. Murni-data: handler menghitung tanda tiap sel & memilih
// peran yang disorot (?role=), view tak memanggil authz.

// PermMark = tanda satu sel matriks presentasional blok B.
//   - "✓" izin penuh, "~" izin dengan cakupan "own" (hanya desa ditugaskan,
//     diterapkan seragam ke semua sel granted — presentasional, bukan CRUD
//     granular sungguhan), "✗" tak diizinkan.
type PermMark string

const (
	PermMarkFull    PermMark = "✓"
	PermMarkOwnOnly PermMark = "~"
	PermMarkNone    PermMark = "✗"
)

// PermissionSetRow = satu baris matriks presentasional (satu modul CRM) untuk
// peran yang sedang disorot.
type PermissionSetRow struct {
	Label                      string
	View, Create, Edit, Delete PermMark
}

// RoleSwitchOption = satu opsi di pemilih peran (details/summary + link
// native — CSP-safe, tanpa JS). Selected menandai opsi yang sedang disorot.
type RoleSwitchOption struct {
	Name        string
	DisplayName string
	Selected    bool
}

// PermissionSetView = data siap-render blok B (+ dipakai ulang blok C untuk
// peran yang sama). Base dari handler (view tak merakit path sendiri).
// Roles/RoleSwitchOption dipertahankan di sini utk kompatibilitas handler
// (buildPermissionSetView) walau UI switcher sudah dicabut (lihat
// permissionSetsSection) — pemilihan peran tetap lewat query ?role= dari
// tautan "Lihat" per-baris di tabel blok A.
type PermissionSetView struct {
	Base               string
	Roles              []RoleSwitchOption
	SelectedRole       string // nama tampilan peran yang sedang disorot
	SelectedScopeValue string // nilai mesin cakupan data F3 ("all"/"own"/"none")
	SelectedScopeLabel string // label cakupan data F3 (dipakai jg blok C)
	Rows               []PermissionSetRow
}

// permissionSetsSection merender blok B+C sebagai MODAL (bukan lagi section
// inline): id="permission-sets" + class="modal" memanfaatkan dukungan bawaan
// daisyUI utk trigger ":target" (dicek langsung di static/daisyui.js — daftar
// selector pembuka modal termasuk "&:target"), jadi tautan "Lihat" yang sudah
// ada (?role=X#permission-sets) otomatis MEMBUKA modal ini tanpa JS/Datastar
// baru sama sekali (konsisten gotcha #16: navigasi native, bukan SSE). Tutup
// = link ke "#" (mengosongkan fragment, same-document, tanpa reload).
func permissionSetsSection(v PermissionSetView) g.Node {
	return h.Div(
		h.ID("permission-sets"),
		h.Class("modal"),
		g.Attr("role", "dialog"),
		h.Div(
			h.Class("modal-box max-w-2xl w-11/12 min-w-0 grid gap-4"),
			h.Div(
				h.Class("grid gap-2 min-w-0"),
				h.H2(h.Class("text-lg font-semibold"), g.Text("Permission Sets")),
				h.H3(h.Class("font-medium"), g.Text("Peran: "+v.SelectedRole)),
				h.P(h.Class("text-base-content/70"),
					g.Text("Gambaran akses satu peran per modul CRM. Untuk menyunting izin, "+
						"buka halaman detail peran (tombol Edit di tabel peran).")),
				permissionSetsTable(v.Rows),
			),
			RecordOwnershipCard(v.SelectedScopeValue, v.SelectedScopeLabel),
			h.Div(h.Class("modal-action"),
				h.A(h.Href("#"), h.Class("btn min-h-11"), g.Text("Tutup"))),
		),
	)
}

// permissionSetsTable = tabel Lihat/Kelola per modul, dibungkus ui.TableScroll
// (WAJIB tiap <table>, konvensi mobile-first). Kolom Buat/Ubah/Hapus DISATUKAN
// jadi satu kolom "Kelola" — permissionSetRows (handler) selalu mengisi
// Create/Edit/Delete dgn nilai IDENTIK (tak pernah berbeda), jadi menampilkan
// ketiganya terpisah cuma pengulangan visual; r.Create dipakai sbg wakil.
func permissionSetsTable(rows []PermissionSetRow) g.Node {
	trs := make([]g.Node, 0, len(rows))
	for _, r := range rows {
		trs = append(trs, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4"), g.Text(r.Label)),
			h.Td(h.Class("py-2 pr-4 text-center"), permMarkBadge(r.View)),
			h.Td(h.Class("py-2 text-center"), permMarkBadge(r.Create)),
		))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Modul")),
			h.Th(h.Class("py-2 pr-4 font-medium text-center"), g.Text("Lihat")),
			h.Th(h.Class("py-2 font-medium text-center"), g.Text("Kelola (Buat/Ubah/Hapus)")),
		)),
		h.TBody(g.Group(trs)),
	))
}

// permMarkBadge menerjemahkan PermMark → span berwarna (sukses/peringatan/
// redup) agar ✓/~/✗ langsung terbaca tanpa membaca legenda.
func permMarkBadge(m PermMark) g.Node {
	cls := "text-base-content/30"
	switch m {
	case PermMarkFull:
		cls = "text-success font-semibold"
	case PermMarkOwnOnly:
		cls = "text-warning font-semibold"
	}
	return h.Span(h.Class(cls), g.Text(string(m)))
}

// recordOwnershipText = satu kalimat penjelasan per cakupan data F3 (blok C).
// Teks tetap (3 varian) — bukan dibangun dari fragmen, agar tiap kalimat bisa
// dibaca utuh & konsisten dgn istilah businessScopeOptions (roles_card.go).
func recordOwnershipText(scopeValue string) string {
	switch scopeValue {
	case "all":
		return "Peran ini melihat & mengelola data di SEMUA desa workspace, tanpa batasan kepemilikan."
	case "own":
		return "Peran ini hanya melihat & mengelola data pada desa yang DITUGASKAN padanya (ownership, sumbu F3)."
	case "none":
		return "Peran ini tak melihat data lewat kepemilikan desa sama sekali — akses (bila ada) murni dari izin modul di atas."
	default:
		return ""
	}
}

// RecordOwnershipCard merender blok C: kartu ringkas cakupan data peran yang
// sama dengan yang disorot blok B. Tak fetch data baru — scopeValue/scopeLabel
// dioper handler dari data yang sudah dihitung utk blok B (satu sumber).
func RecordOwnershipCard(scopeValue, scopeLabel string) g.Node {
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		h.H2(h.Class("text-lg font-semibold"), g.Text("Record Ownership Rules")),
		h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(
				h.Class("card-body min-w-0 gap-2"),
				h.Div(h.Class("flex items-center gap-2"),
					h.Span(h.Class("text-base-content/70 text-sm"), g.Text("Cakupan data:")),
					h.Span(h.Class("badge badge-outline"), g.Text(scopeLabel)),
				),
				h.P(h.Class("text-sm text-base-content/70"),
					g.Text(recordOwnershipText(scopeValue))),
			),
		),
	)
}
