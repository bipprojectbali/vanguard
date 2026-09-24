package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// invite_autojoin.go — dipisah dari invite_service.go agar tiap file di bawah
// batas 150 baris. Isinya DUA fungsi yang dipanggil dari alur login/register
// (bukan dari handler invite.go langsung): acceptInvitesByEmail (BL-170,
// auto-join saat login/register/OAuth) & acceptPendingInvite (undangan
// tersimpan di session dari sebelum akun ada).

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
