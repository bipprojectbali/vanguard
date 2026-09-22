package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// role_edit_levels.go — opsi & builder dropdown tingkat izin per modul
// (levelOptions dkk), dipisah dari role_edit.go (sudah di ambang batas 500
// baris/tipe View) agar varian write-enforced-aware tak mendorongnya lewat
// batas. Modul TANPA titik enforcement "write" nyata di backend
// (RoleModulePerm.WriteEnforced == false — audit 2026-09: dashboard,
// subscriptions, activities, 4 halaman Reports) HANYA menawarkan
// "Tak ada"/"Lihat": opsi "Kelola" akan menjanjikan kemampuan yang tak
// pernah dicek CanBusiness(ctx,obj,"write") di mana pun, sehingga
// menyesatkan admin yang mengira mencentangnya membuka aksi tulis.
// business.conf: write MENCAKUP read, jadi baris "write" LAMA di DB (mis.
// grant default Manager crm:subscriptions write) tetap berfungsi sbg read
// tanpa perubahan — handler (roleModuleRows, roles_card.go) menurunkan Level
// yang dioper ke sini sudah direndahkan ke "read" utk modul ini, jadi select
// selalu punya <option> yang cocok (tak pernah default diam-diam ke "Tak ada").

// levelOptions = pilihan tingkat izin utk modul ber-WriteEnforced. Value =
// string mesin yang dibaca readRoleMatrix di handler ("read"/"write"; selain
// itu → dilewati = none).
var levelOptions = []ScopeOption{
	{Value: "none", Label: "Tak ada"},
	{Value: "read", Label: "Lihat"},
	{Value: "write", Label: "Kelola"},
}

// levelOptionsReadOnly = varian TANPA "Kelola" utk modul yang backend hanya
// pernah cek "read" (RoleModulePerm.WriteEnforced == false).
var levelOptionsReadOnly = []ScopeOption{
	{Value: "none", Label: "Tak ada"},
	{Value: "read", Label: "Lihat"},
}

// levelSelect = dropdown tingkat izin satu modul (name="level.<obj>"), versi
// STATIS (tanpa Datastar) — dipakai saat !canEdit atau baris tak reaktif.
func levelSelect(obj, current string, disabled, writeEnforced bool) g.Node {
	attrs := []g.Node{h.Class("select select-sm"), h.Name("level." + obj)}
	if disabled {
		attrs = append(attrs, h.Disabled())
	}
	return h.Select(append(attrs, g.Group(levelOpts(current, writeEnforced)))...)
}

// levelOpts = daftar <option> levelOptions (atau levelOptionsReadOnly bila
// !writeEnforced), current terpilih. Dipakai levelSelect (statis) &
// roleMatrixRow (varian reaktif, role_edit.go) agar daftar opsi tak dua kali
// diketik.
func levelOpts(current string, writeEnforced bool) []g.Node {
	src := levelOptions
	if !writeEnforced {
		src = levelOptionsReadOnly
	}
	opts := make([]g.Node, 0, len(src))
	for _, o := range src {
		opts = append(opts, optionSel(o.Value, o.Label, current))
	}
	return opts
}
