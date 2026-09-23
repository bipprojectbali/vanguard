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

// invite_service.go — logika undangan (validasi token, terima, auto-accept).
// Dipisah dari invite.go (handler HTTP) agar tiap file di bawah batas 150 baris.

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

// acceptInvitesByEmail menerima SEMUA undangan pending yang cocok email user —
// jalur UTAMA BL-170 (auto-join saat login/register/OAuth, bukan lagi klik
// tautan). Loop: lazimnya satu undangan, tapi tak ada yang mencegah admin di
// beberapa workspace mengundang email yang sama sebelum orangnya mendaftar.
// Pre-identity-agnostic (WithSuper, invites TANPA RLS) — aman dipanggil
// sebelum atau sesudah tenant aktif diketahui. Fail-soft PER-UNDANGAN: satu
// yang gagal (log, lanjut) tak boleh menggagalkan login atau menghentikan sisa
// undangan lain diproses. TAK memindahkan tenant aktif session yang SUDAH ada
// (beda dari acceptInvite/token — bergabung pasif via email tak boleh diam-diam
// memindahkan workspace yang sedang dilihat user existing). TAPI bila session
// BELUM punya tenant aktif sama sekali (user baru, startIdentity berjalan
// sebelum invite ini diproses sehingga ListMembershipsByUser-nya masih kosong),
// aktifkan workspace pertama yang baru bergabung — tanpa ini homeFor() mengira
// user tetap "tanpa workspace" dan mengarahkannya ke /workspace/new, yang di
// mode single ditutup RequireMulti → 404 senyap walau membership-nya sudah sah.
func (h *Handler) acceptInvitesByEmail(ctx context.Context, email string, uid int64) {
	email = normalizeEmail(email)
	if email == "" {
		return
	}
	var invs []db.ListPendingInvitesByEmailRow
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		rows, e := q.ListPendingInvitesByEmail(ctx, email)
		invs = rows
		return e
	}); err != nil {
		h.Log.Warn("invite: auto-accept by email gagal load", "err", err)
		return
	}
	var firstTenantID int64
	var firstName, firstSlug string
	for _, inv := range invs {
		tenantID := inv.TenantID
		var slug string
		if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
			if err := applyInviteMembership(ctx, q, uid, tenantID, inv.Role, inv.BusinessRole, inv.Kind); err != nil {
				return err
			}
			if err := q.AcceptInvite(ctx, inv.Token); err != nil {
				return err
			}
			t, err := q.GetTenant(ctx, tenantID)
			if err != nil {
				return err
			}
			slug = t.Slug
			return nil
		}); err != nil {
			h.Log.Warn("invite: auto-accept by email gagal", "tenant_id", tenantID, "err", err)
			continue
		}
		h.auditWorkspace(ctx, uid, "invite.accept", tenantID, map[string]string{"role": inv.Role})
		if firstTenantID == 0 {
			firstTenantID, firstName, firstSlug = tenantID, inv.TenantName, slug
		}
	}
	if firstTenantID != 0 && session.TenantID(ctx) == 0 {
		session.SetActiveTenant(ctx, firstTenantID, firstName, firstSlug)
	}
}

// acceptPendingInvite menerima undangan yang tersimpan di session — dipanggil
// SETELAH register/login sukses. Skenario: orang membuka tautan undangan sebelum
// punya akun → token disimpan → begitu akun jadi, ia langsung masuk workspace
// pengundang (tak perlu klik tautan dua kali). FAIL-SOFT: undangan kedaluwarsa
// atau invalid TIDAK menggagalkan login — user tetap masuk workspace sendiri.
func (h *Handler) acceptPendingInvite(r *http.Request) {
	ctx := r.Context()
	token := session.PendingInvite(ctx)
	if token == "" {
		return
	}
	session.ClearPendingInvite(ctx) // one-shot: jangan coba lagi tiap login
	if err := h.acceptInvite(ctx, token, session.UserID(ctx)); err != nil {
		h.Log.Warn("invite: auto-accept gagal", "err", err)
	}
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
