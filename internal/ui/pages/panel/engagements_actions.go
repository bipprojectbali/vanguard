package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// engagements_actions.go — tombol & panel aksi inline daftar Engagements,
// dipisah dari engagements_list.go (ukuran file). Murni-data; render identik.

func engagementStatusForm(base, id, currentStatusLabel, outcome string) g.Node {
	if currentStatusLabel == "Planned" {
		return engagementActions(base, id, outcome)
	}
	// done / skipped / rescheduled → bisa dibuka ulang ke planned (status-only).
	return h.Div(h.Class("flex flex-wrap items-center gap-1"),
		engagementStatusBtn(base, id, "planned", "Plan Ulang", "btn-ghost"))
}

// engagementActions membangun tombol + panel inline untuk baris "Planned".
// Signal per-baris (done{id}/resc{id}) unik agar tak bertabrakan lintas baris
// di halaman yang sama.
func engagementActions(base, id, outcome string) g.Node {
	post := base + "/engagements/" + id + "/status"
	doneSig, rescSig := "done"+id, "resc"+id

	buttons := []g.Node{
		h.Button(h.Type("button"), h.Class("btn btn-xs btn-success min-h-11"),
			data.On("click", "$"+doneSig+" = !$"+doneSig), g.Text("✓ Done")),
		engagementStatusBtn(base, id, "skipped", "Skip", "btn-ghost"),
		h.Button(h.Type("button"), h.Class("btn btn-xs text-warning btn-ghost min-h-11"),
			data.On("click", "$"+rescSig+" = !$"+rescSig), g.Text("Reschedule")),
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		data.Signals(map[string]any{doneSig: false, rescSig: false}),
		h.Div(h.Class("flex flex-wrap items-center gap-1"), g.Group(buttons)),
		engagementDonePanel(post, outcome, doneSig),
		engagementReschedulePanel(post, rescSig),
	)
}

// engagementStatusBtn = form submit-langsung status-only (Skip/Plan Ulang).
func engagementStatusBtn(base, id, targetStatus, label, extraCls string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/engagements/"+id+"/status"),
		h.Input(h.Type("hidden"), h.Name("status"), h.Value(targetStatus)),
		h.Button(h.Type("submit"), h.Class("btn btn-xs min-h-11 "+extraCls),
			g.Text(label)),
	)
}

// engagementActionPanel = kerangka form aksi inline (border/bg/padding sama),
// hidden status + field spesifik + tombol simpan berwarna. Tampil saat $sig true.
func engagementActionPanel(post, sig, status, btnColorCls string, fields ...g.Node) g.Node {
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

// engagementDonePanel = panel "Done": ringkasan hasil (outcome, opsional) diisi
// ulang dgn nilai tersimpan, submit status=done. Tampil saat $done{id} true.
// g.Text meng-escape (gotcha #15).
func engagementDonePanel(post, outcome, sig string) g.Node {
	return engagementActionPanel(post, sig, "done", "btn-success",
		h.Label(h.Class("grid gap-1 text-xs"),
			h.Span(g.Text("Ringkasan hasil (opsional)")),
			h.Textarea(h.Name("outcome"), h.Rows("2"),
				h.Class("textarea textarea-bordered textarea-sm text-base w-full"),
				g.Text(outcome))),
	)
}

// engagementReschedulePanel = panel "Reschedule": jadwal interaksi baru
// (scheduled_at, WAJIB), submit status=rescheduled. Tampil saat $resc{id} true.
func engagementReschedulePanel(post, sig string) g.Node {
	return engagementActionPanel(post, sig, "rescheduled", "text-warning btn-ghost",
		h.Label(h.Class("grid gap-1 text-xs"),
			h.Span(g.Text("Jadwal interaksi baru")),
			h.Input(h.Type("datetime-local"), h.Name("scheduled_at"), h.Required(),
				h.Class("input input-bordered input-sm text-base min-h-11 w-full"))),
	)
}
