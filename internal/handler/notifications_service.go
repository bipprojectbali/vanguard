package handler

import (
	"context"
	"encoding/json"
	"errors"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// notifications_service.go — pemetaan baris DB → baris view + penyusun pesan.
// Dipisah dari notifications.go (handler HTTP) agar tiap file di bawah batas 150.

// notifPayload = bentuk payload JSONB yang kita tulis. Disimpan sebagai SNAPSHOT
// (lihat migrasi 00009): pesan lama tetap terbaca walau role/nama berubah lagi.
type notifPayload struct {
	Role       string `json:"role,omitempty"`        // role BARU (member.role.changed)
	Actor      string `json:"actor,omitempty"`       // yang melakukan, sudah aman ditampilkan
	EntityCode string `json:"entity_code,omitempty"` // kode entitas (mis. SUB-0007); BUKAN PII
}

// buildNotifRows memetakan peristiwa ke baris view + menyusun kalimatnya.
// Payload rusak/tak terbaca TIDAK menggagalkan render — baris tetap tampil
// dengan pesan generik (fail-soft; notifikasi bukan sumber kebenaran data).
func buildNotifRows(rows []db.ListNotificationsRow) []panel.NotifRow {
	out := make([]panel.NotifRow, 0, len(rows))
	for _, n := range rows {
		var p notifPayload
		if len(n.Payload) > 0 {
			_ = json.Unmarshal(n.Payload, &p)
		}
		ws := ""
		if n.TenantName != nil {
			ws = *n.TenantName
		}
		out = append(out, panel.NotifRow{
			Text:      notifText(n.Kind, ws, p),
			Workspace: ws,
			When:      fmtLocal(n.CreatedAt),
			Unread:    !n.ReadAt.Valid,
		})
	}
	return out
}

// notifText menyusun kalimat notifikasi. Kind tak dikenal (mis. baris lama dari
// versi lebih baru) → kalimat netral, bukan string kosong.
func notifText(kind, workspace string, p notifPayload) string {
	switch kind {
	case "member.role.changed":
		if p.Role != "" {
			return "Role Anda di " + orDefault(workspace, "workspace") + " diubah menjadi " + p.Role + "."
		}
		return "Role Anda di " + orDefault(workspace, "workspace") + " diubah."
	case "member.business_role.changed":
		if p.Role != "" {
			return "Peran CRM Anda di " + orDefault(workspace, "workspace") + " diubah menjadi " + p.Role + "."
		}
		return "Peran CRM Anda di " + orDefault(workspace, "workspace") + " dicabut."
	case "member.removed":
		return "Anda dikeluarkan dari " + orDefault(workspace, "sebuah workspace") + "."
	case "workspace.joined":
		return "Anda bergabung ke " + orDefault(workspace, "sebuah workspace") + "."
	case "renewal.upsell.pending":
		return "Renewal upsell " + orDefault(p.EntityCode, "langganan") + " menunggu persetujuan Anda."
	case "renewal.approved":
		return "Renewal " + orDefault(p.EntityCode, "langganan") + " Anda telah disetujui."
	case "renewal.rejected":
		return "Renewal " + orDefault(p.EntityCode, "langganan") + " Anda ditolak."
	default:
		return "Ada pembaruan pada keanggotaan Anda."
	}
}

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
