package handler

import (
	"context"
	"encoding/json"
	"errors"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// notifications_notify.go — baris undangan pending, kode error PRG undangan, dan
// penulisan notifikasi (notify). Dipisah dari notifications_service.go (pemetaan
// baris + penyusun kalimat) agar file itu di bawah batas 150. orDefault dipakai
// notifText di sana, tetap dideklarasikan di sini karena TAK dipakai apa pun di
// file ini — satu paket, boleh lintas file.

// buildNotifInvites memetakan undangan pending ke baris view. Token dibawa ke
// form aksi (terima/tolak) — bukan rahasia baru: penerima memang pemiliknya.
func buildNotifInvites(rows []db.ListPendingInvitesByEmailRow) []panel.NotifInviteRow {
	out := make([]panel.NotifInviteRow, 0, len(rows))
	for _, i := range rows {
		out = append(out, panel.NotifInviteRow{
			Token:     i.Token,
			Workspace: i.TenantName,
			Role:      i.Role,
			Expires:   fmtLocal(i.ExpiresAt),
		})
	}
	return out
}

// inviteErrCode memetakan error undangan ke kode PRG (?err=CODE) — pasangan
// notifErrMsg. Dipisah dari inviteErrText (yang menghasilkan kalimat untuk
// halaman publik /invite) karena jalur ini redirect, bukan render langsung.
func inviteErrCode(err error) string {
	switch {
	case errors.Is(err, errInviteNotFound):
		return "notfound"
	case errors.Is(err, errInviteExpired):
		return "expired"
	case errors.Is(err, errInviteUsed):
		return "used"
	default:
		return "failed"
	}
}

// notify menulis satu notifikasi untuk user tertentu. FAIL-SOFT & tx TERPISAH
// (pola sama auditLog): gagal memberi tahu TIDAK boleh membatalkan aksi utama —
// role yang sudah berubah tak boleh ter-rollback hanya karena kabarnya gagal
// dicatat. WithSuper karena notifications lintas-workspace (tanpa RLS) dan
// penerimanya bisa bukan anggota workspace aktor.
func (h *Handler) notify(ctx context.Context, userID, tenantID int64, kind string, p notifPayload) {
	raw := []byte("{}")
	if b, err := json.Marshal(p); err == nil {
		raw = b
	}
	var tid *int64
	if tenantID > 0 {
		tid = &tenantID
	}
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		_, e := q.CreateNotification(ctx, db.CreateNotificationParams{
			UserID: userID, TenantID: tid, Kind: kind, Payload: raw,
		})
		return e
	}); err != nil {
		// Log pakai ID saja — jangan pernah email/nama (Rule 12: no PII di log).
		h.Log.Error("notify", "kind", kind, "userID", userID, "err", err)
	}
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
