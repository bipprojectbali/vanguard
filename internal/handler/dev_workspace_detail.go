package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/dev"
)

// dev_workspace_detail.go — aksi PLATFORM atas identitas SATU workspace (BL-53):
// ganti nama + siklus hidup (arsip/pulihkan/hapus). Dulu wewenang owner di
// /w/{slug}/settings; dipindah ke ruang developer agar jadi keputusan platform,
// sejalan dengan suspend/restore yang sudah di sini. Lintas-tenant BY ID: scope
// /dev = WithSuper (bypass RLS), jadi h.q(ctx) beroperasi di workspace mana pun
// tanpa keanggotaan — sama seperti DevWorkspaceSuspend/Restore.

// DevWorkspaceDetail — GET /dev/workspaces/{id}. Halaman aksi satu workspace.
// Termasuk yang suspended/archived/terhapus: platform sengaja tembus gerbang
// siklus hidup (0005), dan aksi pemulihan mustahil bila barisnya tak tampak.
func (h *Handler) DevWorkspaceDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	t, err := h.q(ctx).GetTenant(ctx, id)
	if err != nil {
		http.NotFound(w, r) // id tak ada → 404, bukan 500
		return
	}
	h.renderShell(w, r, "Workspace", devBrand(), "/dev/workspaces", devNav(ctx),
		dev.WorkspaceDetail(dev.WorkspaceDetailView{
			ID:      t.ID,
			Name:    t.Name,
			Slug:    t.Slug,
			Status:  t.Status,
			Primary: t.IsPrimary,
			Deleted: t.DeletedAt.Valid,
			ErrMsg:  wsErrMsg(r.URL.Query().Get("err")),
		}))
}

// DevWorkspaceRename — POST /dev/workspaces/{id}/rename. Ganti nama tampilan;
// slug immutable (mengubahnya mematikan tautan tersimpan). TIDAK menyentuh
// session: operator platform mengubah workspace LAIN, jadi brand sidebar-nya
// sendiri tak boleh ikut berubah. Audit workspace.rename (target = workspace itu).
func (h *Handler) DevWorkspaceRename(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" || len(name) > maxWorkspaceNameLen {
		h.devWorkspaceRedirect(w, r, id, "name")
		return
	}
	if err := h.q(ctx).UpdateTenant(ctx, db.UpdateTenantParams{ID: id, Name: name}); err != nil {
		h.Log.Error("dev workspace: rename", "tenant_id", id, "err", err)
		h.devWorkspaceRedirect(w, r, id, "failed")
		return
	}
	h.auditWorkspace(ctx, session.UserID(ctx), "workspace.rename", id, nil)
	h.devWorkspaceRedirect(w, r, id, "")
}

// DevWorkspaceArchive — POST /dev/workspaces/{id}/archive. Jadikan hanya-baca.
// Rumah aplikasi ditolak: mengarsipkannya membuat seluruh aplikasi read-only.
// Dijaga di sini (pesan ?err=primary) DAN di SQL (`AND NOT is_primary`).
func (h *Handler) DevWorkspaceArchive(w http.ResponseWriter, r *http.Request) {
	h.devWorkspaceLifecycle(w, r, "workspace.archive", func(id int64) error {
		return h.q(r.Context()).ArchiveTenant(r.Context(), id)
	})
}

// DevWorkspaceUnarchive — POST /dev/workspaces/{id}/unarchive. Kebalikan arsip.
func (h *Handler) DevWorkspaceUnarchive(w http.ResponseWriter, r *http.Request) {
	h.devWorkspaceLifecycle(w, r, "workspace.unarchive", func(id int64) error {
		return h.q(r.Context()).UnarchiveTenant(r.Context(), id)
	})
}

// DevWorkspaceDelete — POST /dev/workspaces/{id}/delete. SOFT-delete (masa
// tenggang 30 hari; pulihkan lewat tombol Pulihkan). Rumah aplikasi ditolak,
// dijaga di handler DAN SQL.
func (h *Handler) DevWorkspaceDelete(w http.ResponseWriter, r *http.Request) {
	h.devWorkspaceLifecycle(w, r, "workspace.delete", func(id int64) error {
		return h.q(r.Context()).SoftDeleteTenant(r.Context(), id)
	})
}

// devWorkspaceLifecycle = kerangka bersama arsip/unarsip/hapus: parse id → tolak
// rumah aplikasi (untuk aksi yang memang menolaknya) → jalankan → audit →
// kembali ke detail. Rumah aplikasi diperiksa lebih dulu agar penolakannya
// terbaca (?err=primary) alih-alih no-op senyap dari guard SQL.
func (h *Handler) devWorkspaceLifecycle(w http.ResponseWriter, r *http.Request, action string, fn func(int64) error) {
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	// Arsip & hapus menolak rumah aplikasi; unarsip tak menyentuhnya. Cek is_primary
	// hanya untuk aksi yang menolaknya agar unarchive tak terhalang tak perlu.
	if action == "workspace.archive" || action == "workspace.delete" {
		if t, err := h.q(ctx).GetTenant(ctx, id); err == nil && t.IsPrimary {
			h.devWorkspaceRedirect(w, r, id, "primary")
			return
		}
	}
	if err := fn(id); err != nil {
		h.Log.Error("dev workspace: "+action, "tenant_id", id, "err", err)
		h.devWorkspaceRedirect(w, r, id, "failed")
		return
	}
	h.auditWorkspace(ctx, session.UserID(ctx), action, id, nil)
	h.devWorkspaceRedirect(w, r, id, "")
}

// devWorkspaceRedirect = PRG kembali ke halaman detail (bukan daftar): operator
// tetap menatap workspace yang barusan diubah dan melihat hasilnya seketika.
// code kosong → tanpa ?err.
func (h *Handler) devWorkspaceRedirect(w http.ResponseWriter, r *http.Request, id int64, code string) {
	dest := "/dev/workspaces/" + strconv.FormatInt(id, 10)
	if code != "" {
		dest += "?err=" + code
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}
