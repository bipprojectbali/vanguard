package handler

import (
	"strconv"

	"go_starter/internal/settings"
)

// dev_users_msg.go — pemetaan kode ?err= → pesan untuk jalur kuota per-user di
// /dev/users (DevUserQuota/DevUserQuotaReset). Terpisah dari settingsErr
// (dev_settings_msg.go): kode "quota"/"failed" di sana bicara soal DEFAULT
// GLOBAL, di sini soal HAK KHUSUS satu user — teksnya perlu beda konteks
// walau kodenya sama, jadi tak direuse.
//
// Ditemukan saat audit 156h (dev_settings): DevUserQuota/DevUserQuotaReset
// memakai redirect PRG native (bukan SSE flash seperti aksi role/status/hapus
// di halaman yang sama) tapi UsersView tak pernah punya slot Err — kode
// hilang senyap. Lihat [[bl-156-toast-floating]].
func devUsersErrMsg(code string) string {
	switch code {
	case "quota":
		return "Kuota harus antara " + strconv.Itoa(settings.MinWorkspaceQuota) +
			" dan " + strconv.Itoa(settings.MaxWorkspaceQuota)
	case "failed":
		return "Gagal menyimpan perubahan kuota"
	default:
		return ""
	}
}
