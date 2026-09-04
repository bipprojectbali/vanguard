package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// cs_trainings_actions.go — tombol & panel aksi inline daftar Training,
// dipisah dari cs_trainings_list.go (ukuran file). Murni-data; render identik.

func csTrainingStatusForm(base, id, currentStatusLabel, notes string) g.Node {
	switch currentStatusLabel {
	case "Scheduled":
		return csTrainingActions(base, id, notes, true)
	case "Rescheduled":
		return csTrainingActions(base, id, notes, false)
	default: // Completed/Cancelled → bisa dibuka ulang (status-only)
		return h.Div(h.Class("flex flex-wrap items-center gap-1"),
			csTrainingStatusBtn(base, id, "scheduled", "Buka Ulang", "btn-ghost"))
	}
}

// csTrainingActions membangun tombol + panel inline. allowReschedule=false
// untuk baris Rescheduled (tak menawarkan jadwal-ulang lagi, cermin transisi
// lama). Signal per-baris (done{id}/resc{id}) unik agar tak bertabrakan lintas
// baris di halaman yang sama.
func csTrainingActions(base, id, notes string, allowReschedule bool) g.Node {
	post := base + "/trainings/" + id + "/status"
	doneSig, rescSig := "done"+id, "resc"+id

	sig := map[string]any{doneSig: false}
	buttons := []g.Node{
		h.Button(h.Type("button"), h.Class("btn btn-xs btn-success min-h-11"),
			data.On("click", "$"+doneSig+" = !$"+doneSig), g.Text("✓ Selesai")),
	}
	panels := []g.Node{csTrainingDonePanel(post, notes, doneSig)}
	if allowReschedule {
		sig[rescSig] = false
		buttons = append(buttons,
			h.Button(h.Type("button"), h.Class("btn btn-xs btn-warning min-h-11"),
				data.On("click", "$"+rescSig+" = !$"+rescSig), g.Text("Jadwal Ulang")))
		panels = append(panels, csTrainingReschedulePanel(post, notes, rescSig))
	}
	buttons = append(buttons, csTrainingStatusBtn(base, id, "cancelled", "Batal", "text-error btn-ghost"))

	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		data.Signals(sig),
		h.Div(h.Class("flex flex-wrap items-center gap-1"), g.Group(buttons)),
		g.Group(panels),
	)
}

// csTrainingStatusBtn = form submit-langsung status-only (Batal/Buka Ulang).
func csTrainingStatusBtn(base, id, targetStatus, label, extraCls string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/trainings/"+id+"/status"),
		h.Input(h.Type("hidden"), h.Name("status"), h.Value(targetStatus)),
		h.Button(h.Type("submit"), h.Class("btn btn-xs min-h-11 "+extraCls),
			g.Text(label)),
	)
}

// csTrainingActionPanel = kerangka form aksi inline (border/bg/padding sama),
// hidden status + field spesifik + tombol simpan berwarna. Tampil saat $sig true.
func csTrainingActionPanel(post, sig, status, btnColorCls string, fields ...g.Node) g.Node {
	body := []g.Node{
		h.Method("post"), h.Action(post),
		h.Class("grid gap-2 rounded-box border border-base-300 bg-base-200 p-3 min-w-0"),
		h.Input(h.Type("hidden"), h.Name("status"), h.Value(status)),
	}
	body = append(body, fields...)
	body = append(body, h.Button(h.Type("submit"),
		h.Class("btn btn-xs "+btnColorCls+" min-h-11"), g.Text("Simpan")))
	return showWhen("$"+sig, "min-w-0", h.FormEl(body...))
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

// csTrainingReschedulePanel = panel "Jadwal Ulang": tanggal & jam baru (wajib)
// + catatan, submit status=rescheduled. Tampil saat $resc{id} true.
func csTrainingReschedulePanel(post, notes, sig string) g.Node {
	return csTrainingActionPanel(post, sig, "rescheduled", "btn-warning",
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
