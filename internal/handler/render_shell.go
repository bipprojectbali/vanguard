package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/settings"
	"go_starter/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// render_shell.go — jalur render AppShell + sidebar (renderShell/renderWorkspaceShell),
// opsi pemilih workspace, dan helper SSE patch(). Dipisah dari render.go (setter
// konfigurasi global) & render_page.go (Layout landing/app) agar tiap file di bawah
// ambang tipe Route/Handler (150). Satu paket: headNodes() dibagi ketiga jalur.
// renderWorkspaceShell mengirim halaman di dalam ruang kerja. Merakit sendiri
// slug, menu, dan currentPath dari request — pemanggil cukup menyebut sub-path
// (mis. "/members"). Tanpa ini tiap handler workspace mengulang perakitan yang
// sama, dan satu yang keliru menghasilkan menu menunjuk workspace lain.
func (h *Handler) renderWorkspaceShell(w http.ResponseWriter, r *http.Request, title, sub string, body g.Node) {
	ctx := r.Context()
	slug := slugFromRequest(r)
	nav := workspaceNav(slug, canManageMembers(ctx), canEditWorkspace(ctx), canViewAccounts(ctx), canViewContacts(ctx), canViewLeads(ctx), canViewDeals(ctx), canViewSalesActivity(ctx), canViewAllActivities(ctx), canViewPlans(ctx), canViewSubscriptions(ctx), canViewSLAPolicies(ctx), canViewPlaybooks(ctx), canViewKBArticles(ctx), canManageRoles(ctx), canViewTickets(ctx), canViewHealthScore(ctx), canViewSuccessPlans(ctx), canViewEngagements(ctx), canViewCSRenewals(ctx), canViewReports(ctx), canViewImplTasks(ctx), canViewTrainings(ctx))
	h.renderShell(w, r, title, session.TenantName(ctx), wsPath(slug, sub), nav, body)
}

// renderShell mengirim halaman dengan AppShell (sidebar). brand = label brand
// di sidebar, currentPath untuk active-state, nav = menu panel.
func (h *Handler) renderShell(w http.ResponseWriter, r *http.Request, title, brand, currentPath string, nav []ui.NavItem, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	workspaces, canCreate := h.workspaceOptions(r.Context())
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
		Notifications:      h.notifBadge(r.Context()),
		Workspaces:         workspaces,
		ActiveTenantID:     session.TenantID(r.Context()),
		CanCreateWorkspace: canCreate,
	}
	if err := ui.AppShell(d, body).Render(w); err != nil {
		h.Log.Error("render shell", "path", r.URL.Path, "err", err)
	}
}

// workspaceOptions memuat daftar workspace user (untuk switcher sidebar) +
// apakah ia masih boleh membuat workspace baru (kuota). Fail-soft: error → daftar
// kosong & tak bisa buat (sidebar tetap tampil, brand jadi teks biasa).
// memberships TANPA RLS → aman dibaca lewat h.q(ctx) di scope mana pun.
func (h *Handler) workspaceOptions(ctx context.Context) ([]ui.WorkspaceOption, bool) {
	// Mode single: tak ada yang bisa dipilih maupun dibuat (0006 §4). Switcher
	// disembunyikan dgn mengosongkan daftarnya — brand sidebar otomatis jadi teks
	// biasa, tanpa cabang render tersendiri di view.
	if appmode.IsSingle() {
		return nil, false
	}
	uid := session.UserID(ctx)
	if uid == 0 {
		return nil, false
	}
	// Render TIDAK boleh panic karena wiring: sebagian halaman (mis. hasil test /
	// route tanpa Scope) tak punya Queries ber-scope. Tanpa itu, switcher cukup
	// tak ditampilkan — bukan alasan menggagalkan seluruh halaman.
	q, ok := ctx.Value(scopeCtxKey{}).(*db.Queries)
	if !ok || q == nil {
		return nil, false
	}
	rows, err := q.ListMembershipsByUser(ctx, uid)
	if err != nil {
		h.Log.Error("shell: list workspaces", "err", err)
		return nil, false
	}
	out := make([]ui.WorkspaceOption, 0, len(rows))
	owned := 0
	for _, m := range rows {
		out = append(out, ui.WorkspaceOption{TenantID: m.TenantID, Name: m.Name, Role: m.Role})
		// Workspace primer TIDAK dihitung — sama persis dengan CountOwnedWorkspaces
		// yang menegakkan kuota. Dua hitungan yang berbeda sedikit saja membuat
		// user melihat tombol yang lalu ditolak (atau kehilangan tombol yang masih
		// jadi haknya).
		if m.Role == authz.RoleNameOwner && !m.IsPrimary {
			owned++
		}
	}
	// Kuota EFEKTIF (override per-user, selainnya default global). Sumber yang
	// SAMA dengan penegakan di WorkspaceCreate — kalau berbeda, user melihat
	// tombol "Buat workspace" yang lalu ditolak, atau sebaliknya kehilangan
	// tombol padahal masih berhak.
	quota := 0
	if u, e := q.GetUser(ctx, uid); e == nil {
		quota = settings.EffectiveWorkspaceQuota(u.WorkspaceQuota)
	}
	return out, owned < quota
}

// patch mengirim satu atau lebih fragment gomponents ke browser via Datastar SSE
// (§4.4 jalur 2). Elemen di-morph berdasarkan atribut id-nya (mode default outer).
func patch(w http.ResponseWriter, r *http.Request, log *slog.Logger, nodes ...g.Node) {
	sse := datastar.NewSSE(w, r) // set header SSE + flush otomatis; TIDAK return error
	for _, n := range nodes {
		var sb strings.Builder
		if err := n.Render(&sb); err != nil {
			log.Error("patch render", "path", r.URL.Path, "err", err)
			return
		}
		if err := sse.PatchElements(sb.String()); err != nil {
			log.Error("patch send", "err", err) // klien putus, dsb.
			return
		}
	}
}
