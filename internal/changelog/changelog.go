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

// currentReleases = rilis terbaru (1.5.0 ke atas); rilis lama (1.0.0-1.4.0)
// di changelog_archive.go — dipisah krn ambang File Health (Service/Lainnya
// 300 baris). Releases (di bawah) menggabungkan keduanya.
var currentReleases = []Release{
	{
		Version: "1.12.0",
		Date:    "2026-09-25",
		Summary: "Papan Kanban Deal kini bisa digeser (drag-and-drop) untuk memindah tahap, termasuk pilih banyak deal sekaligus dan pindah massal ke Closed Won/Lost.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Kartu Deal pada papan Kanban kini bisa diseret (drag-and-drop) langsung ke kolom tahap tujuan untuk memindahkannya — tak perlu lagi membuka halaman detail tiap deal satu per satu. Tujuan yang tak valid dari tahap asal otomatis ditolak, mengikuti alur tahap yang sama seperti kontrol pindah tahap di halaman detail.",
					"Bisa memilih beberapa kartu Deal sekaligus (dalam satu tahap yang sama) lalu menyeretnya bersamaan ke tahap lain — termasuk pindah massal ke Closed Won atau Closed Lost, dengan pilihan mengisi satu alasan untuk semua deal atau alasan masing-masing per deal.",
					"Kolom pada papan Kanban kini bisa dilebarkan/diciutkan agar lebih mudah fokus pada satu tahap saat deal-nya banyak.",
				},
			},
		},
	},
	{
		Version: "1.11.0",
		Date:    "2026-09-24",
		Summary: "Undangan anggota kini otomatis diterapkan saat login (tanpa perlu klik tautan), dan admin bisa memberi peran tertentu akses ke halaman Anggota.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Anggota yang sudah didaftarkan admin lewat form \"Undang\" kini langsung masuk dengan peran yang ditentukan begitu ia login atau mendaftar — tak perlu lagi mengklik tautan undangan.",
					"Form Undang kini juga meminta Jenis Anggota (internal/eksternal), dan pilihan Peran CRM otomatis menyesuaikan jenis yang dipilih.",
					"Halaman Peran & Perizinan kini punya izin baru \"User management\" — admin bisa memberi peran tertentu akses Lihat atau Kelola ke halaman Anggota, dibatasi hanya boleh melihat/mengelola anggota berjenis internal, eksternal, atau keduanya sesuai yang diatur.",
					"Jena AI kini bisa menjawab pertanyaan tentang deal/lead milik anggota tim lain (mis. \"deal yang dibuat oleh Budi\") untuk peran yang memang berwenang melihat semua data — sebelumnya selalu dianggap menanyakan data milik sendiri saja.",
				},
			},
		},
	},
	{
		Version: "1.10.0",
		Date:    "2026-09-23",
		Summary: "Perapian matriks Peran & Perizinan: keterangan Renewals/Churn kini kondisional, opsi izin yang tak berefek disembunyikan, dan tampilan kolom Akses lebih konsisten.",
		Sections: []Section{
			{
				Title: "Diubah",
				Items: []string{
					"Keterangan \"butuh izin langganan berstatus aktif juga\" pada baris Renewals & Churn di matriks Peran kini hanya muncul saat kombinasi izinnya benar-benar bermasalah, tak lagi selalu tampil untuk semua peran.",
					"Opsi \"Kelola\" pada 7 modul CRM (ringkasan performa, langganan berstatus aktif, aktivitas, dan 4 halaman laporan) yang sebenarnya tak berefek kini disembunyikan dari matriks Peran — mencegah admin memberi izin yang kelihatannya aktif tapi tak melakukan apa-apa.",
					"Peran kustom yang diberi kemampuan setara Admin/Manager/Sales/CSM kini bisa melihat nilai kontrak/MRR/anggaran sesuai kapabilitas yang diberikan — sebelumnya dibatasi hanya untuk 4 nama peran baku, peran hasil rename/tambahan tenant selalu dianggap tak berhak walau sudah diberi izinnya.",
				},
			},
			{
				Title: "Diperbaiki",
				Items: []string{
					"Lebar kolom pilihan akses pada matriks Peran tak lagi berubah-ubah saat keterangan tambahan muncul/hilang.",
					"Kotak centang \"Lihat Nilai Kontrak\" di editor Peran kini bisa dicentang untuk peran mana pun yang memang berhak melihat nilai kontrak/MRR — sebelumnya cuma bisa dicentang kalau peran itu juga punya akses ke halaman Langganan, walau haknya sendiri sudah berlaku di halaman lain (Leads, Akun, dsb).",
					"Formulir tambah Lead dan Akun baru kini lebih cepat dimuat di jaringan lambat — daftar wilayah yang ikut dikirim ke halaman sempat membengkak tanpa perlu.",
				},
			},
		},
	},
	{
		Version: "1.9.0",
		Date:    "2026-09-21",
		Summary: "Asisten chat \"Jena AI\" hadir sebagai uji coba terbatas, dan halaman Customer Success kini bisa menyinkronkan data pemakaian desa langsung dari Desa+.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Asisten chat \"Jena AI\" (uji coba) — tombol mengambang di pojok kiri-bawah pada halaman workspace untuk bertanya cara pakai aplikasi, sekaligus mencari & merangkum data Desa, Langganan, Deal, dan Lead milik sendiri. Jawaban data selalu mengikuti hak akses dan penyamaran (ARR/HP) yang sama seperti halaman biasa — tak pernah membocorkan data di luar cakupan penanya.",
					"Tombol \"Sinkron dari Desa+\" pada halaman Customer Success (khusus desa berlangganan aktif) — menarik otomatis 3 kolom pemakaian produk (Aktivitas Terakhir, Jumlah Pengguna Aktif, Fitur yang Paling Sering Dipakai) dari sistem Desa+, menggantikan pengisian manual. Kolom yang sudah tersinkron ditandai sebagai data otomatis dan terkunci dari suntingan manual.",
				},
			},
		},
	},
	{
		Version: "1.8.0",
		Date:    "2026-09-18",
		Summary: "Lead kini bisa diimpor massal lewat CSV, halaman Peran & Perizinan dirombak jadi lebih ringkas dan reaktif, gerbang akses Laporan kini per-domain, dan perbaikan kebocoran ARR pada halaman Desa.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Impor Lead massal lewat unggah CSV — pola sama dengan impor Desa dan Kontak: alur dua langkah (unggah → pratinjau seluruh baris → Konfirmasi Impor). Kode Kecamatan wajib diisi per baris; satu baris tak valid membatalkan seluruh file, bukan hanya baris itu.",
				},
			},
			{
				Title: "Diubah",
				Items: []string{
					"Halaman Peran & Perizinan dirombak jadi lebih ringkas — matriks perizinan dan aturan kepemilikan data kini dibuka lewat tombol \"Lihat\" per peran dalam satu jendela, bukan lagi tampil terpisah memanjang. Pengaturan keamanan nomor HP/WhatsApp dan tampilan Nilai Kontrak/MRR kini berada di halaman detail tiap peran, dan otomatis menyesuaikan begitu akses Kontak/Leads peran itu diubah.",
					"Kotak centang \"Setujui\" dan \"Lihat Nilai (ARR)\" pada matriks Peran kini otomatis nonaktif saat level akses baris itu diset \"Tak ada\" — mencegah admin menyimpan izin yang saling bertentangan.",
					"Kolom \"Setujui\" pada baris Deal di matriks Peran disembunyikan sementara (belum ada alur persetujuan Deal yang memakainya); tombol Setujui/Tolak untuk perpanjangan langganan kini dikelompokkan di bawah izin \"Renewals\", sejalan dengan halaman tempat tombolnya benar-benar muncul.",
					"Gerbang akses halaman Laporan kini per-domain (Sales, Langganan, Customer Success, Support) — admin bisa memberi akses laporan Sales saja tanpa otomatis membuka laporan domain lain. Menu sidebar yang seluruh isinya tak bisa diakses kini disembunyikan total, tak lagi tampil redup.",
				},
			},
			{
				Title: "Diperbaiki",
				Items: []string{
					"ARR pada kartu \"Ringkasan Langganan\" di halaman Desa kini disamarkan mengikuti hak akses yang sama dengan halaman Langganan — sebelumnya bisa tampil utuh untuk peran yang seharusnya hanya melihat versi tersamar.",
					"Tombol pencarian cepat kode wilayah yang sempat tak muncul di halaman impor Kontak kini tampil, konsisten dengan halaman impor Desa dan Lead.",
				},
			},
		},
	},
	{
		Version: "1.7.0",
		Date:    "2026-09-15",
		Summary: "Desa dan Kontak kini bisa diimpor massal lewat CSV, pencarian cepat kode wilayah Kemendagri hadir di banyak halaman, notifikasi otomatis mengingatkan perpanjangan langganan, sort kolom melengkapi Sales Activities & Activity Log, dan dropdown Kontak di form Aktivitas kini mengikuti Target yang dipilih.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Impor Desa massal lewat unggah CSV — alur dua langkah (unggah → pratinjau seluruh baris → Konfirmasi Impor), identifikasi tiap baris pakai kode wilayah Kemendagri. Desa yang kodenya sudah terdaftar (duplikat) ditandai jelas dengan tautan ke data yang sudah ada; kepemilikan desa opsional diisi lewat email anggota tim. Satu baris tak valid membatalkan seluruh file.",
					"Impor Kontak massal lewat unggah CSV — pola sama dengan impor Desa: alur dua langkah (unggah → pratinjau seluruh baris → Konfirmasi Impor). Kode Desa wajib diisi per baris; satu baris tak valid (desa tak dikenal/di luar akses, atau lebih dari satu kontak utama per desa) membatalkan seluruh file, bukan hanya baris itu.",
					"Modal pencarian cepat kode wilayah Kemendagri (Kecamatan & Desa) — bisa dibuka dari banyak halaman (form Akun/Lead/Kontak, halaman impor CSV) tanpa kehilangan isian yang sedang diketik di halaman itu. Ketik nama (minimal 3 huruf) atau kode persis untuk hasil instan, plus tab \"Wilayah\" untuk menelusuri berjenjang Provinsi → Kabupaten/Kota → Kecamatan.",
					"Notifikasi otomatis mengingatkan perpanjangan langganan — pemilik langganan kini mendapat notifikasi saat masa berlaku tersisa 30, 14, 7 hari, dan tepat pada hari-H, sehingga follow-up perpanjangan tak lagi mengandalkan pengecekan manual berkala.",
					"Sort kolom (klik header untuk urutkan naik/turun) kini menjangkau Sales Activities dan Activity Log, termasuk kolom Tanggal — melengkapi seluruh modul lain yang sudah bisa diurutkan. Kolom yang bisa disortir namun belum aktif kini menampilkan ikon \"⇅\" di semua tabel agar terlihat jelas bisa diklik.",
				},
			},
			{
				Title: "Diubah",
				Items: []string{
					"Dropdown Kontak pada form Tambah/Ubah Aktivitas (jenis Panggilan/Chat) kini mengikuti Target yang dipilih: memilih Akun menampilkan kontak desa itu saja, memilih Deal atau Kontak otomatis memilih kontak terkait, dan memilih Lead menampilkan info kontak lead langsung (nama/HP/WhatsApp/email) karena Lead belum tercatat sebagai Kontak.",
				},
			},
		},
	},
	{
		Version: "1.6.0",
		Date:    "2026-09-14",
		Summary: "Sort kolom kini tersedia di hampir semua tabel CRM, Lead bisa dicatat aktivitasnya, Beranda dapat grafik ringkas per-modul, perubahan tahap Deal jadi berurutan, detail Langganan didesain ulang, dan badge Perpanjangan diperbaiki.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Sort kolom (klik header untuk urutkan naik/turun) kini tersedia di tabel Desa (Akun), Kontak, Deal, Quote, Perpanjangan, Tiket, dan Health Score — melengkapi Langganan dan Leads yang sudah bisa diurutkan sebelumnya.",
					"Lead kini bisa dicatat riwayat aktivitasnya (telepon, pertemuan, dll) seperti Deal dan Kontak, lengkap dengan pencarian lead lewat kode saat memilih target aktivitas.",
					"Halaman Beranda kini menampilkan grafik ringkas di bawah kartu KPI untuk Sales, Langganan, Customer Success, dan Support — menyesuaikan peran pengguna yang login.",
				},
			},
			{
				Title: "Diubah",
				Items: []string{
					"Halaman detail Langganan didesain ulang mengikuti pola kartu 2-kolom — Identitas & Langganan, Status & Lifecycle, Financials, Renewal, serta System & Audit — kini juga menampilkan ringkasan Health Score/Onboarding dan tautan ke deal sumbernya.",
					"Ubah tahap Deal kini hanya bisa maju satu langkah berurutan, tak bisa lompat tahap — mencegah kesalahan input dan menjaga jejak proses penjualan tetap rapi.",
				},
			},
			{
				Title: "Diperbaiki",
				Items: []string{
					"Badge \"Diperpanjang\" serta KPI terkait pada halaman Perpanjangan tak lagi salah tampil untuk langganan yang sudah churn/berhenti — status daur hidup kini diprioritaskan atas riwayat perpanjangan lama.",
					"Menambah aktivitas dari halaman Lead kini mengarahkan tombol kembali & sorotan menu sidebar dengan benar — sebelumnya salah mengarah ke menu lain.",
				},
			},
		},
	},
	{
		Version: "1.5.0",
		Date:    "2026-09-14",
		Summary: "Toast notifikasi mengambang menyeragamkan seluruh pesan aksi CRM, Kontak kini punya kode sistem sendiri, dan Daftar Langganan serta Leads bisa diurutkan lewat klik header kolom.",
		Sections: []Section{
			{
				Title: "Baru",
				Items: []string{
					"Kontak kini punya kode entitas sistem sendiri (format KON-001, KON-002, …), tampil di tabel & detail kontak — sebelumnya hanya Lead dan Desa yang punya kode semacam ini.",
					"Header kolom pada Daftar Langganan dan Leads kini bisa diklik untuk mengurutkan data naik/turun per kolom — Desa, Paket, MRR, Masa Berlaku, Renewal Date, CSM untuk Langganan; Kode, Lead, Sumber, Status, Rating, Estimasi, Pemilik untuk Leads — lengkap dengan indikator arah panah.",
				},
			},
			{
				Title: "Diubah",
				Items: []string{
					"Seluruh notifikasi hasil aksi (tambah/ubah/hapus/approve) di semua modul kini tampil sebagai notifikasi mengambang seragam, menggantikan pesan sebaris yang polanya berbeda-beda di tiap modul; beberapa modul yang sebelumnya sama sekali tak punya pesan sukses (Desa, Kontak, Langganan, Kebijakan SLA, Knowledge Base) kini ikut mendapat notifikasi.",
				},
			},
		},
	},
}

// Releases = seluruh rilis, TERBARU DI INDEKS 0. Sumber tunggal modal + badge.
// Nomor versi teratas menjadi acuan badge "ada pembaruan".
var Releases = append(currentReleases, archivedReleases...)

// Current mengembalikan nomor versi rilis terbaru (Releases[0].Version); ""
// bila belum ada rilis. Dipakai handler untuk mengisi ShellData.ChangelogVersion
// (acuan badge).
func Current() string {
	if len(Releases) == 0 {
		return ""
	}
	return Releases[0].Version
}
