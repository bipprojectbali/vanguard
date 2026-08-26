package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_impl_tasks_form.go — form buat task implementasi baru (Modul 6 Customer
// Success, sub-item Onboarding 6.2.1.1). Murni-data: opsi dropdown (akun,
// anggota) sudah dirakit handler. Native POST → 303 (gotcha #16). Meniru
// engagements_form.go.

// CSImplTaskFormView = data halaman form buat task baru. Action = URL POST
// tujuan. Accounts/Members dioper handler.
type CSImplTaskFormView struct {
	Base     string
	Action   string
	Err      string
	Accounts []CSImplTaskAccountOption
	Members  []CSImplTaskMemberOption
}

// CSImplTaskForm merender form buat task implementasi baru.
func CSImplTaskForm(v CSImplTaskFormView) g.Node {
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Buat Task Implementasi Baru")),
			h.A(h.Href(v.Base+"/impl-tasks"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar task")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "cs-impl-task-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Informasi Task",
			csImplTaskAccountSelect(v.Accounts),
			field("Nama Task", "task_name", "", true, "text"),
		),
		formCard("Penugasan & Jadwal",
			csImplTaskMemberSelect(v.Members),
			csImplTaskDateField("Due Date (opsional)", "due_date", ""),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Buat Task")),
			h.A(h.Href(v.Base+"/impl-tasks"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// csImplTaskAccountSelect = dropdown pilih desa (wajib). Scope sudah
// difilter handler (CSM melihat desanya; Admin/Manager melihat semua).
func csImplTaskAccountSelect(accounts []CSImplTaskAccountOption) g.Node {
	opts := make([]g.Node, 0, len(accounts)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih desa —")))
	for _, a := range accounts {
		opts = append(opts,
			h.Option(h.Value(strconv.FormatInt(a.ID, 10)), g.Text(a.Name)),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Desa", h.For("account_id")),
		h.Select(
			h.ID("account_id"), h.Name("account_id"), h.Required(),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// csImplTaskMemberSelect = dropdown owner / penanggung jawab task (opsional).
func csImplTaskMemberSelect(members []CSImplTaskMemberOption) g.Node {
	opts := make([]g.Node, 0, len(members)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Belum ditugaskan —")))
	for _, m := range members {
		opts = append(opts,
			h.Option(h.Value(strconv.FormatInt(m.ID, 10)), g.Text(m.Name)),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Penanggung Jawab", h.For("owner_id")),
		h.Select(
			h.ID("owner_id"), h.Name("owner_id"),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// csImplTaskDateField = <input type="date"> untuk due_date (opsional).
func csImplTaskDateField(label, name, val string) g.Node {
	attrs := []g.Node{
		h.Type("date"), h.ID(name), h.Name(name),
		h.Class("input w-full text-base"),
	}
	if val != "" {
		attrs = append(attrs, h.Value(val))
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label(label, h.For(name)),
		h.Input(attrs...),
	)
}
