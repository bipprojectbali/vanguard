package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts_detail_cards.go — ENAM kartu ringkasan detail kontak, dipisah dari
// contacts_detail.go agar file halaman tetap di bawah batas file-health. Murni
// susunan data: semua nilai sudah diformat/disamarkan handler (F4). Urutan kartu
// mengikuti wireframe M3; kolom "Terakhir Dihubungi/Aktivitas" ditunda modul
// Activities (handler mengisi "—").

// contactDetailCards merender grid dua kolom (mobile: satu kolom) berisi enam
// kartu: Identitas, Peran & Otoritas, Komunikasi, Alamat, Ringkasan Keterlibatan,
// dan Sistem & Audit (read-only).
func contactDetailCards(v ContactDetailView) g.Node {
	return h.Div(
		h.Class("grid gap-4 md:grid-cols-2 min-w-0"),
		detailCard("Identitas", []detailField{
			{"Sapaan", v.Salutation},
			{"Nama Depan", v.FirstName},
			{"Nama Belakang", v.LastName},
			{"Jabatan", v.JobTitle},
			{"Desa", v.VillageName},
			{"Pemilik Kontak", v.OwnerName},
		}),
		detailCard("Peran & Otoritas", []detailField{
			{"Kategori Perangkat Desa", v.PositionCategory},
			{"Peran", v.ContactRole},
			{"Kontak Utama", boolLabel(v.IsPrimary)},
			{"Kontak Teknis", boolLabel(v.IsTechnical)},
			{"Periode Menjabat", v.TermPeriod},
		}),
		detailCard("Komunikasi", []detailField{
			{"HP (Pribadi)", v.MobilePhone},
			{"WhatsApp", v.WhatsappNumber},
			{"Telepon Kantor", v.OfficePhone},
			{"Email", v.Email},
			{"Kanal Pilihan", v.PreferredChannel},
		}),
		detailCard("Alamat", []detailField{
			{"Alamat", v.MailingAddress},
			{"Kota", v.City},
			{"Kode Pos", v.PostalCode},
		}),
		detailCard("Ringkasan Keterlibatan", []detailField{
			{"Terakhir Dihubungi", v.LastContacted},
			{"Aktivitas Terakhir", v.LastActivity},
			{"Opt-out Email", boolLabel(v.EmailOptOut)},
			{"Jangan Hubungi", boolLabel(v.DoNotContact)},
		}),
		detailCard("Sistem & Audit", []detailField{
			{"Dibuat Oleh", v.CreatedByName},
			{"Tanggal Dibuat", v.CreatedAt},
			{"Diubah Oleh", v.UpdatedByName},
			{"Terakhir Diubah", v.UpdatedAt},
		}),
	)
}
