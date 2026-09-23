package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_activities_row.go — row-rendering & badge helper untuk
// ActivitiesList (sales_activities.go: baris tabel, badge jenis/status,
// tautan target). Dipisah krn ambang File Health (View/Component 300
// baris); murni pindah, tak ada perubahan logika.

func activityTableRow(base string, a ActivityRow) g.Node {
	href := base + "/activities/" + strconv.FormatInt(a.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), activityKindBadge(a.Kind))),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(a.Subject))),
		h.Td(h.Class("py-2 pr-4"), activityTargetLink(base, a.TargetType, a.TargetID)),
		link(orDash(a.Owner), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), activityStatusBadge(a.Status))),
		link(orDash(a.Created), "py-2"),
	)
}

// activityKindBadge = badge jenis berwarna token semantik daisyUI (bukan absolut).
// Mencakup seluruh 5 kind yang dibangun M7 (task/meeting/call/chat/note).
func activityKindBadge(kind string) g.Node {
	cls, label := "badge badge-ghost", activityKindLabel(kind)
	switch kind {
	case "task":
		cls = "badge badge-info"
	case "meeting":
		cls = "badge badge-secondary"
	case "call":
		cls = "badge badge-success"
	case "chat":
		cls = "badge badge-accent"
	case "note":
		cls = "badge badge-ghost"
	}
	return h.Span(h.Class(cls), g.Text(label))
}

// activityKindLabel memetakan enum kind → label Indonesia.
// Mencakup seluruh 5 kind M7 (email sengaja dikecualikan iterasi ini).
func activityKindLabel(kind string) string {
	switch kind {
	case "task":
		return "Tugas"
	case "meeting":
		return "Pertemuan"
	case "call":
		return "Panggilan"
	case "chat":
		return "Chat"
	case "note":
		return "Catatan"
	default:
		return kind
	}
}

// activityStatusBadge = badge status Task berwarna token semantik; status kosong
// (call/note) → teks "—" (tak ada daur status).
func activityStatusBadge(status string) g.Node {
	if status == "" {
		return h.Span(h.Class("text-base-content/50"), g.Text("—"))
	}
	cls := "badge badge-ghost"
	switch status {
	case "Completed":
		cls = "badge badge-success"
	case "In Progress":
		cls = "badge badge-info"
	case "Deferred":
		cls = "badge badge-warning"
	}
	return h.Span(h.Class(cls), g.Text(status))
}

// activityTargetLink = tautan ke halaman target (tipe + id, tanpa lookup nama →
// hindari N+1). Label = "Deal #123" / "Desa #45" / "Kontak #7".
func activityTargetLink(base, targetType string, id int64) g.Node {
	seg, label := activityTargetParts(targetType, id)
	if seg == "" {
		return h.Span(h.Class("text-base-content/50"), g.Text("—"))
	}
	return h.A(
		h.Href(base+"/"+seg+"/"+strconv.FormatInt(id, 10)),
		h.Class("link link-hover truncate block"), g.Text(label))
}

// activityTargetParts memetakan target_type → (segmen path, label tampil).
// Segmen kosong untuk tipe tak ber-halaman v1 (tak seharusnya terjadi lewat picker).
func activityTargetParts(targetType string, id int64) (seg, label string) {
	idStr := strconv.FormatInt(id, 10)
	switch targetType {
	case "deal":
		return "deals", "Deal #" + idStr
	case "account":
		return "accounts", "Desa #" + idStr
	case "contact":
		return "contacts", "Kontak #" + idStr
	case "lead":
		return "leads", "Lead #" + idStr
	default:
		return "", ""
	}
}
