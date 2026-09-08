// Package changelog menyediakan catatan rilis berfokus BISNIS/CRM sebagai data
// terstruktur — sumber TUNGGAL untuk modal "Pembaruan" di UI dan penentu nomor
// versi badge "ada pembaruan".
//
// Sengaja terpisah dari CHANGELOG.md (log dev yang verbose: RLS, MCP, DSN, dsb.):
// paket ini hanya memuat perubahan yang berarti bagi PENGGUNA CRM, ditulis dalam
// bahasa operator — bukan detail infrastruktur. Menambah rilis = sisipkan entri
// baru di INDEKS 0 (terbaru dulu); badge otomatis menyala di browser yang belum
// melihat versi itu (lihat static/changelog.js).
package changelog

// Section = satu kelompok perubahan dalam sebuah rilis (mis. "Baru",
// "Diperbaiki"). Title boleh "" bila rilis hanya punya satu daftar tak berjudul.
type Section struct {
	Title string
	Items []string
}

// Release = satu versi rilis beserta perubahannya (bahasa pengguna).
type Release struct {
	Version  string // semver TANPA prefix "v" (mis. "1.0.0")
	Date     string // ISO "YYYY-MM-DD"
	Summary  string // ringkas satu kalimat; "" bila tak ada
	Sections []Section
}

// Releases = seluruh rilis, TERBARU DI INDEKS 0. Sumber tunggal modal + badge.
// Nomor versi teratas menjadi acuan badge "ada pembaruan".
var Releases = []Release{
	{
		Version: "1.0.0",
		Date:    "2026-09-08",
		Summary: "Rilis pertama CRM Desa+ — sembilan modul inti untuk mengelola desa dari prospek hingga langganan aktif.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Beranda ringkas — ringkasan lintas domain (Sales, Langganan, Customer Success, Support) yang tampil sesuai izin peran.",
					"Manajemen Desa (Accounts) — daftar & detail desa dengan kode wilayah resmi Kemendagri, beserta kontak per desa.",
					"Sales — Leads, konversi Lead → Desa/Deal, pipeline Deal enam tahap, dan Quote (builder item + pajak).",
					"Langganan & Paket — kelola langganan, paket & harga (IDR), perpanjangan, serta pemantauan churn.",
					"Customer Success — Health Score, Success Plans, Engagement, Customer Journey, dan Knowledge Base.",
					"Aktivitas — linimasa terpadu (tugas, telepon, catatan, pertemuan, chat) per desa.",
					"Laporan — empat laporan siap pakai (Sales, Langganan, Customer Success, Support) dengan ekspor CSV.",
					"Peran & keamanan data — matriks peran CRM dan penyamaran data sensitif (ARR, catatan internal, kontak).",
				},
			},
		},
	},
}

// Current mengembalikan nomor versi rilis terbaru (Releases[0].Version); ""
// bila belum ada rilis. Dipakai handler untuk mengisi ShellData.ChangelogVersion
// (acuan badge).
func Current() string {
	if len(Releases) == 0 {
		return ""
	}
	return Releases[0].Version
}
