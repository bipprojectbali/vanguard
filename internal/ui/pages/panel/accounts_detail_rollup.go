package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts_detail_rollup.go — kartu ringkasan lintas-modul di detail Account
// (Langganan, Customer Success, Sistem/Audit) + baris Related Records ("Terkait").
// Murni-data seperti accounts_detail.go: semua string/badge-class sudah
// diputuskan handler (F4 masking utk MRR/ARR via maskARR); view hanya merender.

// SubscriptionSummaryView = ringkasan langganan TERBARU satu desa. MRR/ARR sudah
// disamarkan handler (F4, sama kelas sensitivitas dgn VillageBudget) sebelum
// sampai ke sini. Href non-kosong → judul kartu jadi tautan "lihat semua".
type SubscriptionSummaryView struct {
	PlanName         string
	StatusLabel      string
	MRR              string
	ARR              string
	RenewalOrEndDate string
	Href             string
}

// CustomerSuccessSummaryView = ringkasan kesehatan pelanggan satu desa. Tanpa
// masking (Health Score terbuka utk semua role, konvensi yang sudah ada).
type CustomerSuccessSummaryView struct {
	HealthLabel        string
	HealthBadgeClass   string
	LifecycleStage     string
	LastEngagementDate string
	CSMName            string
	Href               string
}

// AuditView = metadata dibuat/diperbarui. Bukan data sensitif → tanpa masking.
type AuditView struct {
	CreatedByName string
	CreatedAt     string
	UpdatedByName string
	UpdatedAt     string
}

// RelatedRecordChip = satu ringkasan modul lain terkait desa ini. Href kosong →
// chip info non-klik (dipakai Deals/Tickets: belum ada rute daftar ber-filter
// desa utk keduanya).
type RelatedRecordChip struct {
	Label     string
	ValueText string
	Href      string
}

type RelatedRecordsView struct {
	Chips []RelatedRecordChip
}

// detailRow = satu baris label:nilai generik. value sudah berupa g.Node siap
// render (mendukung badge/tautan, bukan cuma teks polos) — dipakai cardRows &
// detailCard.
func detailRow(label string, value g.Node) g.Node {
	return h.Div(
		h.Class("grid gap-1 sm:grid-cols-3 sm:gap-2 py-2 border-b border-base-300/50 last:border-0"),
		h.Dt(h.Class("text-sm text-base-content/60"), g.Text(label)),
		h.Dd(h.Class("sm:col-span-2 break-words"), value),
	)
}

// cardRows = kartu satu kelompok baris siap-render. titleHref non-kosong →
// judul kartu jadi tautan "lihat lebih lanjut" (mis. "Ringkasan Langganan" →
// daftar langganan workspace).
func cardRows(title, titleHref string, rows ...g.Node) g.Node {
	heading := g.Node(h.H2(h.Class("font-semibold mb-2"), g.Text(title)))
	if titleHref != "" {
		heading = h.H2(h.Class("font-semibold mb-2"),
			h.A(h.Href(titleHref), h.Class("link link-hover"), g.Text(title)))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			heading,
			h.Dl(h.Class("min-w-0"), g.Group(rows)),
		),
	)
}

// SubscriptionSummaryCard = kartu "Ringkasan Langganan". Status pakai badge
// subStatusBadge (sama komponen dgn daftar langganan, internal/ui/pages/panel/subscriptions.go)
// agar warna status konsisten lintas halaman.
func SubscriptionSummaryCard(v SubscriptionSummaryView) g.Node {
	return cardRows("Ringkasan Langganan", v.Href,
		detailRow("Paket", g.Text(orDash(v.PlanName))),
		detailRow("Status", subStatusBadge(v.StatusLabel)),
		detailRow("MRR", g.Text(orDash(v.MRR))),
		detailRow("ARR", g.Text(orDash(v.ARR))),
		detailRow("Berakhir/Perpanjangan", g.Text(orDash(v.RenewalOrEndDate))),
	)
}

// CustomerSuccessSummaryCard = kartu "Ringkasan Customer Success". Badge health
// pakai class yang sudah diputuskan handler (healthScoreStatus) — view tak
// menerjemahkan status jadi warna sendiri (satu sumber kebenaran warna).
func CustomerSuccessSummaryCard(v CustomerSuccessSummaryView) g.Node {
	healthBadge := g.Node(g.Text(orDash(v.HealthLabel)))
	if v.HealthLabel != "" {
		healthBadge = h.Span(h.Class("badge "+v.HealthBadgeClass), g.Text(v.HealthLabel))
	}
	return cardRows("Ringkasan Customer Success", v.Href,
		detailRow("Kesehatan", healthBadge),
		detailRow("Tahap Siklus Hidup", g.Text(orDash(v.LifecycleStage))),
		detailRow("Terakhir Engagement", g.Text(orDash(v.LastEngagementDate))),
		detailRow("CS", g.Text(orDash(v.CSMName))),
	)
}

// RelatedRecords = baris "Terkait" selebar penuh di bawah grid 2-kolom. Grid
// 2-kolom mobile → 4-kolom sm+ (mobile-first, senada grid dashboard.go).
func RelatedRecords(v RelatedRecordsView) g.Node {
	chips := make([]g.Node, 0, len(v.Chips))
	for _, c := range v.Chips {
		chips = append(chips, relatedRecordChip(c))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Terkait")),
			h.Div(h.Class("grid grid-cols-2 md:grid-cols-4 gap-3 min-w-0"), g.Group(chips)),
		),
	)
}

// relatedRecordChip = satu chip. Href kosong → h.Div non-klik (Deals/Tickets:
// belum ada rute daftar ber-filter desa); non-kosong → h.A dgn tap target 44px.
func relatedRecordChip(c RelatedRecordChip) g.Node {
	body := []g.Node{
		h.Div(h.Class("text-xs text-base-content/60"), g.Text(c.Label)),
		h.Div(h.Class("text-sm font-medium break-words"), g.Text(orDash(c.ValueText))),
	}
	if c.Href != "" {
		return h.A(
			h.Href(c.Href),
			h.Class("rounded-box border border-base-300 p-3 min-h-11 min-w-0 flex flex-col justify-center gap-1 hover:bg-base-200"),
			g.Group(body),
		)
	}
	return h.Div(
		h.Class("rounded-box border border-base-300 p-3 min-h-11 min-w-0 flex flex-col justify-center gap-1"),
		g.Group(body),
	)
}
