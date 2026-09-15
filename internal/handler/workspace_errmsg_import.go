package handler

// workspace_errmsg_import.go — lanjutan wsErrMsg khusus kode PRG impor CSV
// (BL-63, Modul 2 Accounts). Dipisah dari workspace_errmsg_crm.go (Sales/
// Subscriptions) agar tiap file di bawah ambang tipe Route/Handler (150) dan
// kelompok kode tetap satu tanggung jawab per file. Kode yang SUDAH ada &
// dipakai ulang di sini (bukan didefinisikan ulang): "village_id" (kode desa
// di baris CSV tak cocok master), "village_code_dup" (tabrakan vs akun HIDUP
// yang SUDAH ADA di tenant — dipakai bersama create manual/konversi lead,
// workspace_errmsg.go), "account_type" dst. dari parseAccountForm
// (accounts_form.go/workspace_errmsg.go) — satu baris CSV tak valid memetakan
// ke kode form yang SAMA dgn create manual, bukan set pesan baru yang bisa
// berbeda maknanya untuk kolom yang sama. "village_code_dup_file" DI BAWAH
// SENGAJA dipisah dari "village_code_dup" (bukan didefinisikan ulang di sana)
// — sama-sama menolak baris tapi PENYEBABNYA beda (akun lain yang sudah ada
// vs baris lain di file yang sama), jadi pesannya juga wajib beda agar
// operator tahu ke mana harus memperbaiki (data master vs file-nya sendiri).
func wsErrMsgImport(code string) string {
	switch code {
	case "import_invalid":
		return "File CSV tidak terbaca atau kolom header tidak dikenali. Unduh template dan coba lagi."
	case "import_empty":
		return "File CSV tidak berisi baris data."
	case "import_too_many":
		return "File CSV melebihi batas jumlah baris impor sekali unggah."
	case "import_row_invalid":
		return "Data berubah sejak pratinjau dibuat — silakan unggah ulang file CSV."
	case "owner_email_notfound":
		return "Email pemilik pada salah satu baris tidak cocok dengan anggota workspace ini."
	case "village_code_dup_file":
		return "Kode desa ini dipakai lebih dari satu baris di file yang sama."
	default:
		return ""
	}
}
