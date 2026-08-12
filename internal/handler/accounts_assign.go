package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// accounts_assign.go — penugasan CSM utama & cadangan (AccountAssign) dan
// soft-delete desa (AccountDelete), plus helper anggota workspace yang keduanya
// pakai (parseMemberRef, assignableMembers). Keduanya aksi TULIS ber-gerbang F3.

// AccountAssign — POST /w/{workspace}/accounts/{id}/assign. Menetapkan CSM utama
// & cadangan. Kandidat WAJIB anggota workspace (dicek GetMembership) — memasang
// non-anggota membuat baris yang tak bisa dilihat siapa pun lewat F3.
func (h *Handler) AccountAssign(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	// F3 gate: hanya yang boleh menyentuh baris ini yang boleh menugaskan CSM-nya.
	if _, ok := h.loadOwnedAccount(w, r, id); !ok {
		return
	}

	assigned, code := h.parseMemberRef(ctx, r.FormValue("assigned_csm"))
	if code != "" {
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", code)
		return
	}
	backup, code := h.parseMemberRef(ctx, r.FormValue("backup_csm"))
	if code != "" {
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", code)
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).AssignAccountCSM(ctx, db.AssignAccountCSMParams{
		AssignedCsm: assigned,
		BackupCsm:   backup,
		UpdatedBy:   &uid,
		ID:          id,
	}); err != nil {
		h.Log.Error("accounts: assign", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "account.assign", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/accounts/"+strconv.FormatInt(id, 10), "assigned")
}

// AccountDelete — POST /w/{workspace}/accounts/{id}/delete. Soft-delete.
// Reversibel di DB (deleted_at), tapi tak ada UI restore di M2-3 — jejak audit
// mencatat siapa & kapan.
func (h *Handler) AccountDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedAccount(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteAccount(ctx, db.SoftDeleteAccountParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("accounts: delete", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "account.delete", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/accounts", "deleted")
}

// parseMemberRef menerjemahkan nilai form dropdown CSM: "" → (nil, "") (lepas
// tugas); angka → validasi keanggotaan → (&id, "") atau (nil, "csm") bila bukan
// anggota; tak terurai → (nil, "csm"). Non-anggota ditolak agar tak ada baris
// yatim yang tak terlihat siapa pun.
func (h *Handler) parseMemberRef(ctx context.Context, raw string) (*int64, string) {
	if raw == "" {
		return nil, ""
	}
	uid, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, "csm"
	}
	if _, err := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{
		UserID: uid, TenantID: session.TenantID(ctx),
	}); err != nil {
		return nil, "csm"
	}
	return &uid, ""
}

// assignableMembers = kandidat penerima tugas CSM (semua anggota workspace).
// Label = nama bila ada, jatuh ke email — penanda orang, bukan id telanjang.
func (h *Handler) assignableMembers(ctx context.Context) ([]panel.AccountMemberOption, error) {
	rows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]panel.AccountMemberOption, 0, len(rows))
	for _, m := range rows {
		label := m.Email
		if m.Name != nil && *m.Name != "" {
			label = *m.Name
		}
		out = append(out, panel.AccountMemberOption{ID: m.UserID, Label: label})
	}
	return out, nil
}
