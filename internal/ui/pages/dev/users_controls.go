package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// users_controls.go — kontrol per-baris tabel user (/dev/users): kuota, peran
// global & per-workspace, status, dan hapus. Dipisah dari users.go agar file
// induk di bawah ambang tipe View/Component (300). Satu paket dev — perilaku &
// tampilan identik; hanya lokasi deklarasi yang berpindah.

// quotaControl = jatah workspace user + tombol memberi/mencabut hak khusus.
//
// Menampilkan ASAL angkanya, bukan cuma angkanya: "3 global" vs "5 khusus".
// Tanpa penanda itu, operator tak bisa tahu siapa yang akan ikut berubah saat
// default global diubah — justru pertanyaan yang membuat halaman ini berguna.
// Root env dikecualikan: ia super_admin di semua workspace, kuota tak berlaku.
func quotaControl(u UserRow) g.Node {
	if u.IsRoot {
		return h.Span(h.Class("text-base-content/50 text-sm"), g.Text("—"))
	}
	id := strconv.FormatInt(u.ID, 10)
	origin := "global"
	if u.QuotaOverride {
		origin = "khusus"
	}
	nodes := []g.Node{
		h.Method("post"), h.Action("/dev/users/" + id + "/quota"),
		h.Class("flex flex-wrap items-center gap-1 min-w-0"),
		h.Input(
			h.Type("number"), h.Name("quota"),
			h.Value(strconv.Itoa(u.Quota)),
			g.Attr("min", "1"), g.Attr("max", "100"),
			h.Class("input input-sm w-16"),
		),
		h.Button(h.Type("submit"), h.Class("btn btn-sm"), g.Text("Set")),
		h.Span(h.Class("text-xs text-base-content/60"), g.Text(origin)),
	}
	form := h.FormEl(nodes...)
	if !u.QuotaOverride {
		return h.Div(h.Class("flex flex-col gap-1 min-w-0"), form)
	}
	// Hanya yang PUNYA hak khusus yang bisa dikembalikan ke global — tombol pada
	// yang sudah mengikuti global tak melakukan apa-apa selain membingungkan.
	return h.Div(
		h.Class("flex flex-col gap-1 min-w-0"),
		form,
		h.FormEl(
			h.Method("post"), h.Action("/dev/users/"+id+"/quota/reset"),
			h.Button(h.Type("submit"), h.Class("btn btn-xs btn-ghost"),
				g.Text("kembalikan ke global")),
		),
	)
}

// roleControl merender SATU baris kontrol per WORKSPACE tempat user jadi anggota
// (role kini per-workspace, bukan properti user). Root env → badge (immutable).
// Tanpa keanggotaan → penanda "—" (user ada, tapi belum/tak lagi di workspace mana pun).
func roleControl(u UserRow, roles []string, canManageSuper bool) g.Node {
	if u.IsRoot {
		return badge("root (semua workspace)", "")
	}
	if len(u.Workspaces) == 0 {
		return h.Span(h.Class("text-base-content/50 text-sm"), g.Text("—"))
	}
	items := make([]g.Node, 0, len(u.Workspaces))
	for _, ws := range u.Workspaces {
		items = append(items, h.Div(
			h.Class("flex items-center gap-2"),
			h.Span(h.Class("text-xs text-base-content/70 truncate max-w-[10rem]"), g.Text(ws.Name)),
			workspaceRoleSelect(u.ID, roles, ws),
		))
	}
	return h.Div(h.Class("flex flex-col gap-1"), g.Group(items))
}

// workspaceRoleSelect = dropdown ubah role user DI SATU workspace. tenant dikirim
// sebagai hidden field (handler butuh tahu workspace mana). Balasan SSE me-render
// ulang baris + toast (tanpa reload).
func workspaceRoleSelect(userID int64, roles []string, ws WorkspaceRole) g.Node {
	opts := make([]g.Node, 0, len(roles))
	for _, r := range roles {
		opts = append(opts, roleOption(r, ws.Role))
	}
	return ui.FormPostSelectWith(
		"/dev/users/"+strconv.FormatInt(userID, 10)+"/role", "role",
		map[string]string{"tenant": strconv.FormatInt(ws.TenantID, 10)},
		g.Group(opts),
	)
}

func roleOption(val, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(val))...)
}

// statusControl = ubah status via Datastar SSE. Root env → badge (immutable).
func statusControl(u UserRow) g.Node {
	if u.IsRoot {
		return badge(u.Status, "")
	}
	return ui.FormPostSelect("/dev/users/"+strconv.FormatInt(u.ID, 10)+"/status", "status",
		statusOption("active", u.Status),
		statusOption("disabled", u.Status),
		statusOption("blocked", u.Status),
	)
}

func statusOption(val, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(val))...)
}

// deleteControl = tombol hapus (soft-delete) via SSE. Root env kebal → tanpa tombol.
func deleteControl(u UserRow) g.Node {
	if u.IsRoot {
		return g.Text("")
	}
	return h.Button(
		h.Type("button"),
		h.Class("btn btn-error btn-sm"),
		data.On("click", ui.PostAction("/dev/users/"+strconv.FormatInt(u.ID, 10)+"/delete")),
		g.Text("Hapus"),
	)
}
