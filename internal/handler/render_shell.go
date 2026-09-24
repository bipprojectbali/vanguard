package handler

import (
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/changelog"
	"go_starter/internal/session"
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
)

// render_shell.go — jalur render AppShell + sidebar (renderShell/renderWorkspaceShell).
// Dipisah dari render.go (setter konfigurasi global) & render_page.go (Layout
// landing/app) agar tiap file di bawah ambang tipe Route/Handler (150). Satu
// paket: headNodes() dibagi ketiga jalur.
//
// workspaceOptions (opsi pemilih workspace) + patch (helper SSE) dipisah ke
// render_shell_workspace.go agar file ini tetap di bawah ambang yang sama.

// renderWorkspaceShell mengirim halaman di dalam ruang kerja. Merakit sendiri
// slug, menu, dan currentPath dari request — pemanggil cukup menyebut sub-path
// (mis. "/members"). Tanpa ini tiap handler workspace mengulang perakitan yang
// sama, dan satu yang keliru menghasilkan menu menunjuk workspace lain.
func (h *Handler) renderWorkspaceShell(w http.ResponseWriter, r *http.Request, title, sub string, body g.Node) {
	ctx := r.Context()
	slug := slugFromRequest(r)
	nav := workspaceNavCtx(ctx, slug)
	h.renderShell(w, r, title, session.TenantName(ctx), wsPath(slug, sub), nav, body)
}

// renderShell mengirim halaman dengan AppShell (sidebar). brand = label brand
// di sidebar, currentPath untuk active-state, nav = menu panel.
func (h *Handler) renderShell(w http.ResponseWriter, r *http.Request, title, brand, currentPath string, nav []ui.NavItem, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	workspaces, canCreate := h.workspaceOptions(r.Context())
	// Slug asal untuk lonceng (BL-103): halaman di dalam workspace punya {workspace}
	// di path; halaman /notifications sendiri membawanya di ?from — dibaca agar
	// lonceng tetap menautkan workspace yang sama saat sudah di /notifications?from=X
	// (kalau tidak, klik lonceng di sana akan memantul balik ke shell dev).
	fromSlug := slugFromRequest(r)
	if fromSlug == "" {
		fromSlug = r.URL.Query().Get("from")
	}
	// wsBase (BL-163): prefix workspace aktif untuk RegionSearchModal — ""
	// di luar konteks workspace (/dev, /notifications), "/w/{slug}" bila
	// path ini ADA {workspace} (slugFromRequest, bukan fromSlug/session —
	// harus path ini sendiri, sama alasan wsRedirect di wspath.go).
	wsBase := ""
	if slug := slugFromRequest(r); slug != "" {
		wsBase = wsPath(slug, "")
	}
	// Jena AI (BL-162 PoC): hanya tampil di halaman /w/{slug} (data tenant milik
	// user yang bertanya, bukan /dev/notifications), digate permission ai:chat/use
	// DAN provider terkonfigurasi (CLAUDE_PROXY_URL/TOKEN) — dua syarat, keduanya
	// dihitung di handler (view tak boleh cek authz/config sendiri).
	showJenaAI := false
	jenaAIPostURL := ""
	if slug := slugFromRequest(r); slug != "" && authz.Can(r.Context(), "ai:chat", "use") && JenaAIConfigured() {
		showJenaAI = true
		jenaAIPostURL = wsPath(slug, "jena-ai/ask")
	}
	d := ui.ShellData{
		Title:              title,
		BrandLabel:         brand,
		WorkspaceName:      session.TenantName(r.Context()), // brand utama; "" utk platform → fallback BrandLabel
		CurrentPath:        currentPath,
		UserEmail:          session.Email(r.Context()),
		AvatarURL:          session.AvatarURL(r.Context()),
		CSSPath:            cssPath,
		Nav:                nav,
		Panel:              h.panelOf(r.Context(), currentPath),
		QuickLinks:         quickLinksFor(r.Context()),
		Notifications:      h.notifBadge(r.Context(), fromSlug),
		Workspaces:         workspaces,
		ActiveTenantID:     session.TenantID(r.Context()),
		CanCreateWorkspace: canCreate,
		ChangelogVersion:   changelog.Current(),
		ChangelogReleases:  changelog.Releases,
		WSBase:             wsBase,
		ShowJenaAI:         showJenaAI,
		JenaAIPostURL:      jenaAIPostURL,
	}
	if err := ui.AppShell(d, body).Render(w); err != nil {
		h.Log.Error("render shell", "path", r.URL.Path, "err", err)
	}
}
