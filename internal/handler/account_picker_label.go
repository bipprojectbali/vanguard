package handler

// account_picker_label.go — BL-76: SATU sumber kebenaran format label pemilih
// Desa/Account di SELURUH CRM. Semua picker (Sales Deals, Contacts, Success
// Plans, Sales Activities, Tickets, Engagements, CS Trainings, CS Impl Tasks)
// WAJIB memanggil helper ini agar format seragam & tak lagi berdivergensi.
//
// Keputusan user (7 Sep, BL-76):
//
//	(i)  Kode yang tampil = village_code (kode wilayah Kemendagri dari BL-66/67)
//	     SAJA — BUKAN entity_code (kode internal/sistem yang justru DISEMBUNYIKAN
//	     dari UI oleh BL-61). Kode di depan agar terurut & bisa dicari via kode.
//	(ii) village_code NULL/kosong (akun lama pra-BL-66) → fallback NAMA SAJA,
//	     JANGAN render " — nama" menggantung.
//
// Format: "{village_code} — {village_name}"  atau  "{village_name}" bila kode nil.
func accountPickerLabel(villageCode *string, name string) string {
	if villageCode != nil && *villageCode != "" {
		return *villageCode + " — " + name
	}
	return name
}
