package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_detail.go — halaman detail satu langganan + riwayat rantai
// renewal (Modul 5, M5-3b GET-only). Murni-data: MRR/ARR SUDAH disamarkan handler
// (F4; ARR bisa flsHidden). Reuse detailCard/detailField & subStatusBadge dari
// subscriptions.go. Aksi (renew/churn) belum ada — hanya bacaan.
//
// SubItemRow + subItemsCard (kartu "Rincian Paket") dipindah ke
// subscriptions_detail_items.go — dipisah krn ambang File Health yang sama.

// SubDetailView = seluruh data satu langganan siap render. Village/Plan sudah
// diresolusi handler (best-effort, label cadangan bila di luar tenant/terhapus).
// Chain = riwayat rantai renewal (lama→baru), tiap periode ARR-nya disamarkan.
//
// BL-154: redesign kartu 2-kolom (Identitas · Status & Lifecycle · Financials ·
// Renewal · System & Audit). Field baru dikelompokkan per kartu di bawah; field
// lama (MRR/ARR/BillingCycle/Start/Seats/dll) tak berubah arti, hanya pindah
// kartu. 3 field mockup TANPA kolom DB (Setup Fee, Payment Method, Last Invoice)
// SENGAJA di-drop dari v1 (keputusan user, docs/crm/tasks.md BL-154).
type SubDetailView struct {
	Base       string
	ID         int64
	EntityCode string

	Village   string
	AccountID int64
	Plan      string
	Status    string

	MRR          string
	ARR          string
	BillingCycle string
	Start        string
	End          string
	Seats        string
	PaymentState string
	Owner        string

	// Kartu Identitas & Langganan (tambahan BL-154).
	ContractTerm string

	// Kartu Financials (tambahan BL-154). MRR/ARR/PaymentState di atas dipakai
	// ulang di kartu ini.
	Discount string

	// Kartu Status & Lifecycle (BL-154, lintas-modul customer_success — BL-114).
	// Kosong (baris customer_success belum ada) → "—" di view, bukan error.
	HealthLabel        string
	HealthBadgeClass   string
	OnboardingStatus   string
	OnboardingProgress string
	ActivatedAt        string

	// Kartu Renewal (BL-154). DaysToRenewal/RenewalTypeLabel/RenewalStatusLabel
	// derivasi handler (reuse logika kartu/tab Renewals agar badge selaras).
	DaysToRenewal      string
	RenewalTypeLabel   string
	RenewalStatusLabel string
	RenewalStatusClass string
	PrevToCurrent      string

	// Kartu System & Audit (BL-154). SourceDealHref kosong → "—" tanpa tautan
	// (langganan tak berasal dari deal).
	CreatedByName   string
	CreatedAt       string
	UpdatedByName   string
	UpdatedAt       string
	SourceDealLabel string
	SourceDealHref  string

	Chain []SubChainRow

	// Items = baris paket langganan (BL-88 PR2b subscription_items). Kolom "Paket"
	// header = "N paket" bila >1 (dirakit handler); tabel ini merinci tiap paket.
	Items []SubItemRow

	// Umpan balik PRG (?ok=/?err=) — dirakit handler (subscriptionsMsg/wsErrMsg).
	Msg string
	Err string

	// Flag aksi (M5-3c) — SUDAH dihitung handler (view murni-data, tak panggil
	// authz). Menentukan form mana yang tampil; status dicek ulang di sini agar
	// tak menampilkan aksi yang pasti ditolak backend.
	CanRenew    bool
	CanChurn    bool
	CanApprove  bool
	CanActivate bool

	// Domain dropdown churn (dari handler; satu sumber dgn validasi backend).
	ChurnReasons []string
	ChurnTypes   []string
}

// SubItemRow — lihat subscriptions_detail_items.go.

// SubChainRow = satu periode di rantai renewal. IsThis menandai baris yang sedang
// dibuka. MRR/ARR sudah diformat & disamarkan handler.
type SubChainRow struct {
	ID     int64
	IsThis bool
	Status string
	MRR    string
	ARR    string
	Start  string
	End    string
}

// SubDetail merender detail: header (desa + kode + status), kartu identitas,
// kartu nilai & masa berlaku, lalu riwayat rantai renewal.
func SubDetail(v SubDetailView) g.Node {
	// Tombol aksi (BL-125) pindah ke kanan-atas header — tak lagi kartu "Tindakan".
	// Modal dialog-nya dirender terpisah di body (subActionDialogs).
	triggers := subActionTriggers(v)
	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(orDash(v.Village))),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				ui.When(v.EntityCode != "", h.Span(
					h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
				subStatusBadge(v.Status),
			),
		),
		ui.When(len(triggers) > 0, h.Div(
			h.Class("flex flex-wrap items-center gap-2 shrink-0"),
			g.Group(triggers),
		)),
	)

	villageLink := h.A(
		h.Href(v.Base+"/accounts/"+strconv.FormatInt(v.AccountID, 10)),
		h.Class("link link-hover"), g.Text(orDash(v.Village)))

	// BL-154: grid 2-kolom (mobile 1-kolom) — pola sama accounts_detail.go
	// (interleaved pairs supaya jatuh berdampingan di md:grid-cols-2). Kartu
	// Financials gantikan "Nilai & Masa Berlaku" (4 field sesuai mockup: MRR,
	// ARR, Diskon, Status Pembayaran — field lain pindah ke Identitas/Renewal).
	cardsGrid := h.Div(
		h.Class("grid grid-cols-1 md:grid-cols-2 gap-4 min-w-0 items-stretch"),
		subIdentityCard(v, villageLink),
		subStatusLifecycleCard(v),
		detailCard("Financials", []detailField{
			{"MRR", v.MRR},
			{"ARR", v.ARR},
			{"Diskon (%)", v.Discount},
			{"Status Pembayaran", v.PaymentState},
		}),
		subRenewalCard(v),
		subSystemAuditCard(v),
	)

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.A(h.Href(v.Base+"/subscriptions"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar langganan")),
		ui.When(v.Msg != "", ui.Toast(ui.VariantSuccess, "subs-ok", g.Text(v.Msg))),
		ui.When(v.Err != "", ui.Toast(ui.VariantDestructive, "subs-err", g.Text(v.Err))),
		cardsGrid,
		// Tabel Items & Renewal Chain tetap penuh-lebar DI BAWAH grid kartu
		// (bukan masuk salah satu kartu) — sudah dibungkus ui.TableScroll.
		subItemsCard(v),
		subRenewalChainCard(v),
		// Modal aksi (BL-125): pemicunya di header; dialog checkbox-toggle disisipkan
		// di sini (posisi DOM bebas — visibilitas via checkbox, bukan aliran dokumen).
		g.Group(subActionDialogs(v)),
		// Pengelompokan ribuan + saring non-digit utk input "MRR baru" (data-numgroup
		// di subRenewForm): reformat() numgroup.js membuang non-digit pada TIAP input
		// (bukan sekadar saat submit) — huruf mustahil bertahan di field. Dimuat HANYA
		// saat form perpanjang tampil (kondisi sama subActionCard). Backend
		// cleanThousands tetap penjaga bila JS mati.
		ui.When(v.CanRenew && v.Status == "Active",
			h.Script(h.Src("/static/numgroup.js"), h.Defer())),
	)
}

// subItemsCard — lihat subscriptions_detail_items.go.

// subIdentityCard = kartu "Identitas & Langganan" (BL-154: diperluas dgn Siklus
// Tagih/Mulai/Termin Kontrak/Jumlah Seat — pindah dari bekas kartu "Nilai &
// Masa Berlaku"). Desa dirender sebagai TAUTAN (membuka detail desa), sisanya
// field biasa via detailRow (mendukung g.Node utk tautan).
func subIdentityCard(v SubDetailView, villageLink g.Node) g.Node {
	return cardRows("Identitas & Langganan", "",
		detailRow("Desa", villageLink),
		detailRow("Paket", g.Text(orDash(v.Plan))),
		detailRow("Siklus Tagih", g.Text(orDash(v.BillingCycle))),
		detailRow("Mulai", g.Text(orDash(v.Start))),
		detailRow("Termin Kontrak (bulan)", g.Text(orDash(v.ContractTerm))),
		detailRow("Jumlah Seat", g.Text(orDash(v.Seats))),
		detailRow("Pemilik", g.Text(orDash(v.Owner))),
	)
}

// subRenewalChainCard = riwayat rantai renewal (lama→baru). Baris yang sedang
// dibuka ditandai. Dibungkus ui.TableScroll (scroll terkurung di mobile).
func subRenewalChainCard(v SubDetailView) g.Node {
	head := h.H2(h.Class("font-semibold mb-2"), g.Text("Riwayat Perpanjangan"))
	if len(v.Chain) <= 1 {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0"), head,
				h.P(h.Class("text-sm text-base-content/60"),
					g.Text("Belum ada perpanjangan — ini periode langganan pertama."))),
		)
	}
	rows := make([]g.Node, 0, len(v.Chain))
	for _, c := range v.Chain {
		rows = append(rows, subChainRow(v.Base, c))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			head,
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Periode")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("MRR")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("ARR")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Mulai")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Berakhir")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func subChainRow(base string, c SubChainRow) g.Node {
	href := base + "/subscriptions/" + strconv.FormatInt(c.ID, 10)
	period := g.Node(h.A(h.Href(href), h.Class("link link-hover"),
		g.Text("Langganan #"+strconv.FormatInt(c.ID, 10))))
	if c.IsThis {
		period = h.Span(h.Class("flex items-center gap-2"),
			g.Text("Langganan #"+strconv.FormatInt(c.ID, 10)),
			h.Span(h.Class("badge badge-primary badge-sm"), g.Text("Ini")))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4 font-medium"), period),
		h.Td(h.Class("py-2 pr-4"), subStatusBadge(c.Status)),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(c.MRR))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(c.ARR))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(c.Start))),
		h.Td(h.Class("py-2"), g.Text(orDash(c.End))),
	)
}
