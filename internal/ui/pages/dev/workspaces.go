package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// workspaces.go — panel PLATFORM atas workspace (keputusan 0005). Menampilkan
// SEMUA workspace lintas-tenant, termasuk yang ditangguhkan, diarsipkan, dan
// terhapus: justru itu yang perlu dilihat operator, dan restore hanya mungkin
// bila barisnya tampak.

// WorkspaceRow = satu workspace untuk tabel platform (view model, bukan db.Tenant).
type WorkspaceRow struct {
	ID      int64
	Name    string
	Slug    string
	Status  string // active | suspended | archived
	Members int64
	Deleted bool   // dalam masa tenggang — bisa dipulihkan
	Reason  string // alasan penangguhan (kosong bila tak ditangguhkan)
}

// Workspaces merender tabel workspace + aksi platform (tangguhkan, aktifkan,
// pulihkan). page/total/size untuk paginasi — daftar lintas-platform bisa
// ribuan baris, jadi SELALU dipaginasi (Rule 13).
func Workspaces(rows []WorkspaceRow, page int, total int64, size int, errMsg string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text("Workspaces")),
	}
	if errMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "ws-err", g.Text(errMsg)))
	}
	body = append(body,
		h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(
				h.Class("card-body min-w-0"),
				ui.TableScroll(h.Table(
					h.Class("w-full text-sm"),
					h.THead(h.Tr(
						h.Class("border-b border-base-300 text-left text-base-content/70"),
						th("Workspace"), th("Status"), th("Anggota"), th("Aksi"),
					)),
					h.TBody(g.Map(rows, workspaceRow)),
				)),
			),
		),
		workspacePager(page, total, size),
	)
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

func workspaceRow(t WorkspaceRow) g.Node {
	id := strconv.FormatInt(t.ID, 10)
	return h.Tr(
		h.Class("border-b border-base-300/50"),
		h.Td(h.Class("py-2 pr-4"), h.Div(
			h.Class("flex flex-col min-w-0"),
			// Nama = tautan ke halaman detail (ganti nama + zona bahaya, BL-53).
			h.A(h.Href("/dev/workspaces/"+id),
				h.Class("truncate font-medium link link-hover"), g.Text(t.Name)),
			h.Span(h.Class("text-xs text-base-content/60 truncate"), g.Text("/w/"+t.Slug)),
		)),
		h.Td(h.Class("py-2 pr-4"), workspaceStatus(t)),
		h.Td(h.Class("py-2 pr-4"), g.Text(strconv.FormatInt(t.Members, 10))),
		h.Td(h.Class("py-2"), workspaceActions(id, t)),
	)
}

// workspaceStatus = lencana keadaan + alasan penangguhan. Alasan ditampilkan di
// sini (bukan disembunyikan di tooltip) karena operator berikutnya perlu tahu
// KENAPA tanpa menggali audit log.
func workspaceStatus(t WorkspaceRow) g.Node {
	if t.Deleted {
		return h.Div(
			h.Class("flex flex-col gap-1 min-w-0"),
			badge("terhapus", "error"),
			h.Span(h.Class("text-xs text-base-content/60"), g.Text("dalam masa tenggang")),
		)
	}
	switch t.Status {
	case "suspended":
		nodes := []g.Node{h.Class("flex flex-col gap-1 min-w-0"), badge("ditangguhkan", "warning")}
		if t.Reason != "" {
			nodes = append(nodes, h.Span(
				h.Class("text-xs text-base-content/60 break-words"), g.Text(t.Reason)))
		}
		return h.Div(nodes...)
	case "archived":
		return badge("diarsipkan", "neutral")
	default:
		return badge("aktif", "success")
	}
}

// workspaceActions = tombol per keadaan. Tiap aksi = form POST native → 303
// (gotcha #16). flex-wrap wajib: baris tombol tanpa wrap mendorong halaman di 375px.
func workspaceActions(id string, t WorkspaceRow) g.Node {
	base := "/dev/workspaces/" + id
	var actions []g.Node
	switch {
	case t.Deleted:
		actions = append(actions, actionForm(base+"/restore", "Pulihkan", "btn-primary"))
	case t.Status == "suspended":
		actions = append(actions, actionForm(base+"/unsuspend", "Aktifkan", "btn-primary"))
	default:
		// Tangguhkan butuh ALASAN — input sebaris, bukan modal: operator sedang
		// menatap daftar, dan memaksa pindah halaman membuat alasan diisi asal.
		actions = append(actions, h.FormEl(
			h.Method("post"), h.Action(base+"/suspend"),
			h.Class("flex flex-wrap items-center gap-2 min-w-0"),
			ui.Input(h.Type("text"), h.Name("reason"), h.Required(),
				h.Placeholder("alasan penangguhan"),
				h.Class("input input-sm w-full min-w-0 sm:w-48")),
			h.Button(h.Type("submit"), h.Class("btn btn-sm btn-warning btn-outline"),
				g.Text("Tangguhkan")),
		))
	}
	return h.Div(h.Class("flex flex-wrap items-center gap-2 min-w-0"), g.Group(actions))
}

func actionForm(action, label, class string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(action),
		h.Button(h.Type("submit"), h.Class("btn btn-sm "+class), g.Text(label)),
	)
}

// workspacePager = navigasi halaman. flex-wrap di baris tombol (bukan hanya di
// wrapper luar): tanpa itu prev/next mendorong halaman di 375px.
func workspacePager(page int, total int64, size int) g.Node {
	last := int((total + int64(size) - 1) / int64(size))
	if last < 1 {
		last = 1
	}
	var links []g.Node
	if page > 1 {
		links = append(links, pagerLink(page-1, "Sebelumnya"))
	}
	links = append(links, h.Span(
		h.Class("text-sm text-base-content/60 px-2"),
		g.Text("Halaman "+strconv.Itoa(page)+" dari "+strconv.Itoa(last)),
	))
	if page < last {
		links = append(links, pagerLink(page+1, "Berikutnya"))
	}
	return h.Div(h.Class("flex flex-wrap items-center gap-2"), g.Group(links))
}

func pagerLink(page int, label string) g.Node {
	return h.A(
		h.Href("/dev/workspaces?page="+strconv.Itoa(page)),
		h.Class("btn btn-sm btn-outline"),
		g.Text(label),
	)
}
