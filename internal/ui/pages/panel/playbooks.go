package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// playbooks.go — view katalog Playbooks (Modul 6 Customer Success, slice
// A2). Murni-data: StepCount sudah dihitung handler, status sudah bool.
// Meniru sla_policies.go (slice A1). playbooks TANPA soft-delete:
// is_active=false = draf — baris tetap tampil di kelola, ditandai badge.
// Draf/aktifkan = native POST (gotcha #16), aksi tersendiri per baris.
//
// KPI eksekusi ("Sedang Berjalan"/"Selesai (Bln)"/"Tingkat Sukses"), kolom
// "Berjalan"/"Sukses", & filter tab wireframe 6.7 SENGAJA tak ada di slice
// ini — butuh data eksekusi playbook yang belum ada. "Langkah" ditampilkan
// sebagai HITUNGAN baris steps (bukan teks penuh — dijaga singkat di tabel;
// isi lengkap di halaman sunting). Badge status "Aktif"/"Draf" (bukan
// "Aktif"/"Pensiun" seperti A1) mengikuti wording wireframe modul ini.

// PlaybookRow = satu playbook untuk baris tabel kelola. StepCount SUDAH
// dihitung handler (countSteps, baris non-kosong pada teks steps).
type PlaybookRow struct {
	ID               int64
	PlaybookName     string
	TriggerScenario  string
	RecommendedOwner string
	StepCount        int
	Active           bool
}

// PlaybookListView = data halaman /playbooks. CanWrite (manager/csm/admin)
// memunculkan tombol tulis & aksi baris. Keyset lewat NextCursor (BL-6):
// "" = ujung daftar.
type PlaybookListView struct {
	Base       string
	CanWrite   bool
	Err        string
	Msg        string
	Items      []PlaybookRow
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// PlaybookList merender halaman katalog: header + alert + tabel.
func PlaybookList(v PlaybookListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Playbooks")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Prosedur respons per skenario — panduan langkah CS saat health drop, adopsi rendah, mendekati renewal, atau onboarding baru.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/playbooks/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Playbook Baru"),
			)),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "playbooks-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "playbooks-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyPlaybooks())
	} else {
		body = append(body, playbooksTable(v), playbooksPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// playbooksPager = tautan keyset "Berikutnya »" (native <a>, lolos gotcha #16).
// NextCursor kosong = ujung daftar. flex-wrap agar tak mendorong lebar di 375px.
func playbooksPager(v PlaybookListView) g.Node {
	base := v.Base + "/playbooks"
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}

func emptyPlaybooks() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada playbook di katalog."))),
	)
}

// playbooksTable = tabel katalog, dibungkus ui.TableScroll (scroll terkurung,
// tak meluberkan viewport 375px).
func playbooksTable(v PlaybookListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama Playbook")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Skenario Pemicu")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Pemilik Rekomendasi")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Langkah")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, p := range v.Items {
		rows = append(rows, playbookTableRow(v.Base, p, v.CanWrite))
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

func playbookTableRow(base string, p PlaybookRow, canWrite bool) g.Node {
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(orDash(p.PlaybookName))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(p.TriggerScenario))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(p.RecommendedOwner))),
		h.Td(h.Class("py-2 pr-4"), g.Text(stepCountLabel(p.StepCount))),
		h.Td(h.Class("py-2 pr-4"), playbookStatusBadge(p.Active)),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), playbookRowActions(base, p)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// stepCountLabel memformat hitungan langkah ("5 langkah"); 0 → "—".
func stepCountLabel(n int) string {
	if n == 0 {
		return "—"
	}
	return strconv.Itoa(n) + " langkah"
}

// playbookStatusBadge = badge status pakai token semantik daisyUI (bukan
// absolut). Wording "Aktif"/"Draf" (wireframe 6.7).
func playbookStatusBadge(active bool) g.Node {
	if active {
		return h.Span(h.Class("badge badge-success"), g.Text("Aktif"))
	}
	return h.Span(h.Class("badge badge-ghost"), g.Text("Draf"))
}

// playbookRowActions = Sunting + Jadikan Draf/Aktifkan. Draf/aktifkan = FORM
// native POST tersendiri (aksi, bukan navigasi) → 303 PRG di handler.
// flex-wrap agar tak mendorong lebar tabel di mobile.
func playbookRowActions(base string, p PlaybookRow) g.Node {
	id := strconv.FormatInt(p.ID, 10)
	toggle := playbookDraftForm(base, id)
	if !p.Active {
		toggle = playbookActivateForm(base, id)
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(base+"/playbooks/"+id+"/edit"), h.Class("btn btn-ghost btn-xs min-h-11"),
			g.Text("Sunting")),
		toggle,
	)
}

func playbookDraftForm(base, id string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/playbooks/"+id+"/draft"),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-xs text-warning min-h-11"),
			g.Text("Jadikan Draf")),
	)
}

func playbookActivateForm(base, id string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/playbooks/"+id+"/activate"),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-xs text-success min-h-11"),
			g.Text("Aktifkan")),
	)
}

// ── Form buat/sunting playbook ───────────────────────────────────────────────

// PlaybookFormFields = nilai prefill form (semua string agar view netral
// terhadap tipe DB). Kosong (buat) atau terisi (sunting).
type PlaybookFormFields struct {
	PlaybookName     string
	TriggerScenario  string
	Description      string
	Steps            string
	RecommendedOwner string
}

// PlaybookFormView = data halaman form. Action = URL POST tujuan.
// TriggerScenarios & RecommendedOwners dioper handler (view tak memutuskan
// enum).
type PlaybookFormView struct {
	Base              string
	Action            string
	IsEdit            bool
	Err               string
	Fields            PlaybookFormFields
	TriggerScenarios  []string
	RecommendedOwners []string
}

// PlaybookForm merender halaman form lengkap (native POST → 303, gotcha #16).
func PlaybookForm(v PlaybookFormView) g.Node {
	title, submit := "Tambah Playbook", "Simpan Playbook"
	if v.IsEdit {
		title, submit = "Sunting Playbook", "Simpan Perubahan"
	}
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/playbooks"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke katalog")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "playbook-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Identitas",
			field("Nama Playbook", "playbook_name", v.Fields.PlaybookName, true, "text"),
			selectField("Skenario Pemicu", "trigger_scenario", v.Fields.TriggerScenario, v.TriggerScenarios, false),
			selectField("Pemilik Rekomendasi", "recommended_owner", v.Fields.RecommendedOwner, v.RecommendedOwners, false),
		),
		formCard("Prosedur",
			textareaField("Deskripsi", "description", v.Fields.Description),
			textareaField("Langkah (satu langkah per baris)", "steps", v.Fields.Steps),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/playbooks"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}
