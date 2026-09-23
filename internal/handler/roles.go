package handler

import (
	"errors"
	"net/http"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// roles.go — AKSI atas peran CRM workspace: buat, sunting matriks izin, hapus.
// Halaman/editor-nya ada di roles_page.go. Ketiga aksi berbagi satu invarian:
// peran is_system (admin) TAK bisa disunting/dihapus, dan setiap perubahan yang
// menyentuh matriks me-reload enforcer tenant agar berlaku seketika tanpa restart.
// RoleUpdate dipindah ke roles_update.go — dipisah krn ambang File Health
// yang sama.

// RoleCreate — POST /w/{workspace}/roles. Buat peran CRM baru (matriks kosong;
// izinnya disunting kemudian lewat editor). Nama = identitas mesin (subject
// Casbin), display_name = label layar, data_scope = cakupan F3.
func (h *Handler) RoleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageRoles(ctx) {
		wsRedirect(w, r, "/roles", "forbidden")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if !authz.ValidBusinessRoleName(name) {
		wsRedirect(w, r, "/roles", "role_name")
		return
	}
	display := strings.TrimSpace(r.FormValue("display_name"))
	if display == "" || len(display) > roleDisplayMax {
		wsRedirect(w, r, "/roles", "role_display")
		return
	}
	scope := r.FormValue("data_scope")
	if !authz.ValidDataScope(scope) {
		wsRedirect(w, r, "/roles", "role_scope")
		return
	}
	kind := r.FormValue("kind")
	if !authz.ValidKind(kind) {
		wsRedirect(w, r, "/roles", "kind")
		return
	}
	// Deskripsi opsional (boleh kosong) — hanya panjangnya dibatasi. TrimSpace agar
	// spasi belaka = kosong, bukan "deskripsi berisi spasi".
	description := strings.TrimSpace(r.FormValue("description"))
	if len(description) > roleDescMax {
		wsRedirect(w, r, "/roles", "role_desc")
		return
	}
	tenantID := session.TenantID(ctx)
	actorID := session.UserID(ctx)
	// CreateBusinessRole = ON CONFLICT DO NOTHING RETURNING: nama sudah dipakai →
	// zero rows → pgx.ErrNoRows. Itu bukan galat internal, tapi "nama bentrok".
	if _, err := h.q(ctx).CreateBusinessRole(ctx, db.CreateBusinessRoleParams{
		TenantID: tenantID, Name: name, DisplayName: display, Description: description,
		DataScope: scope, Kind: kind, IsSystem: false, CreatedBy: &actorID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			wsRedirect(w, r, "/roles", "role_exists")
			return
		}
		h.Log.Error("roles: create", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	// Tak ada reload enforcer: peran baru bermatriks KOSONG → nol baris policy,
	// tak ada yang berubah di enforcer. Ia mulai deny-default sampai disunting.
	h.auditWorkspace(ctx, actorID, "crm.role.create", tenantID, map[string]string{"role": name})
	wsRedirectOK(w, r, "/roles", "created")
}
