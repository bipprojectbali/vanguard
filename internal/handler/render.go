package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/settings"
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
	html "maragu.dev/gomponents/html"
)

// cssPath adalah path app.css dengan cache-bust hash, di-set saat startup.
var cssPath = "/static/app.css"

// SetCSSPath menetapkan path CSS ber-hash (dipanggil dari main saat startup).
func SetCSSPath(p string) { cssPath = p }

// appName = nama aplikasi untuk brand & judul halaman. Di-inject dari config
// (APP_NAME) via SetAppName, mengikuti pola setter global lainnya di file ini.
//
// Sebelumnya "go_starter" di-hardcode di tujuh tempat. Untuk sebuah TEMPLATE
// yang memang dimaksudkan di-clone, itu bentuk hardcode yang paling mahal:
// bukan cuma melanggar Rule 15, tapi membuat setiap project turunan memampangkan
// nama template-nya di sidebar sampai ada yang menyisirnya satu per satu.
//
// Default "App" menyamai config.getEnv("APP_NAME", "App") — dua tempat yang
// menyimpan default berbeda akan tampak sebagai nama yang berubah-ubah
// tergantung jalur mana yang menyetelnya.
var appName = "App"

// SetAppName menetapkan nama aplikasi (dipanggil dari main saat startup).
func SetAppName(n string) {
	if n != "" {
		appName = n
	}
}

// AppName mengembalikan nama aplikasi yang aktif.
func AppName() string { return appName }

// devBrand = label brand panel platform, mis. "Acme /dev".
func devBrand() string { return appName + " /dev" }

// devMode menandai environment non-production. Menentukan apakah form login
// password ditampilkan (password auth = dev-only; produksi hanya Google).
var devMode bool

// SetDevMode menetapkan flag dev (dipanggil dari main: !cfg.IsProduction()).
func SetDevMode(v bool) { devMode = v }

// isSuperAdminEmail memeriksa apakah email = super-admin env (root immutable).
// Di-inject dari main (config.IsSuperAdminEmail) agar handler tak import config.
var isSuperAdminEmail = func(string) bool { return false }

// SetSuperAdminChecker menyuntik fungsi cek super-admin env dari config.
func SetSuperAdminChecker(fn func(string) bool) { isSuperAdminEmail = fn }

// appTZ = zona waktu untuk agregasi tampilan panel logs (tampilan jam lokal).
// Default UTC bila belum di-set; di-inject dari main via SetAppTimezone.
var appTZ = time.UTC

// SetAppTimezone menetapkan zona waktu aplikasi (dipanggil dari main saat startup).
func SetAppTimezone(loc *time.Location) {
	if loc != nil {
		appTZ = loc
	}
}

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

// renderWorkspaceShell mengirim halaman di dalam ruang kerja. Merakit sendiri
// slug, menu, dan currentPath dari request — pemanggil cukup menyebut sub-path
// (mis. "/members"). Tanpa ini tiap handler workspace mengulang perakitan yang
// sama, dan satu yang keliru menghasilkan menu menunjuk workspace lain.
func (h *Handler) renderWorkspaceShell(w http.ResponseWriter, r *http.Request, title, sub string, body g.Node) {
	ctx := r.Context()
	slug := slugFromRequest(r)
	nav := workspaceNav(slug, canManageMembers(ctx), canEditWorkspace(ctx), canViewAccounts(ctx), canViewContacts(ctx), canViewLeads(ctx), canViewDeals(ctx), canViewSalesActivity(ctx), canViewPlans(ctx), canManageRoles(ctx))
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
