package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// cs_trainings_actions.go — tombol & panel aksi inline daftar Training,
// dipisah dari cs_trainings_list.go (ukuran file). Murni-data; render identik.

// csTrainingStatusForm = aksi per baris (canWrite). Transisi: scheduled →
// completed/rescheduled/cancelled; rescheduled → completed/cancelled.
// BL-180: completed/cancelled TERKUNCI — tak bisa diedit maupun dibuka ulang
// (ditegakkan juga di query), jadi kolom Aksi diisi penanda saja.
//
// BL-28: "✓ Selesai" TAK langsung submit — membuka panel inline (Datastar
// data-show, state form efemeral; pola BL-19/BL-26 showWhen) untuk
// attendance/peserta/catatan. BL-180: "Jadwal Ulang" & "Edit" membuka MODAL
// (modal.go). "Batal" tetap submit langsung — hanya kirim status; query
// COALESCE menjaga field lain.
func csTrainingStatusForm(base string, r CSTrainingRow, trainers []CSTrainingTrainerOption, back string) g.Node {
	switch r.StatusLabel {
	case "Scheduled":
		return csTrainingActions(base, r, trainers, back, true)
	case "Rescheduled":
		return csTrainingActions(base, r, trainers, back, false)
	default: // Completed/Cancelled → terkunci
		return h.Span(h.Class("text-base-content/40"), g.Text("—"))
	}
}

// back = query kembali (?tab=&q=&account=) yang dijahit ke action tiap form agar
// handler mengembalikan user ke tab/pencarian yang sama setelah submit.
// csTrainingActions = tombol burger (☰) → menu popover berisi aksi. "Selesai" =
// panel inline (signal done{id} unik per baris); "Jadwal Ulang"/"Edit" = dialog
// popover (popoverDialog, id resc-/edit-{id}) — SIBLING menu, jadi membukanya
// menutup menu otomatis. allowReschedule=false untuk baris Rescheduled.
func csTrainingActions(base string, r CSTrainingRow, trainers []CSTrainingTrainerOption, back string, allowReschedule bool) g.Node {
	id, notes := strconv.FormatInt(r.ID, 10), r.Notes
	post := base + "/trainings/" + id + "/status" + back
	doneSig, menuID := "done"+id, "menu-"+id

	items := []g.Node{
		h.Button(h.Type("button"), h.Class(csTrainingMenuItemCls),
			g.Attr("popovertarget", menuID), g.Attr("popovertargetaction", "hide"),
			data.On("click", "$"+doneSig+" = !$"+doneSig), g.Text("Selesai")),
	}
	panels := []g.Node{csTrainingDonePanel(post, notes, doneSig)}
	if allowReschedule {
		rescID := "resc-" + id
		items = append(items, popoverTrigger(rescID, "Jadwal Ulang", csTrainingMenuItemCls))
		panels = append(panels,
			popoverDialog(rescID, "Jadwal Ulang Training", csTrainingRescheduleForm(post, notes)))
	}
	editID := "edit-" + id
	items = append(items,
		popoverTrigger(editID, "Edit", csTrainingMenuItemCls),
		csTrainingStatusBtn(base, id, back, "cancelled", "Batal"))
	panels = append(panels,
		popoverDialog(editID, "Edit Training", csTrainingEditForm(base+"/trainings/"+id+back, r, trainers)))

	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		data.Signals(map[string]any{doneSig: false}),
		popoverTrigger(menuID, "☰", "btn btn-ghost btn-sm min-h-11 min-w-11 text-base",
			g.Attr("aria-label", "Aksi training")),
		popoverMenu(menuID, items...),
		g.Group(panels),
	)
}

// csTrainingMenuItemCls = item menu aksi: lebar penuh, rata kiri, tap ≥44px,
// hanya garis bawah (tanpa border kotak). Item terakhir (Batal) tanpa garis
// agar tak dobel dengan border menu.
const csTrainingMenuItemCls = "btn btn-ghost btn-sm w-full min-h-11 justify-start rounded-none border-0 border-b border-base-300"

const csTrainingMenuLastCls = "btn btn-ghost btn-sm w-full min-h-11 justify-start rounded-none border-0"

// csTrainingStatusBtn = form submit-langsung status-only (item menu "Batal").
func csTrainingStatusBtn(base, id, back, targetStatus, label string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/trainings/"+id+"/status"+back),
		h.Input(h.Type("hidden"), h.Name("status"), h.Value(targetStatus)),
		h.Button(h.Type("submit"), h.Class(csTrainingMenuLastCls),
			g.Text(label)),
	)
}

// csTrainingActionForm = form aksi native POST: hidden status + field spesifik +
// tombol simpan berwarna. cls = class <form> (kotak inline atau polos di modal).
func csTrainingActionForm(post, status, btnColorCls, cls string, fields ...g.Node) g.Node {
	body := []g.Node{
		h.Method("post"), h.Action(post), h.Class(cls),
		h.Input(h.Type("hidden"), h.Name("status"), h.Value(status)),
	}
	body = append(body, fields...)
	body = append(body, h.Button(h.Type("submit"),
		h.Class("btn btn-sm "+btnColorCls+" min-h-11"), g.Text("Simpan")))
	return h.FormEl(body...)
}

// csTrainingActionPanel = form aksi dlm kotak inline (border/bg/padding sama).
// Tampil saat $sig true.
func csTrainingActionPanel(post, sig, status, btnColorCls string, fields ...g.Node) g.Node {
	return showWhen("$"+sig, "min-w-0", csTrainingActionForm(post, status, btnColorCls,
		"grid gap-2 rounded-box border border-base-300 bg-base-200 p-3 min-w-0", fields...))
}

// csTrainingDonePanel = panel "Selesai": attendance% + peserta aktual + catatan
// (semua opsional), submit status=completed. Tampil saat $done{id} true.
func csTrainingDonePanel(post, notes, sig string) g.Node {
	return csTrainingActionPanel(post, sig, "completed", "btn-success",
		csTrainingNumField("Attendance (%)", "attendance", "0.01", "100"),
		csTrainingNumField("Peserta aktual", "participants", "1", ""),
		csTrainingNotesField(notes),
	)
}

// csTrainingRescheduleForm = isi modal "Jadwal Ulang": tanggal & jam baru (wajib)
// + catatan, submit status=rescheduled (native POST → 303).
func csTrainingRescheduleForm(post, notes string) g.Node {
	return csTrainingActionForm(post, "rescheduled", "btn-warning", "grid gap-3 min-w-0",
		h.Label(h.Class("grid gap-1 text-xs"),
			h.Span(g.Text("Tanggal & jam baru")),
			h.Input(h.Type("datetime-local"), h.Name("training_date"), h.Required(),
				h.Class("input input-bordered input-sm text-base min-h-11 w-full"))),
		csTrainingNotesField(notes),
	)
}

// csTrainingNumField = input angka (mobile-first: text-base ≥16px anti
// auto-zoom iOS, tap ≥44px). max "" → tanpa batas atas.
func csTrainingNumField(label, name, step, max string) g.Node {
	in := []g.Node{
		h.Type("number"), h.Name(name), h.Min("0"), h.Step(step),
		h.Class("input input-bordered input-sm text-base min-h-11 w-full"),
	}
	if max != "" {
		in = append(in, h.Max(max))
	}
	return h.Label(h.Class("grid gap-1 text-xs"),
		h.Span(g.Text(label)), h.Input(in...))
}

// csTrainingNotesField = textarea catatan/kesimpulan, diisi ulang dgn nilai
// tersimpan. g.Text meng-escape (gotcha #15).
func csTrainingNotesField(notes string) g.Node {
	return h.Label(h.Class("grid gap-1 text-xs"),
		h.Span(g.Text("Catatan (opsional)")),
		h.Textarea(h.Name("notes"), h.Rows("2"),
			h.Class("textarea textarea-bordered textarea-sm text-base w-full"),
			g.Text(notes)))
}

// csTrainingEditForm = isi modal "Edit": topik (wajib), trainer, perkiraan
// peserta, catatan. Desa & tanggal/jam sengaja tak ada (tanggal = Jadwal Ulang).
// Native POST → 303 (gotcha #16). Tanpa id input agar aman berulang per baris.
func csTrainingEditForm(action string, r CSTrainingRow, trainers []CSTrainingTrainerOption) g.Node {
	opts := make([]g.Node, 0, len(trainers)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Belum ditentukan —")))
	for _, m := range trainers {
		opt := []g.Node{h.Value(strconv.FormatInt(m.ID, 10)), g.Text(m.Name)}
		if m.ID == r.TrainerID {
			opt = append(opt, h.Selected())
		}
		opts = append(opts, h.Option(opt...))
	}
	part := []g.Node{
		h.Type("number"), h.Name("participants"), h.Min("0"), h.Step("1"),
		h.Class("input input-bordered input-sm text-base min-h-11 w-full"),
	}
	if r.ParticipantsRaw != "" {
		part = append(part, h.Value(r.ParticipantsRaw))
	}
	return h.FormEl(
		h.Method("post"), h.Action(action), h.Class("grid gap-3 min-w-0"),
		h.Label(h.Class("grid gap-1 text-xs"),
			h.Span(g.Text("Topik Training")),
			h.Input(h.Type("text"), h.Name("training_topic"), h.Required(),
				h.Value(r.TrainingTopic),
				h.Class("input input-bordered input-sm text-base min-h-11 w-full"))),
		h.Label(h.Class("grid gap-1 text-xs"),
			h.Span(g.Text("Trainer")),
			h.Select(h.Name("trainer_id"),
				h.Class("select select-bordered select-sm text-base min-h-11 w-full"),
				g.Group(opts))),
		h.Label(h.Class("grid gap-1 text-xs"),
			h.Span(g.Text("Perkiraan Peserta (opsional)")), h.Input(part...)),
		csTrainingNotesField(r.Notes),
		h.Button(h.Type("submit"), h.Class("btn btn-xs btn-info min-h-11"), g.Text("Simpan")),
	)
}
