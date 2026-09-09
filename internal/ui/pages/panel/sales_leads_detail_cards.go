package panel

import (
	g "maragu.dev/gomponents"
)

// sales_leads_detail_cards.go — EMPAT kartu detail lead (wireframe 4.1), dipisah
// dari sales_leads_detail.go agar file halaman tetap di bawah ambang file-health.
// Murni susunan data: semua nilai sudah diformat/disamarkan handler (F4). Reuse
// detailCard/detailField (accounts_detail.go).

// leadIdentityCard = "Identitas Lead": nama, kontak, jabatan, pemilik, sumber.
func leadIdentityCard(v LeadDetailView) g.Node {
	return detailCard("Identitas Lead", []detailField{
		{"Nama Lead", v.LeadName},
		{"Kontak", v.ContactPerson},
		{"Jabatan", v.JobTitle},
		{"Pemilik", v.Owner},
		{"Sumber", v.LeadSource},
	})
}

// leadQualificationCard = "Kualifikasi & Status". BL-121: baris "Alasan
// Unqualified" HANYA muncul saat status = Unqualified (leadUnqualified) — bukan
// lagi selalu tampil "—". "Konversi" = ringkas keadaan konversi (label jujur).
func leadQualificationCard(v LeadDetailView) g.Node {
	fields := []detailField{
		{"Status", v.Status},
		{"Rating", v.Rating},
		{"Nilai Estimasi", v.EstValue},
	}
	if v.Status == leadUnqualified {
		fields = append(fields, detailField{"Alasan Unqualified", v.UnqualifiedReason})
	}
	fields = append(fields, detailField{"Konversi", leadConvertedLabel(v)})
	return detailCard("Kualifikasi & Status", fields)
}

// leadConvertedLabel = ringkas keadaan konversi utk kartu Kualifikasi (wireframe:
// "Belum — klik Konversi"). Terkonversi → "Sudah dikonversi"; Qualified &
// boleh-tulis → ajakan; selain itu → "Belum".
func leadConvertedLabel(v LeadDetailView) string {
	switch {
	case v.Converted:
		return "Sudah dikonversi"
	case v.CanConvert:
		return "Belum — klik Konversi"
	default:
		return "Belum"
	}
}

// leadLocationCard = "Lokasi & Kontak": wilayah + nomor (tersamar handler) + email.
func leadLocationCard(v LeadDetailView) g.Node {
	return detailCard("Lokasi & Kontak", []detailField{
		{"Provinsi", v.Province},
		{"Kabupaten/Kota", v.Regency},
		{"Kecamatan", v.District},
		{"HP / WhatsApp", v.MobilePhone},
		{"Email", v.Email},
	})
}

// leadSystemAuditCard = "Sistem & Audit" (BL-119, wireframe 4.1): pembuat/pengubah
// + waktu (read-only). Sejajar kartu audit detail Kontak.
func leadSystemAuditCard(v LeadDetailView) g.Node {
	return detailCard("Sistem & Audit", []detailField{
		{"Dibuat Oleh", v.CreatedByName},
		{"Tanggal Dibuat", v.CreatedAt},
		{"Diubah Oleh", v.UpdatedByName},
		{"Terakhir Diubah", v.UpdatedAt},
	})
}
