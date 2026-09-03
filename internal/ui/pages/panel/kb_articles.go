package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// kb_articles.go — view katalog Knowledge Base (Modul 6 Customer Success,
// slice A3). Murni-data: ViewCount/HelpfulVotes/UpdatedAt sudah diformat
// handler. Meniru playbooks.go (slice A2). Status Draft/Published/Archived
// (BL-34) — Archived (arsip) disembunyikan dari tab Aktif, tampil di tab
// Arsip; status='Draft' = draf (baris tetap tampil di tab Aktif, ditandai
// badge). Transisi status = native POST (gotcha #16), aksi tersendiri per
// baris, TOMBOL BERBEDA per status saat ini (lihat kbArticleRowActions).
//
// KPI cards & filter tab kaya wireframe 6.10 SENGAJA tak ada di slice ini —
// lihat komentar kb_articles_page.go (handler) untuk alasan lengkap (tab yang
// ADA cuma Aktif/Arsip, BL-34). Kolom "Rating" menampilkan HITUNGAN suara
// mentah (bukan persentase — total suara tak ada di skema). Badge status
// 3-nilai: Draf (ghost) / Terbit (success) / Arsip (neutral). PENYIMPANGAN
// SADAR dari wireframe (yang menampilkan Review) — keputusan user 3 Sep,
// dicatat di docs/crm/sistem-dan-role.md §6.10.

// KBArticleRow = satu artikel untuk baris tabel kelola. ViewCount/
// HelpfulVotes/UpdatedAt SUDAH diformat handler.
type KBArticleRow struct {
	ID           int64
	ArticleTitle string
	Category     string
	ViewCount    int
	HelpfulVotes int
	Status       string
	UpdatedAt    string
}

// KBArticleListView = data halaman /kb-articles. CanWrite (admin/manager/
// support) memunculkan tombol tulis & aksi baris. Keyset lewat NextCursor
// (BL-6): "" = ujung daftar.
type KBArticleListView struct {
	Base       string
	CanWrite   bool
	Tab        string // BL-34: "aktif" (default, non-Archived) | "arsip"
	Err        string
	Msg        string
	Items      []KBArticleRow
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// KBArticleList merender halaman katalog: header + alert + tabel.
func KBArticleList(v KBArticleListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Knowledge Base")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Katalog artikel bantuan mandiri — bantu pelanggan menemukan jawaban tanpa membuka tiket.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/kb-articles/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Artikel Baru"),
			)),
		),
		kbArticlesTabs(v),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "kb-articles-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "kb-articles-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyKBArticles(v.Tab))
	} else {
		body = append(body, kbArticlesTable(v), kbArticlesPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// kbArticlesTabs = tab LINK Aktif / Arsip (BL-34). Navigasi <a> biasa (lolos
// gotcha #16). Arsip = artikel pensiun (status Archived), disembunyikan dari
// tab Aktif. Meniru pola subStatusFilter (subscriptions.go).
func kbArticlesTabs(v KBArticleListView) g.Node {
	tab := func(label, key string) g.Node {
		href := panelListHref(v.Base+"/kb-articles", [2]string{"tab", kbTabParam(key)})
		cls := "tab min-h-11"
		if v.Tab == key {
			cls += " tab-active font-medium"
		}
		return h.A(h.Href(href), h.Class(cls), g.Text(label))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"),
		tab("Aktif", "aktif"),
		tab("Arsip", "arsip"),
	)
}

// kbTabParam = nilai ?tab= kanonik untuk sebuah tab. Aktif = default lenient →
// param KOSONG (URL bersih `/kb-articles`, panelListHref melewatinya); hanya
// "arsip" yang membawa ?tab=arsip. Menjaga URL default sama seperti sebelum
// BL-34 (pager & tautan tetap `?after=` polos).
func kbTabParam(tab string) string {
	if tab == "arsip" {
		return "arsip"
	}
	return ""
}

// kbArticlesPager = tautan keyset "Berikutnya »" (native <a>, lolos gotcha #16).
// NextCursor kosong = ujung daftar. flex-wrap agar tak mendorong lebar di 375px.
func kbArticlesPager(v KBArticleListView) g.Node {
	// baseHref kanonik memuat ?tab= agar Berikutnya »/« Sebelumnya tetap di
	// tab yang sama (BL-34 × BL-7). Aktif = default → param kosong (URL polos).
	base := panelListHref(v.Base+"/kb-articles", [2]string{"tab", kbTabParam(v.Tab)})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}

func emptyKBArticles(tab string) g.Node {
	msg := "Belum ada artikel di katalog."
	if tab == "arsip" {
		msg = "Arsip kosong — belum ada artikel yang diarsipkan."
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text(msg))),
	)
}

// kbArticlesTable = tabel katalog, dibungkus ui.TableScroll (scroll
// terkurung, tak meluberkan viewport 375px).
func kbArticlesTable(v KBArticleListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Judul Artikel")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kategori")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Dilihat")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Rating")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Diperbarui")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, a := range v.Items {
		rows = append(rows, kbArticleTableRow(v.Base, a, v.CanWrite))
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

func kbArticleTableRow(base string, a KBArticleRow, canWrite bool) g.Node {
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(orDash(a.ArticleTitle))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(a.Category))),
		h.Td(h.Class("py-2 pr-4"), g.Text(strconv.Itoa(a.ViewCount))),
		h.Td(h.Class("py-2 pr-4"), g.Text(helpfulVotesLabel(a.HelpfulVotes))),
		h.Td(h.Class("py-2 pr-4"), g.Text(a.UpdatedAt)),
		h.Td(h.Class("py-2 pr-4"), kbArticleStatusBadge(a.Status)),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), kbArticleRowActions(base, a)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// helpfulVotesLabel memformat hitungan suara membantu ("12 suara"); 0 →
// "—". Bukan persentase — total suara tak tersimpan di skema (lihat
// komentar kb_articles_page.go).
func helpfulVotesLabel(n int) string {
	if n == 0 {
		return "—"
	}
	return strconv.Itoa(n) + " suara"
}

// kbArticleStatusBadge = badge status pakai token semantik daisyUI (bukan
// absolut). BL-34: Draf (ghost) / Terbit (success) / Arsip (neutral) —
// gerbang Review dibuang, Arsip = pensiun-tanpa-hapus. PENYIMPANGAN SADAR dari
// wireframe 6.10 (yang menampilkan Review); dicatat di sistem-dan-role.md §6.10.
func kbArticleStatusBadge(status string) g.Node {
	switch status {
	case "Published":
		return h.Span(h.Class("badge badge-success"), g.Text("Terbit"))
	case "Archived":
		return h.Span(h.Class("badge badge-neutral"), g.Text("Arsip"))
	default:
		return h.Span(h.Class("badge badge-ghost"), g.Text("Draf"))
	}
}

// kbArticleRowActions = Sunting + tombol transisi status sesuai status saat
// ini (BL-34). flex-wrap agar tak mendorong lebar tabel di mobile.
//
//   - Draft     → Terbitkan
//   - Published → Kembalikan ke Draf, Arsipkan
//   - Archived  → Pulihkan (→ Draf)
func kbArticleRowActions(base string, a KBArticleRow) g.Node {
	id := strconv.FormatInt(a.ID, 10)
	nodes := []g.Node{
		h.A(h.Href(base+"/kb-articles/"+id+"/edit"), h.Class("btn btn-ghost btn-xs min-h-11"),
			g.Text("Sunting")),
	}
	switch a.Status {
	case "Draft":
		nodes = append(nodes, kbArticleActionForm(base, id, "publish", "Terbitkan", "text-success"))
	case "Published":
		nodes = append(nodes,
			kbArticleActionForm(base, id, "return-to-draft", "Kembalikan ke Draf", "text-warning"),
			kbArticleActionForm(base, id, "archive", "Arsipkan", "text-error"),
		)
	case "Archived":
		nodes = append(nodes, kbArticleActionForm(base, id, "unarchive", "Pulihkan", "text-info"))
	}
	return h.Div(h.Class("flex flex-wrap items-center gap-2"), g.Group(nodes))
}

func kbArticleActionForm(base, id, path, label, colorClass string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/kb-articles/"+id+"/"+path),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-xs "+colorClass+" min-h-11"),
			g.Text(label)),
	)
}

// ── Form buat/sunting artikel ────────────────────────────────────────────

// KBArticleFormFields = nilai prefill form (semua string agar view netral
// terhadap tipe DB). Kosong (buat) atau terisi (sunting). status TAK di
// sini — transisi jalur tersendiri.
type KBArticleFormFields struct {
	ArticleTitle string
	ArticleBody  string
	Category     string
	Keywords     string
	Visibility   string
}

// KBArticleFormView = data halaman form. Action = URL POST tujuan.
// Visibility/CategoryOptions dioper handler (view tak memutuskan enum;
// category jadi enum terkunci sejak BL-35).
type KBArticleFormView struct {
	Base              string
	Action            string
	IsEdit            bool
	Err               string
	Fields            KBArticleFormFields
	VisibilityOptions []string
	CategoryOptions   []string
}

// KBArticleForm merender halaman form lengkap (native POST → 303, gotcha
// #16).
func KBArticleForm(v KBArticleFormView) g.Node {
	title, submit := "Tambah Artikel", "Simpan Artikel"
	if v.IsEdit {
		title, submit = "Sunting Artikel", "Simpan Perubahan"
	}
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/kb-articles"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke katalog")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "kb-article-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Identitas",
			field("Judul Artikel", "article_title", v.Fields.ArticleTitle, true, "text"),
			selectField("Kategori", "category", v.Fields.Category, v.CategoryOptions, false),
			field("Kata Kunci", "keywords", v.Fields.Keywords, false, "text"),
			selectField("Visibilitas", "visibility", v.Fields.Visibility, v.VisibilityOptions, true),
		),
		formCard("Isi",
			textareaField("Isi Artikel", "article_body", v.Fields.ArticleBody),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/kb-articles"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}
