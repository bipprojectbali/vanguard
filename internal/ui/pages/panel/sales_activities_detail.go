package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_activities_detail.go — halaman detail satu aktivitas. Murni-data:
// TargetLabel & ContactLabel SUDAH diresolusi handler (best-effort, label cadangan
// bila di luar tenant/terhapus). Kolom kosong → "—". Reuse detailCard/detailField
// (accounts_detail.go) & badge (sales_activities.go). Ubah status = kontrol NATIVE
// POST (gotcha #16), hanya untuk kind Task.

// ActivityDetailView = seluruh data satu aktivitas siap render. Field per-kind
// diisi hanya untuk kind terkait; sisanya "" → kartu-nya tak dirender.
type ActivityDetailView struct {
	Base string
	ID   int64

	Kind        string
	Subject     string
	TargetType  string
	TargetID    int64
	TargetLabel string
	Owner       string
	Created     string
	Updated     string

	// Task
	Status       string
	TaskStatuses []string
	DueDate      string
	Priority     string

	// Call
	ContactLabel string
	Direction    string
	ActivityAt   string
	Duration     string
	CallResult   string

	// Note
	Body string

	// Bersama (task/call)
	Notes string

	CanWrite bool
}

// ActivityDetail merender hub detail: header (subjek + jenis + status + aksi),
// back link, kontrol ubah status (Task, bila boleh tulis), kartu identitas, dan
// kartu field per-kind.
func ActivityDetail(v ActivityDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	base := v.Base + "/activities/" + idStr

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(v.Subject)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				activityKindBadge(v.Kind),
				ui.When(v.Kind == "task", activityStatusBadge(v.Status)),
			),
		),
		ui.When(v.CanWrite, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(base+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			deleteActivityForm(base),
		)),
	)

	body := []g.Node{
		header,
		h.A(h.Href(v.Base+"/activities"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar aktivitas")),
		ui.When(v.CanWrite && v.Kind == "task", activityStatusControl(v, base)),
		activityIdentityCard(v),
	}
	body = append(body, activityKindCard(v))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// activityIdentityCard = kartu inti bersama semua kind: target (TAUTAN ke
// halaman target), pemilik, dibuat, diperbarui.
func activityIdentityCard(v ActivityDetailView) g.Node {
	seg, _ := activityTargetParts(v.TargetType, v.TargetID)
	var targetVal g.Node
	if seg == "" {
		targetVal = g.Text(orDash(v.TargetLabel))
	} else {
		targetVal = h.A(
			h.Href(v.Base+"/"+seg+"/"+strconv.FormatInt(v.TargetID, 10)),
			h.Class("link link-hover"), g.Text(orDash(v.TargetLabel)))
	}

	row := func(label string, value g.Node) g.Node {
		return h.Div(
			h.Class("grid gap-1 sm:grid-cols-3 sm:gap-2 py-2 border-b border-base-300/50 last:border-0"),
			h.Dt(h.Class("text-sm text-base-content/60"), g.Text(label)),
			h.Dd(h.Class("sm:col-span-2 break-words"), value),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Identitas")),
			h.Dl(
				h.Class("min-w-0"),
				row("Target", targetVal),
				row("Pemilik", g.Text(orDash(v.Owner))),
				row("Dibuat", g.Text(orDash(v.Created))),
				row("Diperbarui", g.Text(orDash(v.Updated))),
			),
		),
	)
}

// activityKindCard = kartu field spesifik per kind (detailCard). Cabang menentukan
// field mana yang muncul; kind tak dikenal → kartu kosong tak dirender.
func activityKindCard(v ActivityDetailView) g.Node {
	switch v.Kind {
	case "task":
		return detailCard("Detail Tugas", []detailField{
			{"Jatuh Tempo", v.DueDate},
			{"Prioritas", v.Priority},
			{"Status", v.Status},
			{"Catatan", v.Notes},
		})
	case "call":
		return detailCard("Detail Panggilan", []detailField{
			{"Kontak", v.ContactLabel},
			{"Arah", v.Direction},
			{"Waktu", v.ActivityAt},
			{"Durasi (menit)", v.Duration},
			{"Hasil", v.CallResult},
			{"Catatan", v.Notes},
		})
	case "note":
		return detailCard("Catatan", []detailField{
			{"Isi", v.Body},
		})
	default:
		return g.Text("")
	}
}

// activityStatusControl = kontrol ubah status Task: NATIVE POST (gotcha #16).
// Select status + submit. Backend tetap penjaga (validTaskStatuses + F3).
func activityStatusControl(v ActivityDetailView, base string) g.Node {
	opts := make([]g.Node, 0, len(v.TaskStatuses))
	for _, s := range v.TaskStatuses {
		attrs := []g.Node{h.Value(s)}
		if s == v.Status {
			attrs = append(attrs, h.Selected())
		}
		opts = append(opts, h.Option(append(attrs, g.Text(s))...))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Ubah Status")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/status"),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0 items-end"),
				h.Div(
					h.Class("grid gap-1 min-w-0"),
					labelFor("Status", "f-status", true),
					h.Select(
						append([]g.Node{
							h.ID("f-status"), h.Name("status"), h.Required(),
							h.Class("select text-base w-full"),
						}, g.Group(opts))...,
					),
				),
				h.Div(
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Status")),
				),
			),
		),
	)
}

// deleteActivityForm = tombol hapus (soft-delete). Form NATIVE POST → 303 (gotcha #16).
func deleteActivityForm(base string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text("Hapus")),
	)
}
