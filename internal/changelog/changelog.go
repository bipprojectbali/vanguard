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
		Version: "1.4.0",
		Date:    "2026-09-11",
		Summary: "Health Score & Customer Success kini scoped ke desa pelanggan, detail Deal didesain ulang, tap-info jadi bubble, grafik Health Score jadi pie chart, dan sejumlah perapian & perbaikan pada Quote, Langganan, dan Anggota.",
		Sections: []Section{
			{
				Title: "Diubah",
				Items: []string{
					"Halaman Health Score & Customer Success kini hanya menampilkan desa yang pernah/sedang berlangganan — dua tampilan \"Aktif\" (langganan hidup) dan \"Churned\" (mantan pelanggan); desa yang masih sebatas prospek (belum pernah berlangganan) tak lagi muncul di sini.",
					"Detail Customer Success untuk desa yang belum atau tak lagi berlangganan kini hanya bisa dilihat (tak bisa disunting), dengan penjelasan langsung di kartu; tombol Penugasan CS, Implementation Tracker, dan Training Schedule ikut disembunyikan untuk desa tersebut.",
					"Detail Deal didesain ulang mengikuti wireframe terbaru — tahap pipeline kini langkah horizontal di bagian atas, kartu-kartu dirapikan sejajar, dan aksi Sunting/Ubah Tahap/Hapus disembunyikan begitu deal mencapai tahap akhir (Won/Lost).",
					"Seluruh ikon info (ⓘ) di form kini muncul sebagai bubble/popover mengambang, bukan lagi teks yang mengalir di bawah label — tetap dibuka dengan tap, bukan hover.",
					"Halaman Anggota kini hanya menampilkan pilihan \"Peran CRM\" per baris; pengaturan role tenant (Admin/Anggota) dipindah sepenuhnya ke panel developer.",
					"Notifikasi belum dibaca kini tetap terlihat sebagai titik kecil di ikon lonceng saat sidebar diciutkan, tak hanya saat sidebar terbuka penuh.",
					"Kartu format kode untuk Desa dihapus dari halaman Format Kode — desa kini memakai kode wilayah resmi, bukan format kode internal.",
					"Input \"Nominal Pajak (Rp)\" pada form Edit Pajak Quote kini berformat ribuan otomatis saat mengetik.",
					"Status Quote tak bisa dinaikkan dari Draft selama belum ada minimal satu item/paket di dalamnya.",
					"Kolom \"Status\" pada Daftar Langganan berganti nama jadi \"Masa Berlaku\" (agar tak rancu dengan status daur hidup langganan), dengan tingkatan yang lebih halus: Aman, Perlu Perhatian, Segera Jatuh Tempo, Jatuh Tempo (tepat hari-H), dan Masa Tenggang. Label \"Jatuh Tempo\" pada halaman Perpanjangan diganti jadi \"Akan Jatuh Tempo\" agar tak tertukar makna dengan kolom Masa Berlaku.",
					"Kartu \"Sebaran Kesehatan\" dan \"Arah Pergerakan\" pada halaman Health Score kini tampil sebagai pie/donut chart, bukan lagi bar horizontal — lebih mudah dibaca proporsinya sekilas.",
					"Form \"Tandai Churn\" pada detail langganan kini menumpuk Alasan churn & Tipe churn dalam satu kolom penuh, masing-masing dengan ikon info (ⓘ) yang bisa di-tap untuk melihat penjelasan singkat tiap pilihan.",
				},
			},
			{
				Title: "Diperbaiki",
				Items: []string{
					"Halaman detail Quote kini menampilkan pesan sukses/gagal (mis. saat mencoba meng-Accept quote kedua pada deal yang sudah punya quote diterima) — sebelumnya pesan ini senyap tanpa penjelasan apa pun.",
					"Badge \"Diperpanjang\" pada halaman Perpanjangan tak lagi salah tertulis \"Jatuh Tempo\", dan langganan yang sudah diperpanjang tak lagi terhitung dua kali pada KPI.",
				},
			},
		},
	},
	{
		Version: "1.3.0",
		Date:    "2026-09-10",
		Summary: "Detail lead didesain ulang dengan aksi lewat modal, Kategori Forecast jadi pilihan terkunci, perapian aksi langganan & form Health Score, plus perbaikan tanggal dan label pada daftar perpanjangan.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Kategori Forecast pada deal kini pilihan terkunci — Pipeline / Best Case / Commit / Closed — bukan lagi teks bebas yang mudah salah ketik.",
				},
			},
			{
				Title: "Diubah",
				Items: []string{
					"Detail lead didesain ulang — informasi dikelompokkan jadi kartu (Identitas, Kualifikasi & Status, Sistem & Audit), aksi \"Ubah Status\" dan \"Hapus\" kini tombol yang membuka modal konfirmasi, dan baris \"Alasan Unqualified\" hanya muncul saat status lead memang Unqualified.",
					"Aksi pada detail langganan (Perpanjang, Approve, Activate, Churn) kini tombol tersendiri di header yang membuka modal — tak lagi menumpuk dalam satu kartu \"Tindakan\".",
					"Input \"Nilai deal\" pada form konversi lead kini berformat ribuan otomatis, sejajar dengan form lead & deal.",
					"Keterangan \"nomor disamarkan\" pada form lead dipindah ke ikon info (ⓘ) yang bisa di-tap agar form lebih ringkas.",
					"Form sunting Health Score desa dirapikan — badge read-only \"Status Kesehatan\" dan \"Tren Skor\" dilepas dari form; keduanya tetap dihitung otomatis dan tampil di halaman detail.",
				},
			},
			{
				Title: "Diperbaiki",
				Items: []string{
					"Langganan hasil perpanjangan/upsell kini mengisi tanggal Mulai & Berakhir dengan benar — sebelumnya kedua tanggal tampil kosong pada detail langganan hasil perpanjangan.",
					"Badge \"Jatuh Tempo\" pada daftar perpanjangan kini konsisten dengan isi tab \"Jatuh Tempo\": langganan yang sudah tak aktif (mis. Churned/Expired) tak lagi salah berlabel jatuh tempo.",
				},
			},
		},
	},
	{
		Version: "1.2.0",
		Date:    "2026-09-09",
		Summary: "Langganan multi-paket dengan nilai bersumber dari Quote, keamanan nomor telepon yang bisa diatur per-workspace, plus sejumlah perapian tampilan desa & kontak.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Langganan multi-paket — satu langganan kini bisa memuat beberapa paket sekaligus (mis. paket inti + tambahan), masing-masing dengan MRR/ARR sendiri; laporan pendapatan per-paket mengikuti rincian ini.",
					"Keamanan nomor telepon per-workspace — admin menentukan peran mana yang boleh melihat nomor HP/WhatsApp penuh dan mana yang boleh menyuntingnya, lewat matriks di halaman Peran.",
				},
			},
			{
				Title: "Diubah",
				Items: []string{
					"Nilai langganan kini bersumber dari Quote yang diterima (Accepted), bukan angka manual di Deal — saat quote diterima, nilai & termin-nya otomatis jadi dasar langganan; angka manual di Deal dimaknai ulang sebagai \"perkiraan\" pipeline. Satu deal = tepat satu quote diterima.",
					"Penugasan CSM (utama & cadangan) dipindah ke halaman Customer Success desa, dibuka lewat tombol modal — bukan lagi dari form sunting desa.",
					"Halaman Customer Success desa kini jadi pintu masuk ke Implementation Tracker & Training Schedule, terfilter otomatis ke desa itu.",
					"HP Kontak pada desa jadi field biasa tanpa penyamaran — siapa pun yang boleh membuka desa boleh melihat & menyuntingnya.",
					"Perapian tampilan desa: section Wilayah didahulukan di form, judul \"Kontak\" diperjelas jadi \"Kontak Kantor Desa\", Anggaran APBDes tampil dengan format ribuan, dan kolom yang tak relevan untuk desa (Induk Akun, Teritori, Lintang/Bujur) disembunyikan.",
					"Perapian tampilan kontak: label \"Alamat Surat\" jadi \"Alamat\", \"Jabatan (Kategori)\" jadi \"Kategori Perangkat Desa\", dan baris \"Atasan\" dilepas dari detail.",
				},
			},
			{
				Title: "Diperbaiki",
				Items: []string{
					"Tombol \"Simpan\" pada form sunting kontak tak lagi menghasilkan halaman 404 — form kini mengarah ke alamat yang benar di dalam workspace.",
					"Field \"Website\" desa menerima domain telanjang (mis. www.facebook.com) tanpa harus mengetik https://.",
					"Halaman notifikasi tak lagi berubah jadi tampilan panel developer saat dibuka dari dalam workspace oleh akun platform — tampilannya kini mengikuti workspace asal.",
				},
			},
		},
	},
	{
		Version: "1.1.0",
		Date:    "2026-09-08",
		Summary: "Perbaikan pengalaman onboarding: anggota baru tahu harus menunggu peran dari admin, bukan menyangka aplikasi rusak.",
		Sections: []Section{
			{
				Title: "Diperbaiki",
				Items: []string{
					"Anggota baru yang belum diberi peran CRM kini melihat halaman \"menunggu approval admin\" yang jelas, bukan Beranda dengan semua menu mati tanpa penjelasan.",
				},
			},
		},
	},
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
