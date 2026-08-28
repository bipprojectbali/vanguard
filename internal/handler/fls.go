package handler

import "go_starter/internal/authz"

// fls.go — Field-Level Security (sistem-dan-role.md §5, diperkuat §8.3).
//
// Sumbu ini BEDA dari dua yang lain dan tak boleh dicampur: Casbin (F2) menjawab
// "boleh membuka modul APA", ownership (F3) menjawab "atas desa SIAPA", dan FLS
// di sini menjawab "dalam sebuah baris yang SUDAH boleh ia lihat, field mana yang
// boleh tampil". Seseorang bisa berhak membuka Accounts (Casbin) atas desanya
// (ownership) namun TETAP tak boleh melihat nomor HP kontak di dalamnya (FLS).
//
// Kenapa di HANDLER, bukan di view (sama seperti maskEmail): kalau view yang
// menyamarkan, nilai ASLI tetap harus dioper ke sana — dan satu view yang lupa
// akan mengirimnya ke browser, terbaca di source meski tak tampak di layar. Maka
// nilai sensitif disamarkan DI SINI; yang tak berhak TAK PERNAH menerimanya.
// Itu pula kriteria penerimaan F4: "field masked tak pernah dioper ke view".
//
// FLS memakai business_role (sumbu CRM), BUKAN role tenant/platform: super_admin
// atau owner yang bukan pemegang peran CRM = "" → tak lolos apa pun di sini
// (tegak lurus, §3). Naikkan otoritas CRM lewat memberships.business_role, bukan
// lewat wewenang platform.

// flsHidden = penanda universal "disembunyikan". Dipakai untuk field yang, saat
// tak berhak, tetap perlu MENGISI kolom (mis. ARR di tabel langganan) agar tata
// letak tak berubah antar-role — beda dengan catatan internal yang justru lebih
// aman bila kolomnya kosong sama sekali (lihat maskInternalNotes).
const flsHidden = "•••"

// canSeeARR — Nilai Kontrak / ARR terbuka bagi semua business_role KECUALI
// Support (§5): agen tiket tak perlu tahu nilai komersial. Manager SENGAJA tetap
// melihat (§8.3) — ia butuh angka itu untuk menyetujui diskon; itulah satu-
// satunya field yang mengecualikan Manager dari FLS.
//
// Allow-list, bukan deny-list ("!= support"): FLS wajib fail-CLOSED seperti F2
// (Casbin deny-default) & F3 (ScopeNone). Role tak dikenal / role platform yang
// tersasar ke sini (super_admin/owner/staff BUKAN business_role) → tersembunyi,
// bukan lolos diam-diam. Sumbu tetap tegak lurus (§3).
func canSeeARR(businessRole string) bool {
	switch businessRole {
	case authz.BusinessRoleAdmin, authz.BusinessRoleManager,
		authz.BusinessRoleSales, authz.BusinessRoleCSM:
		return true
	default: // support, "", role platform, nilai liar
		return false
	}
}

// canSeeFullPhone — Nomor HP kontak utuh untuk Sales & Admin (§5): Sales adalah
// pemegang hubungan yang memang menghubungi kepala desa; Admin CRM diberi akses
// penuh sebagai pengelola workspace yang perlu memverifikasi/memperbaiki kontak.
// Manager, CSM, & Support tetap melihat versi tersamar — membatasi sebaran PII.
//
// Tetap allow-list (fail-CLOSED): role platform yang tersasar ke sini
// (super_admin/owner/staff BUKAN business_role) & nilai liar tersembunyi. Sumbu
// tetap tegak lurus (§3) — otoritas platform TIDAK membuka field ini; hanya
// business_role = admin yang lolos.
func canSeeFullPhone(businessRole string) bool {
	return businessRole == authz.BusinessRoleSales ||
		businessRole == authz.BusinessRoleAdmin
}

// canSeeInternalNotes — Catatan Internal HANYA Admin & CSM (§5): isinya penilaian
// jujur tentang pelanggan, jadi Sales/Manager/Support tak melihatnya. Predikat
// diekspos terpisah agar handler/view bisa memutuskan menampilkan penanda
// "terkunci" alih-alih kolom kosong, bila memang diinginkan.
func canSeeInternalNotes(businessRole string) bool {
	return businessRole == authz.BusinessRoleAdmin || businessRole == authz.BusinessRoleCSM
}

// Health Score SENGAJA tanpa predikat: §5 menyatakannya terbuka untuk SEMUA role
// ("justru harus dilihat bersama"). Tidak ada gate di sini adalah keputusan, bukan
// kelalaian — menambahkannya akan mengingkari maksud spec. Jangan tambahkan.

// maskARR mengembalikan nilai ARR siap-tampil: string yang SUDAH diformat bila
// berhak, atau penanda tersembunyi bila tidak. Sengaja menerima string
// terformat (bukan angka) supaya urusan format mata uang tetap di handler dan
// FLS hanya soal boleh-tampil-atau-tidak. Nilai asli tak pernah keluar saat
// tersembunyi.
func maskARR(formatted, businessRole string) string {
	if canSeeARR(businessRole) {
		return formatted
	}
	return flsHidden
}

// maskPhone menyamarkan nomor HP bagi role selain Sales & Admin. Penyamaran = SEMBUNYIKAN PENUH
// (penanda tetap), bukan sekadar menutup sebagian: alasan §5 adalah "membatasi
// sebaran PII", dan membocorkan digit awal/akhir tetap membocorkan nomor sekaligus
// panjangnya. Baris tetap bisa dibedakan lewat NAMA kontak — jadi tak ada guna
// membocorkan digit demi "biar bisa dikenali" (beda dari maskEmail, yang lahir
// saat tabel belum punya kolom nama). Nomor kosong tetap kosong (tak ada yang
// disembunyikan).
func maskPhone(phone, businessRole string) string {
	if phone == "" {
		return ""
	}
	if canSeeFullPhone(businessRole) {
		return phone
	}
	return flsHidden
}

// maskInternalNotes mengembalikan catatan bila berhak (Admin/CSM), atau string
// KOSONG bila tidak — bukan penanda "•••". Untuk catatan, mengosongkan lebih
// aman daripada menandai: menandai membocorkan bahwa catatan ITU ADA, dan
// keberadaannya sendiri sudah sinyal ("desa ini dicatati diam-diam"). Bila kelak
// perlu membedakan "kosong" dari "terkunci" di UI, pakai canSeeInternalNotes di
// handler untuk merender penanda — JANGAN oper isinya lalu sembunyikan di view.
func maskInternalNotes(notes, businessRole string) string {
	if canSeeInternalNotes(businessRole) {
		return notes
	}
	return ""
}
