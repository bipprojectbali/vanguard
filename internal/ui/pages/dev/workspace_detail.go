package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// workspace_detail.go — halaman aksi PLATFORM atas SATU workspace (BL-53):
// ganti nama + zona bahaya (arsip/pulihkan/hapus). Identitas workspace dulu
// dikelola owner dari /w/{slug}/settings; kini pindah ke ruang developer dan
// jadi wewenang platform saja. Semua aksi = form POST native → 303 (gotcha #16:
// redirect via SSE menyuntik <script> yang diblokir CSP), bukan Datastar.

// WorkspaceDetailView = data siap-render halaman detail workspace platform.
type WorkspaceDetailView struct {
	ID      int64
	Name    string
	Slug    string
	Status  string // active | suspended | archived
	Primary bool   // rumah aplikasi — tak bisa diarsipkan/dihapus
	Deleted bool   // dalam masa tenggang — bisa dipulihkan
	ErrMsg  string // pesan dari redirect PRG (?err=CODE)
}

// WorkspaceDetail merender identitas + aksi satu workspace untuk operator
// platform. Ganti nama diizinkan selama workspace belum terhapus (masa tenggang
// = pulihkan dulu). Zona bahaya menyesuaikan keadaan; rumah aplikasi hanya
// menampilkan catatan (arsip/hapus ditolak di handler DAN SQL).
func WorkspaceDetail(v WorkspaceDetailView) g.Node {
	id := strconv.FormatInt(v.ID, 10)
	base := "/dev/workspaces/" + id

	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text("Workspace: "+v.Name)),
			h.A(h.Href("/dev/workspaces"), h.Class("btn btn-sm btn-ghost"), g.Text("← Semua workspace")),
		),
		h.Div(h.Class("flex flex-wrap items-center gap-2"),
			detailStatusBadge(v),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("/w/"+v.Slug)),
		),
	}
	if v.ErrMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "ws-detail-err", g.Text(v.ErrMsg)))
	}
	body = append(body, renameCard(base, v), detailDangerZone(base, v))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// detailStatusBadge = lencana keadaan (sejalan dengan daftar workspaces.go).
func detailStatusBadge(v WorkspaceDetailView) g.Node {
	switch {
	case v.Deleted:
		return badge("terhapus", "error")
	case v.Status == "suspended":
		return badge("ditangguhkan", "warning")
	case v.Status == "archived":
		return badge("diarsipkan", "neutral")
	default:
		return badge("aktif", "success")
	}
}

// renameCard = form ganti nama tampilan. Slug read-only (identitas immutable —
// mengubahnya mematikan setiap tautan tersimpan). Workspace terhapus tak bisa
// diganti namanya: pulihkan dulu.
func renameCard(base string, v WorkspaceDetailView) g.Node {
	fields := []g.Node{
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Slug (identitas, tak bisa diubah)", h.For("slug")),
			ui.Input(h.ID("slug"), h.Type("text"), h.Value(v.Slug), h.Disabled()),
		),
	}
	if v.Deleted {
		fields = append(fields, h.P(
			h.Class("text-sm text-base-content/60"),
			g.Text("Workspace dalam masa tenggang penghapusan — pulihkan dulu untuk mengubah nama."),
		))
	} else {
		fields = append(fields,
			h.FormEl(
				h.Method("post"), h.Action(base+"/rename"),
				h.Class("grid gap-2"),
				ui.Label("Nama Workspace", h.For("name")),
				ui.Input(h.ID("name"), h.Type("text"), h.Name("name"),
					h.Value(v.Name), h.Placeholder("mis. Acme Corp"),
					h.MaxLength("60"), h.Required()),
				h.Button(h.Type("submit"), h.Class("btn btn-sm btn-primary w-fit"),
					g.Text("Simpan nama")),
			),
		)
	}
	return ui.Card(fields...)
}

// detailDangerZone = arsip/pulihkan/hapus, menyesuaikan keadaan. Rumah aplikasi
// (is_primary) tak menampilkan tombol apa pun — hanya catatan; penolakannya
// dijaga di handler DAN SQL, dan menawarkan tombol yang pasti gagal adalah cacat
// tersendiri. Semua aksi = form POST native → 303.
func detailDangerZone(base string, v WorkspaceDetailView) g.Node {
	var actions []g.Node
	switch {
	case v.Primary:
		actions = append(actions, h.P(
			h.Class("text-sm text-base-content/70 break-words"),
			g.Text("Workspace ini adalah rumah aplikasi — tak bisa diarsipkan maupun dihapus."),
		))
	case v.Deleted:
		actions = append(actions, detailAction(base+"/restore", "Pulihkan",
			"Batalkan penghapusan selama masih dalam masa tenggang.", "btn-primary"))
	case v.Status == "archived":
		actions = append(actions,
			detailAction(base+"/unarchive", "Aktifkan kembali",
				"Workspace kembali bisa diubah oleh anggotanya.", "btn-primary"),
			detailAction(base+"/delete", "Hapus workspace",
				"Bisa dibatalkan dalam 30 hari. Setelah itu data dihapus permanen.", "btn-error btn-outline"),
		)
	default:
		actions = append(actions,
			detailAction(base+"/archive", "Arsipkan",
				"Workspace jadi hanya-baca. Data tetap utuh dan bisa diaktifkan lagi kapan saja.", "btn-warning btn-outline"),
			detailAction(base+"/delete", "Hapus workspace",
				"Bisa dibatalkan dalam 30 hari. Setelah itu data dihapus permanen.", "btn-error btn-outline"),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-error/40 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold text-error mb-1"), g.Text("Zona Bahaya")),
			h.P(h.Class("text-sm text-base-content/70 mb-3"),
				g.Text("Tindakan di bawah memengaruhi seluruh anggota workspace.")),
			h.Div(h.Class("grid gap-3"), g.Group(actions)),
		),
	)
}

// detailAction = satu baris aksi: penjelasan + tombol form POST native. Baris
// pakai flex-wrap agar tak mendorong halaman di 375px.
func detailAction(action, label, desc, btnClass string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(action),
		h.Class("flex flex-wrap items-center justify-between gap-2 min-w-0"),
		h.P(h.Class("text-sm text-base-content/70 flex-1 min-w-0 break-words"), g.Text(desc)),
		h.Button(h.Type("submit"), h.Class("btn btn-sm "+btnClass), g.Text(label)),
	)
}
