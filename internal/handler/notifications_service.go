package handler

import (
	"encoding/json"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// notifications_service.go — pemetaan baris DB → baris view + penyusun kalimat
// notifikasi. Dipisah dari notifications.go (handler HTTP) agar tiap file di
// bawah batas 150.
//
// buildNotifInvites, inviteErrCode, notify (penulisan), dan orDefault dipisah
// ke notifications_notify.go agar file ini tetap di bawah ambang yang sama.

// notifPayload = bentuk payload JSONB yang kita tulis. Disimpan sebagai SNAPSHOT
// (lihat migrasi 00009): pesan lama tetap terbaca walau role/nama berubah lagi.
type notifPayload struct {
	Role        string `json:"role,omitempty"`         // role BARU (member.role.changed)
	Actor       string `json:"actor,omitempty"`        // yang melakukan, sudah aman ditampilkan
	EntityCode  string `json:"entity_code,omitempty"`  // kode entitas (mis. SUB-0007); BUKAN PII
	VillageName string `json:"village_name,omitempty"` // nama desa/akun (BL-158, subscription.renewal.reminder)
	DaysLeft    *int   `json:"days_left,omitempty"`    // sisa hari ke end_date (BL-158); pointer agar 0 (hari-H) terbedakan dari absen
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
	case "member.kind.changed":
		label := "Internal"
		if p.Role == "external" {
			label = "Eksternal"
		}
		return "Jenis Anggota Anda di " + orDefault(workspace, "workspace") + " diubah menjadi " + label + "."
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
	case "subscription.created.from_deal":
		return "Langganan " + orDefault(p.EntityCode, "baru") + " dibuat dari deal yang dimenangkan."
	case "subscription.renewal.reminder":
		// BL-158: pengingat terjadwal H-30/H-14/H-7/hari-H. VillageName = akun
		// pemilik langganan (bukan workspace platform); DaysLeft nil = payload
		// lama/rusak → kalimat generik, fail-soft konsisten default lain.
		village := orDefault(p.VillageName, "Langganan Anda")
		if p.DaysLeft == nil {
			return "Langganan " + village + " mendekati masa jatuh tempo."
		}
		if *p.DaysLeft == 0 {
			return "Langganan " + village + " jatuh tempo hari ini."
		}
		return "Langganan " + village + " akan jatuh tempo dalam " + strconv.Itoa(*p.DaysLeft) + " hari."
	default:
		return "Ada pembaruan pada keanggotaan Anda."
	}
}
