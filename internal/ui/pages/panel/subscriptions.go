package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions.go — view daftar Subscription Lists (Modul 5, M5-3b GET-only).
// Murni-data: MRR/ARR SUDAH diformat & disamarkan F4 di handler (ARR bisa berupa
// penanda tersembunyi flsHidden). Meniru sales_deals.go (tabel + pager keyset).
// Filter status = LINK <a> (navigasi bookmarkable, lolos gotcha #16), bukan
// Datastar. Aksi (renew/churn) belum ada di slice ini.

// SubStatusAll = penanda "Semua" eksplisit di ?status= (BL-20). Handler
// menerjemahkannya ke "tak menyaring status", DIBEDAKAN dari param status yang
// absen (landing murni → default Active). Sengaja bukan "" agar tab "Semua"
// punya URL kanonik & tetap terjangkau. Bukan status legal (subStatusChk) → tak
// bertabrakan dengan enum status langganan.
const SubStatusAll = "all"

// SubRow = satu langganan untuk baris tabel RAMPING (BL-95, 6 kolom: Desa · Paket ·
// MRR · Status · Renewal Date · CSM). MRR SUDAH diformat & disamarkan F4 di handler.
// Status = label DERIVASI (Aman/Jatuh Tempo/Masa Tenggang untuk Active; lifecycle
// apa adanya untuk non-Active), StatusClass = class badge daisyUI dari handler.
// Renewal = end_date terformat, CSM = nama pemilik. ARR/Mulai/EntityCode dibuang.
type SubRow struct {
	ID          int64
	Village     string
	Plan        string
	Status      string
	StatusClass string
	MRR         string
	Renewal     string
	CSM         string
}

// SubKPIs = 4 kartu KPI header halaman (BL-95). SUDAH diformat di handler (Rp,
// angka, %). TotalMRR + NewMRR (delta "MRR baru bln ini"), ARR, ActiveSubs +
// Accounts (denominator "dari N desa"), ChurnRate (churn 30 hari).
type SubKPIs struct {
	TotalMRR   string
	NewMRR     string
	ARR        string
	ActiveSubs string
	Accounts   string
	ChurnRate  string
}

// SubListView = data halaman /subscriptions. StatusFilter = penanda pilihan tab
// yang diterima dari handler (status legal "Active"/… ATAU SubStatusAll="all"),
// dipakai menandai tab aktif & menyusun tautan/pager — BUKAN nilai saring DB
// (handler menerjemahkan "all" → tak menyaring). Statuses = opsi filter dioper
// handler (view tak memutuskan enum). Keyset lewat NextCursor.
type SubListView struct {
	Base         string
	StatusFilter string
	Statuses     []string
	Query        string // ?q= pencarian bebas (BL-6); "" = tak mencari
	KPIs         SubKPIs
	Err          string
	Items        []SubRow
	NextCursor   string
	After        string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail        string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// SubList merender halaman daftar: header + filter status + alert + tabel + pager.
func SubList(v SubListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Subscription Lists")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Langganan berjalan — paket, nilai berulang, dan masa berlaku.")),
			),
		),
		subKPICards(v.KPIs),
		tabSearchRow(subStatusFilter(v),
			searchBoxInline(v.Base+"/subscriptions", v.Query,
				"Cari langganan — desa, paket, atau kode…", "Cari langganan",
				hiddenField{"status", v.StatusFilter})),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "subs-err", g.Text(v.Err)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptySubs(v))
	} else {
		body = append(body, subsTable(v), subsPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// subKPICards = 4 kartu KPI header (BL-95). Grid mobile-first grid-cols-1 → md:2 →
// lg:4 (pola BL-94). Total MRR bersubtitel delta "+… bln ini" (MRR baru bln ini);
// Langganan Aktif bersubtitel "dari N desa" (total akun desa) — keputusan user.
func subKPICards(k SubKPIs) g.Node {
	return h.Div(
		h.Class("grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 min-w-0"),
		subKPICard("Total MRR", k.TotalMRR, "+"+k.NewMRR+" bln ini", "text-primary"),
		subKPICard("ARR", k.ARR, "proyeksi tahunan", "text-secondary"),
		subKPICard("Langganan Aktif", k.ActiveSubs, "dari "+k.Accounts+" desa", "text-success"),
		subKPICard("Churn Rate", k.ChurnRate, "30 hari terakhir", "text-error"),
	)
}

func subKPICard(label, value, sub, colorCls string) g.Node {
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

// subStatusFilter = baris tab LINK filter status (Semua + tiap status). Navigasi
// bookmarkable (gotcha #16); flex-wrap agar tak mendorong lebar di mobile.
func subStatusFilter(v SubListView) g.Node {
	tab := func(label, status string) g.Node {
		// q dibawa lintas tab (mencari lalu ganti status tak menghapus pencarian).
		href := withQuery(v.Base+"/subscriptions", v.Query, hiddenField{"status", status})
		cls := "tab"
		if v.StatusFilter == status {
			cls += " tab-active font-medium"
		}
		return h.A(h.Href(href), h.Class(cls+" min-h-11"), g.Text(label))
	}
	tabs := make([]g.Node, 0, len(v.Statuses)+1)
	// "Semua" mengirim penanda eksplisit SubStatusAll (bukan ""), agar tab tetap
	// terjangkau ketika landing murni default ke Active (BL-20).
	tabs = append(tabs, tab("Semua", SubStatusAll))
	for _, s := range v.Statuses {
		tabs = append(tabs, tab(s, s))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func emptySubs(v SubListView) g.Node {
	// "all" = tab Semua (tak menyaring status) → diperlakukan seperti tanpa filter.
	statusScoped := v.StatusFilter != "" && v.StatusFilter != SubStatusAll
	msg := "Belum ada langganan."
	switch {
	case v.Query != "":
		msg = "Belum ada langganan yang cocok pencarian."
	case v.NextCursor == "" && statusScoped:
		msg = "Belum ada langganan dengan status ini."
	}
	// Reset mempertahankan status aktif tapi membuang q (kembali ke awal filter).
	reset := withQuery(v.Base+"/subscriptions", "", hiddenField{"status", v.StatusFilter})
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"), g.Text(msg)),
			ui.When(statusScoped || v.Query != "", h.A(
				h.Href(reset), h.Class("btn btn-ghost btn-sm min-h-11"),
				g.Text("« Semua langganan"))),
		),
	)
}

// subsTable = tabel langganan, dibungkus ui.TableScroll (scroll terkurung, tak
// meluberkan viewport 375px).
func subsTable(v SubListView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, s := range v.Items {
		rows = append(rows, subTableRow(v.Base, s))
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("MRR")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Masa Berlaku")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Renewal Date")),
					h.Th(h.Class("py-2 font-medium"), g.Text("CSM")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func subTableRow(base string, s SubRow) g.Node {
	href := base + "/subscriptions/" + strconv.FormatInt(s.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	// StatusClass di-derivasi handler (Aman/Jatuh Tempo/Masa Tenggang utk Active,
	// lifecycle utk non-Active); view hanya merender.
	badgeCls := s.StatusClass
	if badgeCls == "" {
		badgeCls = "badge badge-ghost"
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(orDash(s.Village)))),
		link(orDash(s.Plan), "py-2 pr-4"),
		link(orDash(s.MRR), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href),
			h.Span(h.Class(badgeCls), g.Text(orDash(s.Status))))),
		link(orDash(s.Renewal), "py-2 pr-4"),
		link(orDash(s.CSM), "py-2"),
	)
}

// subStatusBadge = badge status langganan berwarna semantik daisyUI (token, bukan
// absolut). Active hijau, risiko/akhir (Expired/Cancelled/Churned) merah/peringatan.
// Dipakai halaman detail & rollup akun (daftar BL-95 kini pakai StatusClass derivasi).
func subStatusBadge(status string) g.Node {
	cls := "badge badge-ghost"
	switch status {
	case "Active":
		cls = "badge badge-success"
	case "Trial":
		cls = "badge badge-info"
	case "Suspended", "PendingApproval":
		cls = "badge badge-warning"
	case "Expired", "Cancelled", "Churned":
		cls = "badge badge-error"
	}
	return h.Span(h.Class(cls), g.Text(orDash(status)))
}

func subsPager(v SubListView) g.Node {
	base := panelListHref(v.Base+"/subscriptions", [2]string{"status", v.StatusFilter}, [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
