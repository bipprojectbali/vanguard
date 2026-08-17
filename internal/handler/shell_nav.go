package handler

import (
	"context"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// shell_nav.go — pemilihan menu & badge untuk halaman LINTAS-PANEL. Dipisah dari
// render.go (yang sudah di ambang file-health) — bukan sekadar demi ukuran:
// isinya memang concern berbeda, yaitu halaman yang tak dimiliki panel manapun.

// navFor memilih menu sidebar untuk halaman lintas-panel (mis. /notifications).
// Semua pemanggil renderShell lain terikat satu panel dan mengoper nav-nya
// langsung; halaman ini tidak, jadi menu ditentukan dari ROLE — pola precompute
// yang sama dengan quickLinksFor (view tak boleh memanggil authz sendiri).
func navFor(ctx context.Context) []ui.NavItem {
	if role := session.Role(ctx); isPlatformRole(role) || session.IsRoot(ctx) {
		return devNav(ctx)
	}
	// Role tenant → menu ruang kerja AKTIF (dari session: halaman ini tak punya
	// slug di path-nya sendiri). Item "Pengaturan" mengikuti izin yang sama
	// dengan halaman ber-slug agar menu tak berubah-ubah antar halaman.
	return workspaceNav(session.TenantSlug(ctx), canManageMembers(ctx), canEditWorkspace(ctx), canViewAccounts(ctx), canViewContacts(ctx), canViewLeads(ctx), canViewDeals(ctx), canViewSalesActivity(ctx), canViewAllActivities(ctx), canViewPlans(ctx), canViewSubscriptions(ctx), canViewSLAPolicies(ctx), canViewPlaybooks(ctx), canViewKBArticles(ctx), canManageRoles(ctx), canViewTickets(ctx), canViewHealthScore(ctx), canViewSuccessPlans(ctx), canViewEngagements(ctx), canViewCSRenewals(ctx))
}

// brandFor = sub-label panel, dipasangkan dengan navFor agar konteks di sidebar
// cocok dengan menu yang ditampilkan.
func brandFor(ctx context.Context) string {
	if role := session.Role(ctx); isPlatformRole(role) || session.IsRoot(ctx) {
		return devBrand()
	}
	return session.TenantName(ctx)
}

// panelFor menentukan identitas visual shell dari path yang sedang dibuka
// (chip + aksen warna, lihat ui/panelkind.go). Diturunkan dari currentPath —
// BUKAN parameter baru renderShell — agar 9 pemanggilnya tak perlu diubah dan
// mustahil lupa mengoper: satu path selalu memetakan ke satu panel.
//
// /notifications lintas-panel: shell-nya mengikuti navFor (role), jadi
// pemetaannya diserahkan ke panelForRole agar chip selalu cocok dengan menu
// yang tampil — bukan menampilkan menu /dev berlabel RUANG KERJA.
// Path /w/{slug} tak lagi menentukan panel sendirian: sejak 0004 satu alamat
// melayani semua role, jadi chip diturunkan dari ROLE di workspace itu
// (panelForRole) — sumber yang SAMA dengan navFor, supaya chip tak pernah
// bertentangan dengan menu yang tampil.
func panelFor(currentPath string) ui.Panel {
	switch {
	case strings.HasPrefix(currentPath, "/dev"):
		return ui.PanelDev
	default:
		return ui.PanelNone
	}
}

// panelOf = titik masuk tunggal yang dipakai renderShell: path yang dikenal
// menentukan panel; halaman lintas-panel (path tak dikenal, mis. /notifications)
// jatuh ke role — sumber yang SAMA dengan navFor, sehingga chip tak pernah
// bertentangan dengan menu yang tampil.
func (h *Handler) panelOf(ctx context.Context, currentPath string) ui.Panel {
	if p := panelFor(currentPath); p != ui.PanelNone {
		return p
	}
	return panelForRole(ctx)
}

// panelForRole = pasangan navFor/brandFor. Dipakai halaman lintas-panel DAN
// halaman ruang kerja: di /w/{slug} chip menandakan otoritas user DI SANA
// (ADMIN bila owner/admin, RUANG KERJA bila member) — informasi yang justru
// lebih berguna setelah peleburan, karena alamatnya kini sama untuk semua.
func panelForRole(ctx context.Context) ui.Panel {
	role := session.Role(ctx)
	switch {
	case isPlatformRole(role) || session.IsRoot(ctx):
		return ui.PanelDev
	case role == authz.RoleNameOwner || role == authz.RoleNameAdmin:
		return ui.PanelAdmin
	default:
		return ui.PanelUser
	}
}

// notifBadge menghitung yang perlu perhatian: peristiwa belum dibaca + undangan
// pending. Undangan sengaja ikut walau tak pernah ter-auto-read — ia tugas yang
// hanya hilang bila benar-benar ditindak.
//
// FAIL-SOFT 3 lapis (pola workspaceOptions): tanpa uid → nil; gagal query → log
// + nil. Badge hilang jauh lebih baik daripada halaman gagal render.
func (h *Handler) notifBadge(ctx context.Context) *ui.NavBadge {
	uid := session.UserID(ctx)
	if uid == 0 {
		return nil
	}
	item := ui.NavItem{
		Label: "Notifikasi", Href: "/notifications",
		Icon: lucide.Bell(html.Class("size-4")),
	}
	var total int64
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		n, err := q.CountUnreadNotifications(ctx, uid)
		if err != nil {
			return err
		}
		inv, err := q.CountPendingInvitesByEmail(ctx, normalizeEmail(session.Email(ctx)))
		if err != nil {
			return err
		}
		total = n + inv
		return nil
	}); err != nil {
		h.Log.Error("shell: notif badge", "err", err)
		return &ui.NavBadge{Item: item} // menu tetap ada, hanya angkanya absen
	}
	return &ui.NavBadge{Item: item, Count: total}
}
