package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/settings"
	"go_starter/internal/ui/pages/panel"
)

// workspace_switch.go — pindah & buat workspace (model membership: satu user bisa
// anggota banyak workspace). Dipisah dari workspace.go (pengaturan) agar tiap file
// tetap di bawah batas handler 150 baris.

// WorkspaceSwitch — POST /workspace/switch. Pindah workspace aktif. WAJIB
// memvalidasi keanggotaan: tenant datang dari form (user-controlled), tanpa cek
// ini user bisa pindah ke workspace orang lain.
func (h *Handler) WorkspaceSwitch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, err := strconv.ParseInt(r.FormValue("tenant"), 10, 64)
	if err != nil || tenantID == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	uid := session.UserID(ctx)
	if _, err := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: tenantID}); err != nil {
		// Bukan anggota → jangan bocorkan keberadaan workspace; diamkan ke home.
		h.Log.Warn("workspace switch: bukan anggota", "user_id", uid, "tenant_id", tenantID)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	t, err := h.q(ctx).GetTenant(ctx, tenantID)
	if err != nil {
		// Membership ada tapi tenant tak terbaca — tanpa slug tak ada tujuan yang
		// bisa dibentuk (0004), jadi jangan pindahkan session ke keadaan setengah.
		h.Log.Error("workspace switch: get tenant", "tenant_id", tenantID, "err", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	session.SetActiveTenant(ctx, tenantID, t.Name, t.Slug)
	// Alamat kini sama untuk semua role (0004) — yang berubah hanya isinya.
	http.Redirect(w, r, wsPath(t.Slug, ""), http.StatusSeeOther)
}

// WorkspaceNewPage — GET /workspace/new. Form buat workspace baru. Juga jadi
// tujuan Scope saat user tak punya workspace sama sekali.
func (h *Handler) WorkspaceNewPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Workspace Baru",
		panel.WorkspaceNew(wsErrMsg(r.URL.Query().Get("err"))))
}

// WorkspaceCreate — POST /workspace/new. Buat workspace + membership owner
// (atomik), cek KUOTA dulu. PRG: gagal → ?err=CODE.
func (h *Handler) WorkspaceCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" || len(name) > maxWorkspaceNameLen {
		http.Redirect(w, r, "/workspace/new?err=name", http.StatusSeeOther)
		return
	}
	uid := session.UserID(ctx)

	// Route ini TANPA middleware Scope (user mungkin belum punya workspace sama
	// sekali → tak ada tenant untuk di-scope). Semua query lewat WithSuper.
	var (
		newID     int64
		newSlug   string
		overQuota bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		// Kuota: berapa workspace yang sudah DIMILIKI (role owner) vs jatah user.
		u, e := q.GetUser(ctx, uid)
		if e != nil {
			return e
		}
		owned, e := q.CountOwnedWorkspaces(ctx, uid)
		if e != nil {
			return e
		}
		// Kuota EFEKTIF: override per-user bila ada, selainnya default global yang
		// bisa diubah operator saat jalan. Dihitung di satu tempat (settings) agar
		// penegakan di sini tak pernah berbeda dari angka yang ditampilkan sidebar.
		if owned >= int64(settings.EffectiveWorkspaceQuota(u.WorkspaceQuota)) {
			overQuota = true
			return nil
		}
		// Tenant + membership dalam SATU tx → atomik (tak ada workspace yatim).
		slug, e := uniqueSlug(ctx, q, slugify(name))
		if e != nil {
			return e
		}
		t, e := q.CreateTenant(ctx, db.CreateTenantParams{Name: name, Slug: slug})
		if e != nil {
			return e
		}
		newID, newSlug = t.ID, t.Slug
		if _, e = q.CreateMembership(ctx, db.CreateMembershipParams{
			UserID: uid, TenantID: t.ID, Role: authz.RoleNameOwner,
		}); e != nil {
			return e
		}
		// Peran CRM bawaan, di tx yang sama dengan tenant+membership → atomik.
		return seedBusinessRoles(ctx, q, t.ID)
	})
	if err != nil {
		h.Log.Error("workspace create", "err", err)
		http.Redirect(w, r, "/workspace/new?err=failed", http.StatusSeeOther)
		return
	}
	if overQuota {
		http.Redirect(w, r, "/workspace/new?err=quota", http.StatusSeeOther)
		return
	}
	// Langsung pindah ke workspace baru (user jadi owner di sana).
	session.SetActiveTenant(ctx, newID, name, newSlug)
	h.auditWorkspace(ctx, uid, "workspace.create", newID, nil)
	http.Redirect(w, r, wsPath(newSlug, ""), http.StatusSeeOther)
}
