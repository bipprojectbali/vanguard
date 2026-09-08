package handler

import (
	"context"

	"go_starter/internal/session"
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// pending_member.go — gerbang onboarding (Opsi A, BL-105).
//
// Anggota baru yang masuk ruang kerja TANPA peran CRM (business_role kosong)
// belum boleh memakai apa pun: setiap menu CRM fail-closed (CanBusiness). Alih-
// alih membiarkannya di Beranda dengan menu mati tanpa penjelasan, ia diarahkan
// ke halaman "menunggu approval admin" (panel.PendingApproval) dan menu CRM
// disembunyikan sampai admin menetapkan peran — RefreshIdentity menyegarkan
// business_role per-request, jadi begitu peran diberi, menu langsung muncul.
//
// Pengelola (owner/admin/platform) SENGAJA dikecualikan: mereka bisa menetapkan
// peran sendiri lewat banner CRMOnboard (workspace.go) — bukan "menunggu".

// pendingHolding = keputusan murni: anggota non-pengelola tanpa peran CRM.
// Dipisah dari context agar bisa diuji tanpa merangkai session.
func pendingHolding(canManage bool, businessRole string) bool {
	return !canManage && businessRole == ""
}

// isPendingMember menilai keputusan di atas dari context request.
func isPendingMember(ctx context.Context) bool {
	return pendingHolding(canManageMembers(ctx), session.BusinessRole(ctx))
}

// pendingMemberNav = menu minimal untuk anggota yang menunggu peran: HANYA
// Dashboard (yang menampilkan halaman "menunggu approval admin"). Menu CRM
// disembunyikan — bukan sekadar disabled — agar tak ada pintu hantu.
func pendingMemberNav(slug string) []ui.NavItem {
	return []ui.NavItem{
		{Label: "Dashboard", Href: wsPath(slug, ""), Icon: lucide.House(html.Class("size-4"))},
	}
}

// workspaceNavCtx merakit menu ruang kerja dari izin di ctx untuk slug tertentu.
// Titik masuk TUNGGAL (dipakai renderWorkspaceShell & navFor) agar gerbang
// pending berlaku seragam di semua halaman — termasuk lintas-panel
// (/notifications). Sebelumnya daftar 21-argumen ini diduplikasi di dua tempat.
func workspaceNavCtx(ctx context.Context, slug string) []ui.NavItem {
	if isPendingMember(ctx) {
		return pendingMemberNav(slug)
	}
	return workspaceNav(slug, canManageMembers(ctx), canEditWorkspace(ctx), canViewAccounts(ctx), canViewContacts(ctx), canViewLeads(ctx), canViewDeals(ctx), canViewSalesActivity(ctx), canViewAllActivities(ctx), canViewPlans(ctx), canViewSubscriptions(ctx), canViewSLAPolicies(ctx), canViewPlaybooks(ctx), canViewKBArticles(ctx), canManageRoles(ctx), canViewTickets(ctx), canViewHealthScore(ctx), canViewSuccessPlans(ctx), canViewEngagements(ctx), canViewCSRenewals(ctx), canViewReports(ctx), canReadCSJourney(ctx))
}
