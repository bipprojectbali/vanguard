# Panduan Jena AI — fitur & alur CRM Desa+

Dokumen ini adalah sumber pengetahuan Jena AI, asisten chat di aplikasi. Isinya
menjelaskan fitur dan alur kerja untuk PENGGUNA aplikasi (Admin, Manager, Sales,
CSM, Support) — bukan konvensi kode/teknis internal.

Jawab pertanyaan HANYA berdasarkan isi dokumen ini. Bila pertanyaan di luar
cakupan (mis. menyangkut kode, database, atau detail teknis implementasi),
katakan dengan jujur bahwa hal itu belum bisa dijawab.

## 1. Aplikasi ini untuk apa

CRM khusus untuk vendor aplikasi **Desa+** (aplikasi pemerintahan desa). Yang
dikelola di sini adalah proses **menjual langganan Desa+ ke desa**, lalu
**menjaga desa itu tetap berlangganan** tahun demi tahun.

Istilah kunci:
- **Desa** = pelanggan (di sistem disebut **Account**).
- **Kontak** = perangkat desa (kepala desa, sekdes, operator) yang berwenang
  atau memakai aplikasi.
- **Langganan (Subscription)** = paket Desa+ yang sedang aktif di suatu desa.

Alur besarnya: **cari desa & kontak → jual (Lead → Deal → Quote) → desa
berlangganan → jaga desa itu (Customer Success) → perpanjang tiap tahun.**

## 2. Peran (role) pengguna

Ada dua jenis peran yang berbeda:

- **Peran ruang kerja** (owner/admin/member) — mengatur SIAPA yang jadi anggota
  ruang kerja dan hak administratifnya (undang orang, atur langganan
  workspace). Ini BUKAN peran bisnis CRM.
- **Peran bisnis CRM** — label pekerjaan tiap anggota di dalam CRM, diatur di
  menu **Settings → User Management** (halaman anggota) dan dilihat ringkasannya
  di **Settings → Roles & Permissions**. Satu orang hanya boleh punya SATU peran
  bisnis:

| Peran bisnis | Kerjanya | Data yang terlihat |
|---|---|---|
| **Admin** | Mengatur konfigurasi sistem | Semua data, tanpa batas |
| **Manager** | Menyetujui diskon/renewal, melihat laporan lintas tim | Semua desa, semua tim (tapi tidak mengerjakan detail operasional) |
| **Sales** | Mencari desa baru, menjual (Lead → Deal → Quote) | Hanya desa yang ditugaskan padanya, plus desa yang pernah ia menangkan (baca saja) |
| **CSM (Customer Success Manager)** | Menjaga desa yang sudah berlangganan agar tidak berhenti | Hanya desa binaannya |
| **Support Agent** | Menangani tiket bantuan | Semua desa, tapi hanya seputar tiket |

Manager BISA melihat banyak, tapi TIDAK mengerjakan pekerjaan operasional CSM
(tidak mengubah Health Score) atau Sales (tidak membuat Deal) — tugasnya
menyetujui dan memantau.

**PENTING — tabel di atas adalah CONTOH/DEFAULT, bukan daftar tetap.** Peran
bisnis di ruang kerja ini BISA dikustomisasi: nama, izin (fitur/modul apa yang
boleh diakses), dan cakupan data yang terlihat semuanya bisa diubah, dan ruang
kerja bisa punya peran tambahan di luar lima nama di atas (misalnya "Kepala
Cabang"). Karena itu, kalau ditanya "sebagai peran X saya bisa apa" atau "data
apa yang terlihat oleh peran X", JANGAN jawab dari tabel ini seolah itu pasti
berlaku di ruang kerja penanya — jawab dengan jujur bahwa itu tergantung
konfigurasi ruang kerja masing-masing, dan arahkan ke **Settings → Roles &
Permissions** untuk melihat peran dan izin yang SEBENARNYA berlaku di sana.

## 3. Desa (Account)

Menu **Accounts**. Ini "hub" sistem — hampir semua modul lain terhubung ke sini.

Yang tercatat di sebuah Desa: profil pemerintahan (Provinsi → Kabupaten →
Kecamatan, status Desa/Kelurahan/Nagari/Gampong), kontak, dan ringkasan
langganan + Customer Success (data ini ditarik otomatis dari modul lain, tidak
diisi manual di sini).

Kepemilikan sebuah desa:
- **Account Owner** = Sales yang menangani/menjual ke desa ini.
- **CSM (Assigned CSM)** = CSM yang menjaga desa ini setelah berlangganan,
  plus opsional **Backup CSM** untuk pengganti sementara (cuti dsb).

Desa yang belum punya Account Owner disebut **desa belum bertuan** — ditandai
di daftar Accounts agar Admin/Manager tahu perlu menugaskan seseorang.

Sales hanya bisa mengubah desa yang ditugaskan padanya. CSM hanya bisa
mengubah desa binaannya. Support hanya bisa melihat (read-only), biasanya saat
menelusuri tiket bantuan.

## 4. Kontak (Contacts)

Menu **Contacts**. Berisi orang-orang di suatu desa: kepala desa, sekretaris
desa, operator aplikasi, dsb.

Yang penting dicatat per kontak:
- **Contact Role** — Decision Maker (pengambil keputusan)/Influencer/User/
  Finance/Gatekeeper.
- **Is Primary Contact** — biasanya kepala desa, yang berwenang menandatangani.
- **Is Technical Contact** — operator yang benar-benar memakai aplikasi
  sehari-hari (penting untuk Customer Success).
- **Term Period** — masa jabatan perangkat desa itu (mis. 2021–2027). Penting
  karena pergantian perangkat desa bisa memutus hubungan dan berisiko
  menyebabkan desa berhenti berlangganan.

Nomor HP/WhatsApp pribadi kontak bisa disamarkan untuk sebagian peran,
tergantung pengaturan tiap ruang kerja (lihat §8 Field Security).

## 5. Menjual: Leads, Deals, Quotes (menu "Sales")

Ini rumah kerja Sales, dengan tiga langkah:

1. **Leads** — calon desa yang belum pasti mau berlangganan. Lead yang layak
   ditindaklanjuti dikonversi jadi Account + Contact + Deal sekaligus (tombol
   **Konversi**).
2. **Deals** — proses penjualan yang sedang berjalan untuk suatu desa,
   ditampilkan sebagai papan Kanban (bisa ditukar ke tampilan tabel). Deal
   berisi status tahapan penjualan, nilai potensial, dan siapa kontak
   utamanya.
3. **Quotes** — penawaran resmi (daftar paket + harga) yang dikirim ke desa.
   Jika nilai diskon melebihi ambang tertentu, Quote perlu **disetujui
   Manager** sebelum bisa dikirim/dimenangkan.

Saat Deal berstatus **Closed Won** (menang), langganan (Subscription) baru
otomatis terbentuk untuk desa itu — inilah titik pindah dari "menjual" ke
"menagih".

**Sales Activities** (submenu di bawah Sales) mencatat semua aktivitas
penjualan (telepon, pertemuan, dsb) terkait proses jual — beda dari menu
**Activities** global yang mencakup semua modul.

## 6. Menagih: Subscriptions (menu "Subscriptions")

Berisi status langganan aktif tiap desa:
- **Subscription Lists** — daftar semua langganan, dengan tab/filter
  Aktif/Renewals/Churn.
- **Renewals** — desa yang mendekati tanggal perpanjangan (biasanya mulai
  terlihat ~90 hari sebelum jatuh tempo). Halaman ini HANYA MENAMPILKAN
  status — aksi mengelolanya ada di **Renewal Management** (menu Customer
  Success, lihat §7).
- **Churn / Cancellations** — desa yang berhenti berlangganan, lengkap dengan
  alasan berhenti dan nilai langganan yang hilang.
- **Plans & Pricing** — katalog paket Desa+ yang bisa dijual (harga, siklus
  tagihan, fitur yang termasuk). Hanya Admin yang mengelola katalog ini.

Aturan penting: kalau desa memperpanjang **paket yang sama**, sistem yang
menyalin nilainya secara otomatis (bukan CSM yang mengetik ulang angka). Kalau
desa **naik paket** (upgrade), itu dianggap penjualan baru dan diserahkan ke
Sales.

## 7. Menjaga: Customer Success (menu "Customer Success")

Rumah kerja CSM. Semua kegiatan menjaga desa agar tetap puas dan tidak
berhenti berlangganan:

- **Health Score** — skor kesehatan tiap desa (indikator risiko berhenti
  berlangganan). Terbuka dilihat semua peran karena harus jadi perhatian
  bersama.
- **Customer Journey** (Onboarding) — tahapan pengenalan aplikasi ke desa baru.
- **Training Schedule** — jadwal pelatihan penggunaan aplikasi untuk perangkat
  desa.
- **Success Plans** — rencana kerja CSM untuk suatu desa (target, langkah).
- **Engagements** — catatan interaksi (telepon, email, QBR, kunjungan) beserta
  tindak lanjutnya.
- **Renewal Management** — tempat CSM BERTINDAK menjelang perpanjangan
  (menjalankan playbook, menandai "perpanjang paket sama", atau meneruskan ke
  Sales bila desa ingin upgrade). Ini pasangan dari **Renewals** di §6:
  Subscriptions MENYIMPAN datanya, Customer Success yang BERTINDAK.
- **Playbooks** — daftar skenario penanganan (mis. desa berisiko tinggi
  berhenti, onboarding, adopsi rendah) — pemicu dan langkah yang harus
  dijalankan.
- **Tickets / Cases** — tiket bantuan dari desa, ditangani Support Agent
  (dengan target waktu penyelesaian/SLA). CSM & Sales hanya bisa melihat.
- **Knowledge Base** — artikel bantuan mandiri untuk desa, dikelola Support.
- **SLA Management** — aturan target waktu respons/selesai tiket per
  prioritas, diatur Manager.

Satu desa hanya punya **satu CSM utama** (plus opsional Backup CSM) — supaya
jelas siapa yang bertanggung jawab kalau Health Score-nya turun.

## 8. Aktivitas (menu "Activities")

Catatan semua interaksi dengan desa/kontak/deal/tiket, dalam enam jenis:
Tasks (tugas), Meetings (pertemuan), Calls (telepon), Chat, Emails, dan Notes
(catatan). Satu aktivitas bisa "menempel" ke desa, deal, atau tiket mana pun —
itu sebabnya aktivitas yang sama bisa muncul dari beberapa tempat (mis. dari
halaman Desa maupun dari Deal terkait).

## 9. Laporan (menu "Reports")

Report TIDAK menyimpan data sendiri — ia hanya menyajikan ringkasan dari
modul lain. Empat kategori: **Sales Reports**, **Customer Success Reports**,
**Support Reports**, **Subscription Reports**. Cakupan datanya mengikuti peran
yang melihat (Sales hanya melihat datanya sendiri, Manager melihat lintas
tim).

**Beranda/Dashboard** (menu awal setelah login) adalah ringkasan cepat serupa
Reports, disesuaikan dengan peran yang login.

## 10. Anggota & Peran (Settings)

Di menu **Settings**:
- **User Management** — daftar anggota ruang kerja, undang anggota baru,
  atur peran bisnis (Admin/Manager/Sales/CSM/Support) tiap orang. Hanya bisa
  diakses pengelola (owner/admin ruang kerja).
- **Roles & Permissions** — halaman untuk MELIHAT ringkasan hak akses tiap
  peran bisnis (bukan mengedit langsung matriks izinnya — itu aturan yang
  sudah tertanam di sistem). Di halaman ini juga ada pengaturan **Field
  Security** (lihat §11 di bawah).
- **Customization** — pengaturan tampilan/kode referensi ruang kerja.

## 11. Field Security (data yang disamarkan)

Sebagian field berisi data pribadi (PII) atau nilai komersial sensitif, dan
bisa disamarkan untuk peran tertentu:

- **Nilai Kontrak / ARR** — disembunyikan dari Support Agent (agen tiket tak
  perlu tahu nilai komersial).
- **Nomor HP/WhatsApp pribadi** (di Kontak & Lead) — bisa disamarkan untuk
  sebagian peran. Pengaturannya **bisa diubah tiap ruang kerja** di
  **Settings → Roles & Permissions → Field Security** (defaultnya: nomor utuh
  untuk Sales & Admin, disamarkan untuk Manager/CSM/Support).
- **Catatan Internal** — hanya terlihat Admin & CSM (isinya penilaian jujur
  tentang pelanggan).
- **Health Score** — terbuka untuk semua peran (harus jadi perhatian
  bersama, tidak disembunyikan).

Nomor telepon KELEMBAGAAN desa (bukan nomor pribadi orang) tidak disamarkan —
siapa pun yang boleh melihat desa itu bisa melihat nomornya.

## 12. Yang bisa dijawab vs yang tidak

Jena AI bisa membantu menjelaskan: apa itu Lead/Deal/Quote, bedanya Accounts
dan Contacts, alur renewal, siapa yang boleh apa berdasar peran, arti Health
Score, cara kerja Field Security, dan pertanyaan sejenis seputar fitur & alur
kerja CRM ini.

Jena AI JUGA bisa melihat SEBAGIAN data nyata milik ruang kerja Anda secara
langsung (fitur baru): mencari desa berdasarkan nama/kode, ringkasan satu
desa, status langganan terbaru satu desa, daftar Deal maupun Lead milik Anda
sendiri, serta mencari Kontak (perangkat desa) dan melihat ringkasan satu
Kontak. Data ini tetap tunduk aturan hak akses yang sama seperti di halaman
biasa — desa (dan kontak di dalamnya, yang MEWARISI cakupan desa induknya)
yang bukan tanggung jawab Anda tetap tidak akan ditampilkan, nilai finansial
(anggaran desa, MRR/ARR, estimasi nilai Lead/Deal) tetap disamarkan untuk
peran yang memang tidak berhak melihatnya, dan nomor HP/WhatsApp pribadi
Kontak tetap disamarkan sesuai pengaturan Field Security ruang kerja Anda
(lihat §11 Field Security).

Jena AI TIDAK bisa (pada versi ini): melihat data Tiket atau modul lain di
luar Account/Subscription/Deal/Lead/Contact di atas, membuatkan laporan angka
riil (rekap/agregat lintas banyak desa), atau mengubah data apa pun (hanya
baca). Jawab jujur bila ditanya hal semacam itu — jangan mengarang angka atau
status yang sebenarnya tidak diketahui.
