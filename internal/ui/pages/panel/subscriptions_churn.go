package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_churn.go — view dasbor Churn (Modul 5, Menu 5.2/5.4, READ-ONLY).
// Murni-data: MRR Hilang/Tgl Churn/CSM SUDAH diformat di handler. Tab tipe churn =
// LINK <a> (navigasi bookmarkable, lolos gotcha #16), bukan Datastar. TANPA aksi
// (dasbor baca) — churn ditandai dari detail langganan. Meniru subscriptions_renewals.go.

// ChurnRow = satu langganan yang berhenti untuk baris tabel Churn. Semua nilai
// SUDAH diformat di handler (LostMRR, ChurnDate = string; CSM = nama anggota).
type ChurnRow struct {
	ID        int64
	Village   string
	Plan      string
	LostMRR   string
	Reason    string
	Type      string
	ChurnDate string
	CSM       string
}

// ChurnType = satu tab tipe churn (key untuk href + label tampilan). Dioper handler;
// view tak memutuskan enum tipe.
type ChurnType struct{ Key, Label string }

// ChurnView = data halaman /subscriptions/churn. Type = tipe aktif terpilih (” =
// semua); Types = opsi tab dioper handler. Keyset lewat NextCursor.
type ChurnView struct {
	Base       string
	Type       string
	Types      []ChurnType
	Err        string
	Items      []ChurnRow
	NextCursor string
}

// ChurnList merender halaman: header + tab tipe + alert + tabel + pager.
func ChurnList(v ChurnView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Churn / Cancellations")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Langganan yang telah berhenti & nilai MRR yang hilang. Aksi churn dikerjakan di detail langganan.")),
			),
		),
		churnTypeTabsView(v),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "churn-err", g.Text(v.Err)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyChurn())
	} else {
		body = append(body, churnTable(v), churnPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// churnTypeTabsView = baris tab LINK tipe churn. Navigasi bookmarkable (gotcha #16);
// flex-wrap agar tak mendorong lebar di mobile.
func churnTypeTabsView(v ChurnView) g.Node {
	tabs := make([]g.Node, 0, len(v.Types))
	for _, t := range v.Types {
		cls := "tab min-h-11"
		if v.Type == t.Key {
			cls += " tab-active font-medium"
		}
		tabs = append(tabs, h.A(
			h.Href(v.Base+"/subscriptions/churn?type="+t.Key),
			h.Class(cls), g.Text(t.Label)))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func emptyChurn() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"), g.Text("Tak ada langganan berhenti pada filter ini.")),
		),
	)
}

// churnTable = tabel churn, dibungkus ui.TableScroll (scroll terkurung, tak
// meluberkan viewport 375px). Kolom mengikuti wireframe 5.2/5.4.
func churnTable(v ChurnView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, s := range v.Items {
		rows = append(rows, churnTableRow(v.Base, s))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Paket")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("MRR Hilang")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Alasan")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tipe")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tgl Churn")),
					h.Th(h.Class("py-2 font-medium"), g.Text("CS")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func churnTableRow(base string, s ChurnRow) g.Node {
	href := base + "/subscriptions/" + strconv.FormatInt(s.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(orDash(s.Village)))),
		link(orDash(s.Plan), "py-2 pr-4"),
		link(orDash(s.LostMRR), "py-2 pr-4"),
		link(orDash(s.Reason), "py-2 pr-4"),
		link(orDash(s.Type), "py-2 pr-4"),
		link(orDash(s.ChurnDate), "py-2 pr-4"),
		link(orDash(s.CSM), "py-2"),
	)
}

func churnPager(v ChurnView) g.Node {
	if v.NextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	href := v.Base + "/subscriptions/churn?type=" + v.Type + "&after=" + v.NextCursor
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(href), h.Class("btn min-h-11"), g.Text("Berikutnya »")),
	)
}
