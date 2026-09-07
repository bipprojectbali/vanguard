package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_churn.go — view dasbor Churn (Modul 5, Menu 5.2/5.4, READ-ONLY).
// Murni-data: MRR Hilang/Tgl Churn/Tenure/CSM & nilai KPI SUDAH diformat di handler.
// Tab tipe churn = LINK <a> (navigasi bookmarkable, lolos gotcha #16), bukan Datastar.
// TANPA aksi (dasbor baca) — churn ditandai dari detail langganan. BL-92: banner
// peringatan (token warning), 4 kartu KPI, kolom Tenure, tombol Ekspor CSV.

// ChurnKPIs = 4 KPI dasbor Churn, SUDAH diformat di handler. VillagesActive dipakai
// sub-label kartu Desa Churn ("dari M aktif"), bukan kartu tersendiri.
type ChurnKPIs struct {
	ChurnRate       string // Churn Rate 30 hari, "%"
	ChurnedMRR      string // Rp, hilang bulan berjalan (ter-mask F4)
	VillagesChurned string // N desa churn 30 hari
	VillagesActive  string // M desa aktif (snapshot) — sub "dari M aktif"
	AvgTenure       string // rata masa langganan sebelum berhenti, "N bln"
}

// ChurnRow = satu langganan yang berhenti untuk baris tabel Churn. Semua nilai
// SUDAH diformat di handler (LostMRR, ChurnDate, Tenure = string; CSM = nama anggota).
type ChurnRow struct {
	ID        int64
	Village   string
	Plan      string
	LostMRR   string
	Reason    string
	Type      string
	ChurnDate string
	Tenure    string
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
	KPIs       ChurnKPIs
	Err        string
	Items      []ChurnRow
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// ChurnList merender halaman: header + ekspor + banner + KPI + tab tipe + tabel + pager.
func ChurnList(v ChurnView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Churn / Cancellations")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Langganan Desa+ berhenti · alasan · tren · winback")),
			),
			churnExportBtn(v),
		),
		churnBanner(),
		churnKPICards(v.KPIs),
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

// churnBanner = banner info bertoken warning (peringatan, BUKAN error/absolut):
// churn = kehilangan, analisis di sini, winback ditangani di Customer Success.
func churnBanner() g.Node {
	return ui.Alert(ui.VariantWarning, "churn-info",
		g.Text("Churn = kehilangan. Desa+ berhenti berlangganan. Analisis di sini · winback ditangani di Customer Success."))
}

// churnExportBtn = tombol ekspor CSV read-only (route baru /subscriptions/churn/
// export). LINK <a> (navigasi, lolos gotcha #16); tipe aktif ikut menyaring ekspor.
func churnExportBtn(v ChurnView) g.Node {
	return h.A(
		h.Href(v.Base+"/subscriptions/churn/export?type="+v.Type),
		g.Attr("download", ""),
		h.Class("btn btn-sm btn-outline min-h-11"),
		g.Text("Ekspor CSV"),
	)
}

// churnKPICards = 4 kartu KPI. Grid mobile-first grid-cols-1 → md:2 → lg:4 (BL-92).
func churnKPICards(k ChurnKPIs) g.Node {
	return h.Div(
		h.Class("grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 min-w-0"),
		churnKPICard("Churn Rate", k.ChurnRate, "30 hari terakhir", "text-error"),
		churnKPICard("Churned MRR", k.ChurnedMRR, "hilang bulan ini", "text-error"),
		churnKPICard("Desa Churn", k.VillagesChurned, "dari "+k.VillagesActive+" aktif", ""),
		churnKPICard("Avg Tenure", k.AvgTenure, "sebelum berhenti", ""),
	)
}

func churnKPICard(label, value, sub, colorCls string) g.Node {
	numCls := "text-2xl font-bold"
	if colorCls != "" {
		numCls += " " + colorCls
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body p-3"),
			h.P(h.Class("text-xs text-base-content/60 truncate"), g.Text(label)),
			h.P(h.Class(numCls), g.Text(value)),
			h.P(h.Class("text-xs text-base-content/50 truncate"), g.Text(sub)),
		),
	)
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
// meluberkan viewport 375px). Kolom mengikuti wireframe 5.2/5.4 + Tenure (BL-92).
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tenure")),
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
		link(orDash(s.Tenure), "py-2 pr-4"),
		link(orDash(s.CSM), "py-2"),
	)
}

func churnPager(v ChurnView) g.Node {
	base := v.Base + "/subscriptions/churn?type=" + v.Type
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
