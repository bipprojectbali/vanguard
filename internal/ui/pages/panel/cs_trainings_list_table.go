package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_trainings_list_table.go — tabel daftar training (Modul 6, sub-item
// Onboarding 6.2.1.2) untuk cs_trainings_list.go. Aksi ganti status per baris
// (csTrainingStatusForm) di cs_trainings_actions.go.

// csTrainingsTable = tabel daftar training, dibungkus ui.TableScroll. Kolom
// Aksi hanya muncul bila canWrite=true.
func csTrainingsTable(v CSTrainingsListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Topik")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jadwal")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Trainer")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peserta")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Attendance")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Catatan")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, row := range v.Items {
		rows = append(rows, csTrainingTableRow(v.Base, row, v.CanWrite))
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

func csTrainingTableRow(base string, r CSTrainingRow, canWrite bool) g.Node {
	id := strconv.FormatInt(r.ID, 10)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 max-w-[120px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.AccountName)))),
		h.Td(h.Class("py-2 pr-4 max-w-[200px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.TrainingTopic)))),
		h.Td(h.Class("py-2 pr-4 text-sm text-base-content/70"), g.Text(r.TrainingDate)),
		h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-sm "+r.StatusBadge), g.Text(r.StatusLabel))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(r.TrainerName))),
		h.Td(h.Class("py-2 pr-4"), g.Text(r.Participants)),
		h.Td(h.Class("py-2 pr-4"), g.Text(r.Attendance)),
		h.Td(h.Class("py-2 pr-4 max-w-[160px]"),
			h.Span(h.Class("block truncate text-base-content/70"), g.Text(orDash(r.Notes)))),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), csTrainingStatusForm(base, id, r.StatusLabel, r.Notes)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}
