package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui"
	"go_starter/internal/ui/pages/panel"

	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// workspaceName adalah bentuk signal form ganti nama workspace.
type workspaceName struct {
	Name string `json:"name"`
}

// canEditWorkspace melaporkan apakah aktor boleh mengubah PENGATURAN
// workspace/aplikasi (nama, dst). Root env selalu boleh (super_admin efektif).
//
//	multi  → owner (pemilik) atau platform. Admin/member hanya lihat.
//	single → admin JUGA boleh (0006 §7).
//
// Kenapa berbeda — dan alasannya BUKAN lagi "di mode single tak ada owner".
// Sejak 0007 rumah aplikasi punya owner: super_admin. Yang tersisa adalah alasan
// yang sebenarnya sejak awal: admin di mode single adalah PEMBANTU super_admin,
// dan hal sepele seperti mengganti nama aplikasi tak boleh menuntut seseorang
// menyunting .env lalu me-restart. Yang fundamental tetap dijaga di tempat lain
// (zona bahaya ditolak untuk workspace primer, kuota & mode di-gate
// platform:settings) — bukan di sini.
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
	body = append(body, panel.Placeholder("Beranda", "Selamat datang di "+session.TenantName(ctx)+"."))
	h.renderWorkspaceShell(w, r, "Beranda", "", g.Group(body))
}

// WorkspaceSettings — GET /w/{workspace}/settings. Form nama workspace (AppShell).
// Admin boleh LIHAT (read-only); hanya owner/platform boleh ubah (canEdit).
func (h *Handler) WorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := h.q(ctx).GetTenant(ctx, session.TenantID(ctx))
	if err != nil {
		h.Log.Error("workspace: get tenant", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	canEdit := canEditWorkspace(ctx)
	h.renderWorkspaceShell(w, r, "Pengaturan", "/settings", panel.Workspace(panel.WorkspaceView{
		Base:   wsPath(slugFromRequest(r), ""),
		Name:   t.Name,
		Slug:   t.Slug,
		Status: t.Status,
		// Ganti nama ditolak saat terarsip (read-only), tapi zona bahaya TETAP
		// tampil — di sanalah tombol "Aktifkan kembali" berada. Menyembunyikannya
		// akan mengunci owner di luar workspace-nya sendiri.
		CanEdit: canEdit && t.Status == TenantActive,
		// Zona bahaya (arsip/hapus) tak berlaku bagi RUMAH APLIKASI (0007):
		// mengarsipkannya membuat seluruh aplikasi read-only, menghapusnya
		// menghapus aplikasinya. Yang menahan bukan lagi mode — route selalu
		// terdaftar sekarang — melainkan sifat workspace-nya, dijaga di handler
		// dan di SQL. Di sini hanya disembunyikan agar tak ada tombol yang
		// menjanjikan pintu buntu.
		CanOwn:   canEdit && !IsPrimaryWorkspace(ctx),
		Archived: t.Status == TenantArchived,
		ErrMsg:   wsErrMsg(r.URL.Query().Get("err")),
	}))
}

// WorkspaceUpdate — POST /w/{workspace}/settings. Ganti nama workspace
// (owner/platform). Guard di handler (bukan cuma route) karena admin BOLEH
// membuka halamannya tapi TAK boleh ganti nama. Slug tak diubah (immutable —
// mengubahnya mematikan setiap tautan tersimpan). Audit + refresh brand sidebar.
func (h *Handler) WorkspaceUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canEditWorkspace(ctx) {
		h.workspaceMsg(w, r, "Hanya owner yang dapat mengubah nama workspace")
		return
	}
	var in workspaceName
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		h.workspaceMsg(w, r, "Nama workspace wajib diisi")
		return
	}
	if len(name) > maxWorkspaceNameLen {
		h.workspaceMsg(w, r, "Nama workspace maksimal 60 karakter")
		return
	}

	tenantID := session.TenantID(ctx)
	if err := h.q(ctx).UpdateTenant(ctx, db.UpdateTenantParams{ID: tenantID, Name: name}); err != nil {
		h.Log.Error("workspace: update", "err", err)
		h.workspaceMsg(w, r, "Gagal menyimpan perubahan")
		return
	}
	// Segarkan brand sidebar seketika (tanpa tunggu request berikutnya).
	session.SetTenantName(ctx, name)
	// Audit (fail-soft). target = tenant sendiri; metadata TANPA nama (bukan PII,
	// tapi konsisten: id saja). actor = user aktif.
	h.auditWorkspace(ctx, session.UserID(ctx), "workspace.rename", tenantID, nil)
	h.workspaceOK(w, r, "Nama workspace disimpan")
}

// workspaceMsg mem-patch alert error (id "workspace-msg") via Datastar SSE.
func (h *Handler) workspaceMsg(w http.ResponseWriter, r *http.Request, msg string) {
	patch(w, r, h.Log, ui.Alert(ui.VariantDestructive, "workspace-msg", g.Text(msg)))
}

// workspaceOK mem-patch alert sukses (id "workspace-msg") via Datastar SSE.
func (h *Handler) workspaceOK(w http.ResponseWriter, r *http.Request, msg string) {
	patch(w, r, h.Log, ui.Alert(ui.VariantDefault, "workspace-msg", g.Text(msg)))
}
