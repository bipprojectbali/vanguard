package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	g "maragu.dev/gomponents"
)

// canEditWorkspace melaporkan apakah aktor boleh mengubah PENGATURAN
// workspace/aplikasi (format kode, dst). Root env selalu boleh (super_admin
// efektif).
//
//	multi  → owner (pemilik) atau platform. Admin/member hanya lihat.
//	single → admin JUGA boleh (0006 §7).
//
// Kenapa berbeda — dan alasannya BUKAN lagi "di mode single tak ada owner".
// Sejak 0007 rumah aplikasi punya owner: super_admin. Yang tersisa adalah alasan
// yang sebenarnya sejak awal: admin di mode single adalah PEMBANTU super_admin,
// dan hal sepele seperti menyesuaikan format kode tak boleh menuntut seseorang
// menyunting .env lalu me-restart. Identitas workspace (nama) & siklus hidupnya
// (arsip/hapus) kini wewenang platform di /dev (BL-53) — bukan di sini.
func canEditWorkspace(ctx context.Context) bool {
	if session.IsRoot(ctx) {
		return true
	}
	role := session.Role(ctx)
	if isPlatformRole(role) || role == authz.RoleNameOwner {
		return true
	}
	return appmode.IsSingle() && role == authz.RoleNameAdmin
}

// WorkspaceHome — GET /w/{workspace}/. Beranda ruang kerja, terbuka untuk SEMUA
// anggota (member/admin/owner). Menggantikan /admin & /user yang dulu terpisah:
// keduanya membedakan role, bukan resource (0004).
func (h *Handler) WorkspaceHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body := make([]g.Node, 0, 2)
	// Banner opt-in CRM: pengelola (owner/admin) yang BELUM punya peran CRM
	// terkunci dari seluruh menu CRM (CanBusiness fail-closed). Tawarkan jalan
	// keluar satu-klik jadi Admin CRM — menyasar deadlock owner baru (business_role
	// NULL sejak lahir). Gerbang = sumbu tenant (canManageMembers), bukan CRM.
	if canManageMembers(ctx) && !IsReadOnly(ctx) && session.BusinessRole(ctx) == "" {
		self := strconv.FormatInt(session.UserID(ctx), 10)
		body = append(body, panel.CRMOnboard(wsPathOf(ctx, "/members/"+self+"/role"), authz.BusinessRoleAdmin))
	}
	body = append(body, h.dashboardHome(ctx))
	h.renderWorkspaceShell(w, r, "Beranda", "", g.Group(body))
}
