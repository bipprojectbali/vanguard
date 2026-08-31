package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// tickets_list_table.go — tabel tiket + badge status/prioritas/SLA + form ubah
// status (ticketsTable, ticketTableRow, ticket*Badge, ticketSLALabel,
// ticketStatusForm, ticketActionBtn), dipisah dari tickets_list.go agar tiap file
// di bawah ambang tipe View/Component (300). Shell + KPI + tab tetap di tickets_list.go.

// ticketsTable = tabel daftar tiket, dibungkus ui.TableScroll (scroll terkurung,
// tak meluberkan viewport 375px). Kolom Aksi hanya muncul bila canWrite=true.
func ticketsTable(v TicketsListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tiket")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Subjek")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Prioritas")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("SLA")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Agen")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, t := range v.Items {
		rows = append(rows, ticketTableRow(v.Base, t, v.CanWrite))
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

func ticketTableRow(base string, t TicketRow, canWrite bool) g.Node {
	id := strconv.FormatInt(t.ID, 10)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 font-mono text-xs"), g.Text(t.Number)),
		h.Td(h.Class("py-2 pr-4 max-w-[120px]"), h.Span(h.Class("block truncate"), g.Text(orDash(t.AccountName)))),
		h.Td(h.Class("py-2 pr-4 max-w-[200px]"), h.Span(h.Class("block truncate"), g.Text(orDash(t.Subject)))),
		h.Td(h.Class("py-2 pr-4"), ticketPriorityBadge(t.Priority)),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap"), ticketSLALabel(t.SLALabel)),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(t.AssignedTo))),
		h.Td(h.Class("py-2 pr-4"), ticketStatusBadge(t.Status)),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), ticketStatusForm(base, id, t.Status)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// ticketPriorityBadge = badge prioritas tiket, token semantik daisyUI.
func ticketPriorityBadge(priority string) g.Node {
	switch priority {
	case "tinggi":
		return h.Span(h.Class("badge badge-error"), g.Text("Tinggi"))
	case "sedang":
		return h.Span(h.Class("badge badge-warning"), g.Text("Sedang"))
	default:
		return h.Span(h.Class("badge badge-ghost"), g.Text("Rendah"))
	}
}

// ticketStatusBadge = badge status tiket, token semantik daisyUI.
func ticketStatusBadge(status string) g.Node {
	switch status {
	case "selesai":
		return h.Span(h.Class("badge badge-success"), g.Text("Selesai"))
	case "eskalasi":
		return h.Span(h.Class("badge badge-error"), g.Text("Eskalasi"))
	case "ditugaskan":
		return h.Span(h.Class("badge badge-info"), g.Text("Ditugaskan"))
	default:
		return h.Span(h.Class("badge badge-ghost"), g.Text("Baru"))
	}
}

// ticketSLALabel = teks label SLA dengan warna semantik:
//   - "Terlanggar" → text-error
//   - "n lagi"     → text-warning bila < 4 jam (mengandung "m" saja tanpa "j")
//   - lainnya      → default
func ticketSLALabel(label string) g.Node {
	cls := "text-sm"
	switch label {
	case "Terlanggar":
		cls += " text-error font-medium"
	case "Terpenuhi":
		cls += " text-success"
	case "—":
		cls += " text-base-content/40"
	}
	return h.Span(h.Class(cls), g.Text(label))
}

// ticketStatusForm = form mini ganti status per baris (Support/Manager/Admin).
// Status tersedia = transisi yang masuk akal; bukan semua status dikompres ke
// satu select. Pola: one-form-per-action (tiap aksi tombol, bukan select-POST).
// flex-wrap agar tak dorong lebar tabel di mobile.
func ticketStatusForm(base, id, currentStatus string) g.Node {
	nodes := []g.Node{}
	// Transisi yang ditampilkan berdasarkan status saat ini
	switch currentStatus {
	case "baru":
		nodes = append(nodes,
			ticketActionBtn(base, id, "ditugaskan", "Tugaskan", "btn-info"),
			ticketActionBtn(base, id, "eskalasi", "Eskalasi", "text-warning btn-ghost"),
		)
	case "ditugaskan":
		nodes = append(nodes,
			ticketActionBtn(base, id, "eskalasi", "Eskalasi", "text-warning btn-ghost"),
			ticketActionBtn(base, id, "selesai", "Selesai", "btn-success"),
		)
	case "eskalasi":
		nodes = append(nodes,
			ticketActionBtn(base, id, "ditugaskan", "Tugaskan", "btn-info"),
			ticketActionBtn(base, id, "selesai", "Selesai", "btn-success"),
		)
	default: // selesai — bisa dibuka ulang
		nodes = append(nodes,
			ticketActionBtn(base, id, "baru", "Buka Ulang", "btn-ghost"),
		)
	}
	return h.Div(h.Class("flex flex-wrap items-center gap-1"), g.Group(nodes))
}

func ticketActionBtn(base, id, targetStatus, label, extraCls string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/tickets/"+id+"/status"),
		h.Input(h.Type("hidden"), h.Name("status"), h.Value(targetStatus)),
		h.Button(h.Type("submit"), h.Class("btn btn-xs min-h-11 "+extraCls),
			g.Text(label)),
	)
}
