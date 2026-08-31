package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/session"
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	html "maragu.dev/gomponents/html"
)

// render_page.go — jalur render Layout landing/app (renderPage) + navigasi turunannya
// (devNav, quickLinksFor). Jalur AppShell/sidebar ada di render_shell.go; setter
// konfigurasi global di render.go. Dipecah agar tiap file di bawah ambang 150.
// renderPage mengirim halaman penuh (navigasi biasa, §4.4 jalur 1). Email &
// avatar untuk nav dibaca dari SESSION (di-set saat login) — tanpa hit DB tiap
// render. Error di-log, tak ditelan.
func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, title string, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d := ui.LayoutData{
		Brand:     appName,
		Title:     title,
		UserEmail: session.Email(r.Context()),
		AvatarURL: session.AvatarURL(r.Context()),
		CSSPath:   cssPath,
	}
	if err := ui.Layout(d, body).Render(w); err != nil {
		h.Log.Error("render page", "path", r.URL.Path, "err", err)
	}
}

// devNav membangun menu panel /dev. "File Health" hanya di dev (route-nya tak
// terdaftar di produksi — source .go tak ada di single-binary).
//
// "Pengaturan" mengikuti IZIN, bukan sekadar berada di panel /dev: aturannya
// berlaku untuk setiap user di platform, jadi staff tak boleh mengubahnya.
// Menampilkan menu yang pasti ditolak route = menu hantu (pola quickLinksFor).
func devNav(ctx context.Context) []ui.NavItem {
	items := []ui.NavItem{
		{Label: "Users", Href: "/dev/users", Icon: lucide.Users(html.Class("size-4"))},
		{Label: "Workspaces", Href: "/dev/workspaces", Icon: lucide.Building2(html.Class("size-4"))},
		{Label: "User Logs", Href: "/dev/logs", Icon: lucide.ChartColumn(html.Class("size-4"))},
	}
	if authz.Can(ctx, "platform:settings", "write") {
		items = append(items, ui.NavItem{
			Label: "Pengaturan", Href: "/dev/settings", Icon: lucide.Settings(html.Class("size-4")),
		})
	}
	if devMode {
		items = append(items,
			ui.NavItem{Label: "File Health", Href: "/dev/health", Icon: lucide.Activity(html.Class("size-4"))},
			ui.NavItem{Label: "Database ERD", Href: "/dev/erd", Icon: lucide.Database(html.Class("size-4"))},
		)
	}
	return items
}

// quickLinksFor membangun pintasan lintas-panel sesuai IZIN user (Casbin,
// di-precompute di sini — bukan dari dalam gomponents). Pintasan /dev menuju
// /dev/users (route "/dev" telanjang tak ada). Hanya muncul untuk yang berhak.
//
// Sejak 0004 hanya tersisa DUA tujuan lintas-panel: platform (/dev) dan ruang
// kerja aktif. Pintasan "Admin" & "User" yang lama hilang bersama peleburannya —
// keduanya kini satu tempat yang sama, dibedakan oleh aksi, bukan alamat.
func quickLinksFor(ctx context.Context) []ui.NavItem {
	var links []ui.NavItem
	if authz.Can(ctx, "dev:users", "read") {
		links = append(links, ui.NavItem{
			Label: "Developer", Href: "/dev/users", Icon: lucide.Terminal(html.Class("size-4")),
		})
	}
	// Hanya bila user benar-benar punya workspace: tanpa slug, wsPathOf jatuh ke
	// /workspace/new — pintasan "Ruang Kerja" yang mengantar ke form buat-baru
	// adalah janji palsu, lebih baik tak ditampilkan.
	if session.TenantSlug(ctx) != "" && authz.Can(ctx, "user:home", "read") {
		links = append(links, ui.NavItem{
			Label: "Ruang Kerja", Href: wsPathOf(ctx, ""), Icon: lucide.House(html.Class("size-4")),
		})
	}
	return links
}
