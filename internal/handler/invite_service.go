package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// invite_service.go — logika undangan (validasi token, terima). Dipisah dari
// invite.go (handler HTTP) agar tiap file di bawah batas 150 baris.
// acceptInvitesByEmail & acceptPendingInvite (dipanggil dari login/register,
// bukan dari invite.go) ada di invite_autojoin.go.

// canManageMembers melaporkan apakah aktor boleh mengelola anggota workspace:
// owner/admin tenant, atau platform (super_admin/staff yang membantu). Tinggal
// di sini karena dipakai BERSAMA oleh members.go & invite.go — mengundang dan
// mengelola anggota adalah izin yang sama.
func canManageMembers(ctx context.Context) bool {
	if session.IsRoot(ctx) {
		return true
	}
	role := session.Role(ctx)
	return role == authz.RoleNameOwner || role == authz.RoleNameAdmin || isPlatformRole(role)
}

var (
	errInviteNotFound = errors.New("undangan tidak ditemukan")
	errInviteExpired  = errors.New("undangan kedaluwarsa")
	errInviteUsed     = errors.New("undangan sudah dipakai")
)

// loadInvite mengambil + memvalidasi undangan. WithSuper: jalur publik, penerima
// belum tentu anggota workspace mana pun (tak ada scope RLS yang relevan).
func (h *Handler) loadInvite(ctx context.Context, token string) (db.GetInviteByTokenRow, error) {
	var inv db.GetInviteByTokenRow
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		row, e := q.GetInviteByToken(ctx, token)
		inv = row
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return inv, errInviteNotFound
		}
		return inv, err
	}
	if inv.AcceptedAt.Valid {
		return inv, errInviteUsed
	}
	if inv.ExpiresAt.Valid && inv.ExpiresAt.Time.Before(time.Now()) {
		return inv, errInviteExpired
	}
	return inv, nil
}

// applyInviteMembership membuat ATAU meng-upgrade membership dari satu undangan
// — dipakai bersama jalur token (acceptInvite) & jalur auto-join-by-email
// (acceptInvitesByEmail, BL-170). CreateMembership ON CONFLICT DO NOTHING: bila
// user SUDAH anggota (mis. join organik lebih dulu), pgx.ErrNoRows di sini
// BUKAN kegagalan — lanjut ke GetMembership utk menilai upgrade. Role invite
// HANYA menaikkan (authz.ParseRole ordinal), tak pernah menurunkan role
// existing. business_role+kind SELALU diterapkan dari undangan (bukan hanya
// saat membership baru) — admin yang mengundang ulang dgn field baru
// mengharapkannya berlaku juga pada anggota yang kebetulan sudah bergabung
// lewat jalur lain sebelum undangan ini diproses.
func applyInviteMembership(ctx context.Context, q *db.Queries, uid, tenantID int64, role string, businessRole *string, kind string) error {
	if _, err := q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: uid, TenantID: tenantID, Role: role,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	m, err := q.GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: tenantID})
	if err != nil {
		return err
	}
	if authz.ParseRole(role) > authz.ParseRole(m.Role) {
		if err := q.UpdateMemberRole(ctx, db.UpdateMemberRoleParams{
			UserID: uid, TenantID: tenantID, Role: role,
		}); err != nil {
			return err
		}
	}
	if err := q.UpdateMemberBusinessRole(ctx, db.UpdateMemberBusinessRoleParams{
		UserID: uid, TenantID: tenantID, BusinessRole: businessRole,
	}); err != nil {
		return err
	}
	return q.UpdateMembershipKind(ctx, db.UpdateMembershipKindParams{
		UserID: uid, TenantID: tenantID, Kind: kind,
	})
}

// acceptInvite membuat membership + menandai undangan terpakai dalam SATU tx
// (atomik). AcceptInvite ber-guard accepted_at IS NULL → klik ganda tak menghasilkan
// dua membership; UNIQUE(user,tenant) jadi jaring kedua.
func (h *Handler) acceptInvite(ctx context.Context, token string, uid int64) error {
	inv, err := h.loadInvite(ctx, token)
	if err != nil {
		return err
	}
	name := inv.TenantName
	tenantID := inv.TenantID
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		if err := applyInviteMembership(ctx, q, uid, tenantID, inv.Role, inv.BusinessRole, inv.Kind); err != nil {
			return err
		}
		return q.AcceptInvite(ctx, token)
	}); err != nil {
		return err
	}
	session.SetActiveTenant(ctx, tenantID, name, inv.TenantSlug)
	session.ClearPendingInvite(ctx)
	h.auditWorkspace(ctx, uid, "invite.accept", tenantID, map[string]string{"role": inv.Role})
	return nil
}

// inviteErrText memetakan error undangan ke pesan untuk user.
func inviteErrText(err error) string {
	switch {
	case errors.Is(err, errInviteNotFound):
		return "Undangan tidak ditemukan atau tautannya salah."
	case errors.Is(err, errInviteExpired):
		return "Undangan sudah kedaluwarsa. Minta undangan baru."
	case errors.Is(err, errInviteUsed):
		return "Undangan ini sudah dipakai."
	default:
		return "Terjadi kesalahan memproses undangan."
	}
}

// inviteLink merakit URL undangan absolut dari request (skema + host). Email
// BELUM dikirim (task terbuka) — link ditampilkan di UI untuk disalin manual.
func inviteLink(r *http.Request, token string) string {
	return baseFrom(r) + "/invite/" + token
}
