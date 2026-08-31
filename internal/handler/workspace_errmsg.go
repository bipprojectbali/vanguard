package handler

// workspace_errmsg.go — peta kode PRG (`?err=CODE`) → pesan untuk halaman
// workspace & platform. SATU tempat, sengaja.
//
// Kenapa dipusatkan: peta seperti ini sempat tersebar, dan yang tersebar mudah
// jadi setengah lengkap — handler mengirim kode baru, petanya lupa diperbarui,
// lalu penolakannya HILANG SENYAP: user menekan tombol, halaman termuat ulang
// tampak normal, tak ada penjelasan apa pun.
//
// Itu bukan kekhawatiran teoretis. Dua kejadian nyata saat menulis ini:
//   - `?err=primary` (tolak arsip/hapus rumah aplikasi) tak pernah dirender
//     karena halaman pengaturan tak punya slot pesan sama sekali;
//   - `members.go` sudah lama mengirim `?err=forbidden` sementara petanya tak
//     mengenal kode itu — jadi setiap penolakan di halaman anggota tampil
//     sebagai halaman yang sekadar termuat ulang.
//
// Kode ringkas di URL (bukan kalimat penuh) agar rapi dan tak membocorkan detail
// internal. Kode tak dikenal → "" (tak ada alert): diam lebih baik daripada
// "error tak dikenal", yang membingungkan tanpa memberi informasi.

// wsErrMsg memetakan kode untuk seluruh halaman ber-workspace (pengaturan,
// anggota, daftar workspace platform).
//
// Satu peta, bukan satu per halaman: kode yang sama harus berbunyi sama di mana
// pun ia muncul, dan `failed`/`forbidden` memang dipakai hampir di semua jalur.
func wsErrMsg(code string) string {
	switch code {
	case "primary":
		return "Workspace ini adalah rumah aplikasi — tak bisa diarsipkan maupun dihapus."
	case "forbidden":
		return "Anda tidak berwenang melakukan tindakan ini."
	case "reason":
		return "Alasan wajib diisi."
	case "name":
		return "Nama workspace wajib diisi (maks 60 karakter)."
	case "quota":
		return "Kuota workspace Anda sudah penuh."
	case "code_entity":
		return "Entitas tidak dikenal."
	case "code_prefix":
		return "Prefix wajib diisi (1–16 karakter)."
	case "code_padding":
		return "Lebar angka harus 0–12."
	case "village_name":
		return "Nama desa wajib diisi (maks 200 karakter)."
	case "account_type":
		return "Tipe akun tidak valid."
	case "village_status":
		return "Status desa tidak valid."
	case "district_id":
		return "Kecamatan yang dipilih tidak valid."
	case "district_required":
		return "Kecamatan wajib dipilih."
	case "classification":
		return "Klasifikasi (IDM) tidak valid."
	case "number":
		return "Jumlah penduduk/dusun harus berupa angka bulat."
	case "budget":
		return "Anggaran desa harus berupa angka."
	case "village_code_dup":
		return "Kode desa (Kemendagri) itu sudah dipakai desa lain di workspace ini."
	case "entity_code":
		return "Kode sistem tidak valid (maks 32 karakter)."
	case "entity_code_dup":
		return "Kode sistem itu sudah dipakai desa lain di workspace ini."
	case "csm":
		return "Orang yang dipilih bukan anggota workspace ini."
	case "first_name":
		return "Nama depan kontak wajib diisi (maks 200 karakter)."
	case "contact_position":
		return "Jabatan kontak tidak valid."
	case "contact_role":
		return "Peran kontak tidak valid."
	case "contact_channel":
		return "Kanal komunikasi pilihan tidak valid."
	case "account_id":
		return "Desa induk wajib dipilih."
	case "role_name":
		return "Nama peran harus 2–32 karakter, huruf kecil/angka/garis bawah, diawali huruf."
	case "role_display":
		return "Nama tampilan peran wajib diisi (maks 60 karakter)."
	case "role_scope":
		return "Cakupan data peran tidak valid."
	case "role_desc":
		return "Deskripsi peran terlalu panjang (maks 200 karakter)."
	case "role_exists":
		return "Nama peran itu sudah dipakai di workspace ini."
	case "role_system":
		return "Peran bawaan sistem tak bisa disunting atau dihapus."
	case "role_notfound":
		return "Peran tidak ditemukan."
	case "crm_role":
		return "Peran CRM yang dipilih tidak ada di workspace ini."
	case "notfound":
		return "Desa tidak ditemukan atau di luar cakupan Anda."
	}
	return wsErrMsgCRM(code)
}
