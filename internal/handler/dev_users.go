package handler

import (
	"math"
	"net/http"
	"strconv"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/dev"

	"github.com/jackc/pgx/v5/pgtype"
)

// dev_users.go — panel platform /dev/users: siapa boleh mengubah apa atas USER
// (identitas global, lintas-workspace). Tampilannya di dev_users_view.go,
// balasan SSE-nya di dev_users_flash.go.

// firstPageCursor mengembalikan cursor keyset untuk halaman pertama:
// (created_at, id) = maksimum, sehingga semua baris lolos syarat < cursor.
func firstPageCursor() (pgtype.Timestamptz, int64) {
	return pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity}, math.MaxInt64
}

// DevUsersList — GET /dev/users. Daftar user (keyset) untuk panel developer.
func (h *Handler) DevUsersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cursorAt, cursorID := pageCursor(r)
	// Ambil SATU lebih banyak dari yang ditampilkan: kelebihan itulah yang
	// menjawab "masih ada lagi?" tanpa COUNT terpisah — dan tanpanya tombol
	// "Berikutnya" akan tetap muncul di halaman terakhir lalu berujung kosong.
	users, err := h.q(ctx).ListUsers(ctx, db.ListUsersParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("dev users: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	users, next := splitPage(users, func(u db.User) (pgtype.Timestamptz, int64) {
		return u.CreatedAt, u.ID
	})
	// Aktor untuk menentukan kontrol mana yang boleh dirender (precompute).
	actorRole := authz.ParseRole(session.Role(ctx))
	canManageSuper := session.IsRoot(ctx) || actorRole >= authz.RoleSuperAdmin
	byUser := h.membershipsByUser(ctx, users) // satu query batch (anti N+1)
	h.renderShell(w, r, "Users", devBrand(), "/dev/users", devNav(ctx),
		dev.UsersPage(dev.UsersView{
			Rows:           toUserRows(users, byUser),
			Roles:          authz.AssignableRoles(appmode.IsSingle()),
			CanManageSuper: canManageSuper,
			NextCursor:     next,
			HasPrev:        r.URL.Query().Get("after") != "",
		}))
}

// DevUserSetRole — POST /dev/users/{id}/role. Ubah role (di-guard + audit).
func (h *Handler) DevUserSetRole(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	newRole := r.FormValue("role")
	if !authz.ValidRoleName(newRole) {
		h.devFlash(w, r, false, "Role tidak valid")
		return
	}
	// Role kini PER-WORKSPACE → butuh tenant mana. Form mengirimnya (panel /dev
	// menampilkan satu baris per membership).
	tenantID, err := strconv.ParseInt(r.FormValue("tenant"), 10, 64)
	if err != nil || tenantID == 0 {
		h.devFlash(w, r, false, "Workspace tidak valid")
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID, tenantID)
	if err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := authz.GuardSetRole(actor, target, authz.ParseRole(newRole)); err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	// Cegah workspace YATIM: menurunkan owner terakhir → ditolak.
	if target.Role == authz.RoleOwner && newRole != authz.RoleNameOwner {
		if n, e := h.q(r.Context()).CountTenantOwners(r.Context(), tenantID); e == nil && n <= 1 {
			h.devFlash(w, r, false, "Tak bisa menurunkan owner terakhir workspace")
			return
		}
	}
	if err := h.q(r.Context()).UpdateMemberRole(r.Context(), db.UpdateMemberRoleParams{
		UserID: targetID, TenantID: tenantID, Role: newRole,
	}); err != nil {
		h.Log.Error("dev users: update role", "err", err)
		h.devFlash(w, r, false, "Gagal menyimpan perubahan")
		return
	}
	h.audit(r.Context(), actor.ID, "member.role.update", targetID, map[string]string{"to": newRole})
	// Kabar untuk yang bersangkutan — SAMA seperti jalur workspace (MemberSetRole).
	// Efeknya identik: wewenangnya berubah. Yang tak boleh terjadi adalah orang
	// mengetahui perubahan itu atau tidak, tergantung pintu mana yang kebetulan
	// dipakai pengelolanya. tenantID = workspace TARGET (bukan workspace aktor,
	// yang di panel lintas-workspace ini sering bukan workspace yang sama).
	h.notify(r.Context(), targetID, tenantID, "member.role.changed", notifPayload{Role: newRole})
	h.devRowUpdated(w, r, targetID, "Role diubah ke "+newRole)
}
