package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_deals.go — view hub Deal: pipeline (KPI + Kanban) & tampilan Tabel.
// Murni-data: Amount & PipelineValue SUDAH diformat/disamarkan F4 di handler.
// Meniru sales_leads.go/accounts.go. Ganti stage = kontrol aksi di detail (native
// POST), BUKAN drag-drop (keputusan terkunci #1) — papan ini read-only.

// DealRow = satu deal untuk kartu Kanban / baris Tabel. Amount SUDAH diformat &
// disamarkan F4 di handler. Owner = nama orang.
type DealRow struct {
	ID            int64
	EntityCode    string
	DealName      string
	Stage         string
	Amount        string
	Probability   string
	ExpectedClose string
	Owner         string
}

// DealStageColumn = satu kolom Kanban (satu stage + kartu di dalamnya, sudah
// terurut handler).
type DealStageColumn struct {
	Stage string
	Cards []DealRow
}

// DealPipelineView = data halaman Deal. View "" = pipeline (Kanban+KPI), "table"
// = Tabel berkeyset. KPI (OpenCount/PipelineValue/WinRate) hanya dipakai view
// pipeline; Items/NextCursor hanya view Tabel.
type DealPipelineView struct {
	Base     string
	View     string
	CanWrite bool
	Err      string
	Msg      string

	// Mine (BL-10) = toggle "Deal Saya" aktif (?mine=1) → papan/tabel/KPI menyempit
	// ke deal milik sendiri. ShowMineToggle = render toggle-nya; HANYA saat cakupan
	// aktor 'all' (bagi 'own' redundan — sudah otomatis milik sendiri). Keduanya
	// diputuskan HANDLER (dari filter.ScopeAll), bukan view.
	Mine           bool
	ShowMineToggle bool

	OpenCount     string
	PipelineValue string
	WinRate       string
	Stages        []DealStageColumn

	StageFilter string
	Query       string // ?q= pencarian bebas (BL-6, hanya view Tabel); "" = tak mencari
	Items       []DealRow
	NextCursor  string
	After       string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail       string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// dealMineParam = "1" bila toggle "Deal Saya" aktif, "" bila tidak — dipakai
// sebagai nilai hiddenField/param URL agar `mine` bertahan lintas view, search,
// dan halaman keyset. "" dilewati oleh withQuery/searchBox (URL kanonik).
func dealMineParam(mine bool) string {
	if mine {
		return "1"
	}
	return ""
}

// DealPipeline merender halaman: header + toggle tampilan + alert + (Kanban+KPI
// | Tabel).
func DealPipeline(v DealPipelineView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Deals")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Pipeline penjualan — pantau tahap, nilai, dan peluang menang.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/deals/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Deal Baru"),
			)),
		),
		dealViewToggle(v),
	}
	// Toggle "Deal Saya" (BL-10): hanya bagi cakupan 'all'. Berlaku untuk KEDUA
	// tampilan (pipeline & tabel). Di view Tabel disejajarkan sebaris dgn kotak
	// cari (BL-68, keputusan user 7 Sep: align dgn Semua/Deal Saya, bukan toggle
	// view) → dirender di dalam tabSearchRow di bawah, JANGAN di sini agar tak
	// dobel. Di view Pipeline (tanpa search) ia berdiri sendiri.
	tableView := v.View == "table"
	if v.ShowMineToggle && !tableView {
		body = append(body, dealMineToggle(v))
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "deals-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "deals-ok", g.Text(v.Msg)))
	}

	if tableView {
		// Search hanya di view Tabel (berkeyset); papan Kanban di luar lingkup slice.
		// mine dipertahankan lintas submit search agar toggle tak tereset.
		// BL-68: bila toggle Semua/Deal Saya tampil (cakupan 'all'), sejajarkan
		// search ke pojok kanan sebaris dgn toggle itu — varian INLINE (lebar
		// terbatas agar input+tombol sebaris); tanpa toggle (cakupan 'own'), search
		// berdiri sendiri full-width.
		if v.ShowMineToggle {
			body = append(body, tabSearchRow(dealMineToggle(v),
				searchBoxInline(v.Base+"/deals", v.Query,
					"Cari deal — nama atau kode…", "Cari deal",
					hiddenField{"view", "table"}, hiddenField{"stage", v.StageFilter},
					hiddenField{"mine", dealMineParam(v.Mine)})))
		} else {
			body = append(body, searchBox(v.Base+"/deals", v.Query,
				"Cari deal — nama atau kode…", "Cari deal",
				hiddenField{"view", "table"}, hiddenField{"stage", v.StageFilter},
				hiddenField{"mine", dealMineParam(v.Mine)}))
		}
		if len(v.Items) == 0 {
			body = append(body, emptyDeals(v))
		} else {
			body = append(body, dealsTable(v), dealsPager(v))
		}
	} else {
		body = append(body, dealKPIs(v), dealKanban(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// pipelineTabLink = satu tautan tab papan Deal (dipakai toggle Pipeline/Tabel &
// toggle Semua/Deal Saya). active menandai tab terpilih (tab-active font-medium);
// href sudah dirakit pemanggil karena tiap toggle menyusun sumbu param berbeda.
func pipelineTabLink(href, label string, active bool) g.Node {
	cls := "tab"
	if active {
		cls += " tab-active font-medium"
	}
	return h.A(h.Href(href), h.Class(cls+" min-h-11"), g.Text(label))
}

// dealViewToggle = dua LINK <a> (Pipeline / Tabel) — navigasi bookmarkable
// (lolos gotcha #16). Tampilan aktif ditandai. mine dibawa lintas view agar
// toggle "Deal Saya" tak tereset saat berpindah Pipeline↔Tabel; stage/q TIDAK
// dibawa (khusus Tabel, tak bermakna di Pipeline).
func dealViewToggle(v DealPipelineView) g.Node {
	mine := dealMineParam(v.Mine)
	tab := func(label, view string) g.Node {
		href := withQuery(v.Base+"/deals", "", hiddenField{"view", view}, hiddenField{"mine", mine})
		return pipelineTabLink(href, label, v.View == view)
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"),
		tab("Pipeline", ""), tab("Tabel", "table"))
}

// dealMineToggle = filter kepemilikan "Semua / Deal Saya" (BL-10) sebagai dua
// LINK <a> (navigasi bookmarkable, lolos gotcha #16). Ortogonal dari toggle view:
// mempertahankan view + stage + q aktif, hanya menukar sumbu ?mine=. Ganti sumbu
// mereset cursor (halaman pertama) — konsisten dgn ganti tab di Leads. Dirender
// HANYA saat ShowMineToggle (cakupan 'all'); gate diputuskan handler.
func dealMineToggle(v DealPipelineView) g.Node {
	link := func(label string, mine bool) g.Node {
		keep := []hiddenField{{"view", v.View}, {"stage", v.StageFilter}}
		if mine {
			keep = append(keep, hiddenField{"mine", "1"})
		}
		href := withQuery(v.Base+"/deals", v.Query, keep...)
		return pipelineTabLink(href, label, v.Mine == mine)
	}
	// tabs-box (daisyUI v5) = gaya pill/kotak — sengaja BEDA dari toggle view di
	// atasnya agar terbaca sebagai filter kepemilikan, bukan sumbu tampilan.
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-box flex-wrap w-fit"),
		link("Semua", false), link("Deal Saya", true))
}

// dealKPIs = tiga kartu ringkas: deal terbuka, nilai pipeline (F4), win rate.
func dealKPIs(v DealPipelineView) g.Node {
	card := func(label, value string) g.Node {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body p-4 min-w-0"),
				h.P(h.Class("text-sm text-base-content/60"), g.Text(label)),
				h.P(h.Class("text-xl font-semibold truncate"), g.Text(orDash(value))),
			),
		)
	}
	return h.Div(
		h.Class("grid gap-3 grid-cols-1 sm:grid-cols-3 min-w-0"),
		card("Deal Terbuka", v.OpenCount),
		card("Nilai Pipeline", v.PipelineValue),
		card("Win Rate", v.WinRate),
	)
}

func emptyDeals(v DealPipelineView) g.Node {
	// "Belum ada deal" (kosong sejati) HANYA saat tanpa penyaring apa pun. mine
	// aktif = tersaring → pakai pesan "tak cocok" + jalan kembali (ada deal, tapi
	// bukan milik aktor).
	if v.NextCursor == "" && v.StageFilter == "" && v.Query == "" && !v.Mine {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body"),
				h.P(h.Class("text-base-content/70"), g.Text("Belum ada deal."))),
		)
	}
	// Kembali ke awal mempertahankan view=table + stage + mine, tapi MEMBUANG q:
	// tanpa tombol Reset, tautan ini satu-satunya jalan keluar dari pencarian
	// tanpa hasil, jadi ia harus mengosongkan kata kunci (bukan mengulanginya).
	back := withQuery(v.Base+"/deals", "",
		hiddenField{"view", "table"}, hiddenField{"stage", v.StageFilter},
		hiddenField{"mine", dealMineParam(v.Mine)})
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"),
				g.Text("Belum ada deal yang cocok pada tampilan ini.")),
			h.A(h.Href(back), h.Class("btn btn-ghost btn-sm min-h-11"),
				g.Text("« Kembali ke awal")),
		),
	)
}

func dealsPager(v DealPipelineView) g.Node {
	mine := ""
	if v.Mine {
		mine = "1"
	}
	base := panelListHref(v.Base+"/deals?view=table", [2]string{"stage", v.StageFilter}, [2]string{"mine", mine}, [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
