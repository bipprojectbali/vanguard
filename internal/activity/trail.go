package activity

import "strings"

// trail.go — menyusun kalimat peristiwa audit. Pure (tanpa DB/HTTP) agar bisa
// diuji tanpa Postgres, sama seperti sisa paket ini.
//
// Kenapa kalimat, bukan kolom-kolom mentah: `member.role.update → #8` menuntut
// pembacanya menerjemahkan sendiri kode dan mencari siapa #8 — dua pekerjaan
// yang justru dilakukan saat orang sedang menyelidiki sesuatu dan paling tak
// punya waktu. Polanya sama dengan notifText di handler.

// Event = satu peristiwa audit siap dirangkai jadi kalimat. Nama-nama datang
// dari JOIN saat baca, TAK PERNAH dari metadata (kolom itu sengaja bebas PII).
// Semua boleh kosong: pelaku/sasaran bisa sudah terhapus, dan jejaknya justru
// wajib tetap terbaca setelah itu.
type Event struct {
	Action     string
	TargetType string
	Actor      string // nama/email pelaku; "" bila sudah terhapus
	TargetUser string // nama/email sasaran (target_type user/session)
	Workspace  string // nama workspace sasaran (target_type workspace)
	Role       string // metadata.to / metadata.role
	Value      string // metadata.value (kuota, mode)
	Reason     string // metadata.reason (suspend)
	Method     string // metadata.method (login: password/google)
}

// Family mengembalikan segmen pertama action ("workspace.create" → "workspace").
// Dipakai untuk mengelompokkan & memberi warna; penamaan aksi kita selalu
// `keluarga.sisanya`, jadi tak butuh tabel pemetaan yang harus dijaga selaras.
func Family(action string) string {
	fam, _, found := strings.Cut(action, ".")
	if !found || fam == "" {
		return "lainnya"
	}
	return fam
}

// orUnknown menjaga kalimat tetap utuh saat namanya hilang. "Seseorang" dipilih
// alih-alih mengosongkan bagian itu: kalimat yang menggantung ("mengubah role
// menjadi admin") membuat pembaca mengira ada data yang gagal dimuat, padahal
// yang terjadi adalah pelakunya memang sudah dihapus.
func orUnknown(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// Sentence menyusun kalimat peristiwa dalam bahasa Indonesia.
//
// Action tak dikenal TIDAK menghasilkan string kosong melainkan kalimat netral
// beserta kodenya: jejak audit dibaca justru saat ada yang aneh, dan baris yang
// menghilang karena kodenya belum dikenal adalah kegagalan paling buruk yang
// bisa dimiliki halaman ini. Baris lama dari versi berbeda juga masuk ke sini.
func Sentence(e Event) string {
	actor := orUnknown(e.Actor, "Seseorang")
	target := orUnknown(e.TargetUser, "seorang user")
	ws := orUnknown(e.Workspace, "sebuah workspace")

	switch e.Action {
	case "auth.login":
		if e.Method != "" {
			return actor + " masuk lewat " + e.Method + "."
		}
		return actor + " masuk."
	case "auth.logout":
		return actor + " keluar."

	case "member.role.update":
		if e.Role != "" {
			return actor + " mengubah role " + target + " menjadi " + e.Role + "."
		}
		return actor + " mengubah role " + target + "."
	case "member.remove":
		return actor + " mengeluarkan " + target + " dari workspace."

	case "invite.create":
		if e.Role != "" {
			return actor + " mengundang seseorang ke " + ws + " sebagai " + e.Role + "."
		}
		return actor + " mengundang seseorang ke " + ws + "."
	case "invite.accept":
		if e.Role != "" {
			return actor + " bergabung ke " + ws + " sebagai " + e.Role + "."
		}
		return actor + " bergabung ke " + ws + "."

	case "workspace.create":
		return actor + " membuat workspace " + ws + "."
	case "workspace.rename":
		return actor + " mengganti nama workspace " + ws + "."
	case "workspace.archive":
		return actor + " mengarsipkan " + ws + "."
	case "workspace.unarchive":
		return actor + " membuka kembali " + ws + " dari arsip."
	case "workspace.delete":
		return actor + " menghapus " + ws + "."
	case "workspace.suspend":
		if e.Reason != "" {
			return actor + " menangguhkan " + ws + " — alasan: " + e.Reason
		}
		return actor + " menangguhkan " + ws + "."

	case "user.status.update":
		if e.Value != "" {
			return actor + " mengubah status " + target + " menjadi " + e.Value + "."
		}
		return actor + " mengubah status " + target + "."
	case "user.delete":
		return actor + " menghapus akun " + target + "."
	case "user.quota.set":
		if e.Value != "" {
			return actor + " memberi " + target + " kuota khusus " + e.Value + " workspace."
		}
		return actor + " memberi " + target + " kuota khusus."
	case "user.quota.reset":
		return actor + " mengembalikan kuota " + target + " ke default global."

	case "settings.workspace_quota":
		if e.Value != "" {
			return actor + " mengubah kuota workspace default menjadi " + e.Value + "."
		}
		return actor + " mengubah kuota workspace default."
	case "platform.tenancy.upgrade":
		return actor + " menaikkan aplikasi ke mode multi-workspace."

	case "contact.create":
		return actor + " menambahkan seorang kontak ke sebuah desa."
	case "contact.update":
		return actor + " menyunting data seorang kontak."
	case "contact.primary":
		return actor + " menetapkan kontak utama sebuah desa."
	case "contact.delete":
		return actor + " menghapus seorang kontak."

	default:
		// Kodenya ikut disebut — tanpa itu pembaca tahu ada sesuatu yang terjadi
		// tapi tak punya apa pun untuk dicari di kode sumber.
		return actor + " melakukan tindakan " + e.Action + "."
	}
}
