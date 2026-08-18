package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_trainings_form.go — form buat jadwal training baru (Modul 6 Customer
// Success, sub-item Onboarding 6.2.1.2). Murni-data: opsi dropdown (akun,
// trainer) sudah dirakit handler. Native POST → 303 (gotcha #16). Meniru
// cs_impl_tasks_form.go / engagements_form.go.

// CSTrainingFormView = data halaman form buat training baru. Action = URL
// POST tujuan. Accounts/Trainers dioper handler.
type CSTrainingFormView struct {
	Base     string
	Action   string
	Err      string
	Accounts []CSTrainingAccountOption
	Trainers []CSTrainingTrainerOption
}

// CSTrainingForm merender form buat jadwal training baru.
func CSTrainingForm(v CSTrainingFormView) g.Node {
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Jadwalkan Training Baru")),
			h.A(h.Href(v.Base+"/trainings"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar training")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "cs-training-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Informasi Training",
			csTrainingAccountSelect(v.Accounts),
			field("Topik Training", "training_topic", "", true, "text"),
		),
		formCard("Jadwal & Pengajar",
			csTrainingDateTimeField("Tanggal & Jam Training", "training_date", ""),
			csTrainingTrainerSelect(v.Trainers),
			field("Perkiraan Peserta (opsional)", "participants", "", false, "number"),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Jadwalkan")),
			h.A(h.Href(v.Base+"/trainings"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// csTrainingAccountSelect = dropdown pilih desa (wajib). Scope sudah
// difilter handler (CSM melihat desanya; Admin/Manager melihat semua).
func csTrainingAccountSelect(accounts []CSTrainingAccountOption) g.Node {
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

// csTrainingTrainerSelect = dropdown pengajar training (opsional).
func csTrainingTrainerSelect(trainers []CSTrainingTrainerOption) g.Node {
	opts := make([]g.Node, 0, len(trainers)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Belum ditentukan —")))
	for _, m := range trainers {
		opts = append(opts,
			h.Option(h.Value(strconv.FormatInt(m.ID, 10)), g.Text(m.Name)),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Trainer", h.For("trainer_id")),
		h.Select(
			h.ID("trainer_id"), h.Name("trainer_id"),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// csTrainingDateTimeField = <input type="datetime-local"> untuk training_date
// (WAJIB — kolom NOT NULL, format dateTimeLayout "2006-01-02T15:04").
func csTrainingDateTimeField(label, name, val string) g.Node {
	attrs := []g.Node{
		h.Type("datetime-local"), h.ID(name), h.Name(name), h.Required(),
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
