package handler

import (
	"strconv"

	"go_starter/internal/settings"
)

// dev_settings_msg.go — pemetaan kode ?err=/?ok= → pesan untuk halaman
// /dev/settings (kuota & tenancy). Dipisah dari dev_settings.go (handler)
// agar tiap file di bawah ambang Route/Handler (150). Murni string→string.
// settingsErr/settingsMsg memetakan kode PRG ke pesan (pola authErrMsg).
func settingsErr(code string) string {
	switch code {
	case "quota":
		return "Kuota harus antara " + strconv.Itoa(settings.MinWorkspaceQuota) +
			" dan " + strconv.Itoa(settings.MaxWorkspaceQuota)
	case "retention":
		return "Masa simpan jejak harus antara " + strconv.Itoa(settings.MinAuditRetentionDays) +
			" dan " + strconv.Itoa(settings.MaxAuditRetentionDays) + " hari"
	case "confirm":
		return "Konfirmasi tidak cocok — ketik nama aplikasi persis seperti yang tertulis"
	case "failed":
		return "Gagal menyimpan pengaturan"
	default:
		return ""
	}
}

func settingsMsg(code string) string {
	switch code {
	case "saved":
		return "Pengaturan disimpan dan langsung berlaku"
	case "tenancy":
		return "Aplikasi kini berjalan sebagai multi-workspace. Berlaku seketika — " +
			"alamat ruang kerja yang sudah ada tidak berubah."
	default:
		return ""
	}
}
