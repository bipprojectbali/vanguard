package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_impl_tasks_list_table.go — tabel daftar task (Modul 6, sub-item
// Onboarding 6.2.1.1) untuk cs_impl_tasks_list.go.

// csImplTasksTable = tabel daftar task, dibungkus ui.TableScroll. Kolom Aksi
// hanya muncul bila canWrite=true.
func csImplTasksTable(v CSImplTasksListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Task")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Owner")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Due Date")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, row := range v.Items {
		rows = append(rows, csImplTaskTableRow(v.Base, row, v.CanWrite))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					g.Group(head),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func csImplTaskTableRow(base string, r CSImplTaskRow, canWrite bool) g.Node {
	id := strconv.FormatInt(r.ID, 10)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 max-w-[120px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.AccountName)))),
		h.Td(h.Class("py-2 pr-4 max-w-[220px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.TaskName)))),
		h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-sm "+r.StatusBadge), g.Text(r.StatusLabel))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(r.OwnerName))),
		h.Td(h.Class("py-2 pr-4 text-sm text-base-content/70"), g.Text(r.DueDate)),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), csImplTaskStatusForm(base, id, r.StatusLabel)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}
