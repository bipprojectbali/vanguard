# CRM Desa+ — sistem, role, dan hak akses

Status: **Rancangan disepakati** (2026-08-06) — hasil diskusi atas spec `crm.pdf`
(22 halaman, 9 modul). Dokumen ini acuan sebelum menggambar di Penpot dan
sebelum menulis kode.

Yang TIDAK ada di sini: rincian field per objek (sudah di spec) dan tata letak
layar (ada di file Penpot `vanguard`, page `wireframe`).

## 1. Sistem apa ini

**CRM vertikal untuk vendor SaaS pemerintahan desa** — mengelola penjualan dan
retensi langganan aplikasi **Desa+** beserta paket-paket di dalamnya.

Bukan CRM umum. Tiga hal di spec yang mengunci ini:

- Objek `Account` bernama **Villages**, dengan `Village Status` (Desa / Kelurahan
  / Nagari / Gampong), hierarki Provinsi → Kabupaten → Kecamatan, dan grup field
  **Government Profile**.
- `Contacts` punya grup **Role & Authority** — bukan sekadar jabatan, melainkan
  siapa yang berwenang menandatangani.
- Modul Subscriptions + Customer Success yang berat — ciri bisnis langganan,
  bukan penjualan putus.

Ringkasnya: **pelanggan = desa · pembeli = perangkat desa · produk = langganan
Desa+ · yang dijaga = perpanjangan kontrak tiap tahun.**

## 2. Cara kerja sistem

Sembilan modul spec adalah satu siklus hidup pelanggan yang berputar:

```
  MENCARI    2 Accounts (desa mana) · 3 Contacts (siapa yang berwenang)
     ↓
  MENJUAL    4.1 Lead → 4.2 Deal → 4.3 Quote → menang
     ↓
  MENAGIH    5.1 Active · 5.3 Plan & harga · 5.2 Renewal · 5.4 Churn
     ↓
  MENJAGA    6 Customer Success — health score → adoption → intervensi
     ↓
             kembali ke 5.2 (perpanjang) atau 5.4 (berhenti)
```

Tiga lapis penopang:

- **7 Activities** — jejak semua interaksi (task, call, chat, email, notes),
  polymorphic: menempel ke desa, deal, maupun tiket mana pun.
- **1 Dashboard** & **8 Reports** — cermin. Spec menyatakannya eksplisit:
  *"Report bukan objek data — ia menyajikan data objek menu lain."*
- **9 Settings** — aturan main (user, hak akses, kustomisasi, otomasi, integrasi).

### Dua hal yang memberi sistem ini karakter

**Pola dua-rumah.** Halaman terakhir spec menegaskan: `5.2 Renewals` (SUMBER) →
*surface* → `6.6 Renewal Management` (AKSI), dengan kalimat **"Subscriptions
MENYIMPAN · Customer Success BERTINDAK."** Data yang sama muncul di dua tempat
dengan pertanyaan berbeda — di 5.2 *"kapan jatuh tempo dan berapa nilainya"*, di
6.6 *"desa ini kritis, siapa yang dihubungi hari ini"*. Pola yang sama berulang
lewat grup field `[rollup dari Modul 5 — read only]` dan `[rollup dari Modul 6]`
di Accounts.

**Support ada DI DALAM Customer Success** (6.9–6.12), bukan modul terpisah.
Konsekuensinya masuk akal: tiket menumpuk menurunkan health score, dan health
score menentukan risiko churn. Memisahkan support memutus rantai itu.

## 3. Role

Lima role internal. Empat pertama berasal dari spec §9.2; **Manager** adalah
tambahan hasil diskusi.

| Role | Inti pekerjaan | Cakupan data |
|---|---|---|
| **Admin** | Konfigurasi sistem — bukan urusan bisnis | Semua, tanpa batas |
| **Manager** | Menyetujui & melihat lintas-tim | Semua desa, semua tim |
| **Sales** | Cari desa baru → deal → quote → menang | Hanya desa yang **ditugaskan** padanya |
| **CSM** | Jaga desa yang sudah jadi pelanggan | Hanya desa binaannya |
| **Support Agent** | Selesaikan tiket sesuai SLA | Semua desa, **tapi hanya objek tiket** |

**Kenapa Manager ada.** Spec §9.4 menyebut *Approval Process (mis. diskon di atas
batas)* — persetujuan mengandaikan atasan. Board Reports 8.1 juga punya filter
"Tim: Semua" dan tabel kinerja per-sales, yaitu pandangan lintas-cakupan yang
justru dilarang bagi Sales biasa. Dipilih **satu role lintas-fungsi**, bukan
Sales Manager / CS Lead / Support Lead terpisah: satu role menutup kebutuhan
approval + laporan lintas-tim tanpa menggandakan tiap fungsi dan tanpa
memanjangkan matriks izin jadi tujuh kolom.

**Kenapa Admin tidak merangkap.** Wewenang teknis (mengubah konfigurasi sistem)
dan wewenang bisnis (menyetujui harga) adalah dua hal berbeda. Mencampurnya
membuat orang yang seharusnya hanya mengurus sistem jadi penentu nilai kontrak.

### Role bisnis adalah sumbu KETIGA, terpisah dari platform & tenant

Template sudah punya dua sumbu role (`internal/authz/role.go`). Role bisnis di
dokumen ini adalah sumbu ketiga — **bukan** perpanjangan dari keduanya.

| Sumbu | Role | Pertanyaan | Ditentukan di |
|---|---|---|---|
| **Platform** | `staff` < `super_admin` | siapa menjalankan aplikasinya | `SUPER_ADMIN_EMAILS` (env) · `platform_staff` |
| **Tenant** | `member` < `admin` < `owner` | siapa mengelola ruang kerja | tabel `memberships` |
| **Bisnis** | Admin · Manager · Sales · CSM · Support | siapa mengerjakan apa di CRM | Settings 9.1 |

Ketiganya **bersarang**, bukan sejajar: platform memuat tenant, tenant memuat
orang, dan tiap orang membawa satu role bisnis. Dua lapis pertama adalah WADAH
(dibuat, dinamai, dihapus); role bisnis adalah LABEL pada orang — tak pernah ada
tindakan "buat CSM baru", yang ada "tambah orang, beri label CSM".

Pemeriksaan berjalan dari luar ke dalam, dan gagal di lapis luar berarti lapis
dalam tak pernah dijalankan: platform (bypass RLS?) → tenant (anggota workspace
ini? tidak → 404) → bisnis (sebagai CSM boleh apa di sini?).

**Pewarisan sengaja timpang.** Platform → tenant MEWARISI (`g, staff, owner` di
`policy.csv`, agar operator bisa membantu saat impersonate). Tenant → bisnis
**TIDAK**, dan tak boleh: seorang `owner` bisa membuat akun dan mengatur role,
tapi ia bukan Sales, bukan CSM, dan **tidak memegang satu desa pun**. Kalau owner
otomatis mewarisi izin bisnis, ia jadi pemilik bayangan seluruh desa, muncul di
laporan kinerja CSM, dan Field-Level Security tak berarti baginya — wewenang
*mengatur* berubah diam-diam jadi wewenang *mengerjakan*. Paralel dengan §8.3:
melihat luas ≠ mengerjakan segalanya.

**super_admin TIDAK ada di matriks §4** karena ia bukan baris di tangga yang
sama. Sales bisa naik jadi Manager; tak seorang pun "naik" jadi super_admin — itu
env-only dan immutable. Konsekuensi yang harus disadari: lewat pewarisan
platform→tenant, **super_admin dapat melihat data desa**, termasuk field yang di
§5 disembunyikan bahkan dari Manager. Itu keputusan sadar template (tanpanya
mustahil menolong pengguna yang macet), tapi berarti **Field-Level Security
adalah aturan peran bisnis, bukan tembok keamanan terhadap operator platform**.
Aksesnya wajib terekam audit — lebih keras daripada Manager, sebab cakupannya
lintas-workspace.

### Tidak boleh rangkap role bisnis

Satu orang = **satu** role bisnis per workspace. Perangkapan (mis. Sales
sekaligus CSM) ditolak.

**Why:** izin yang digabung membuat cakupan data kabur — pertanyaan "desa ini
binaan saya atau sasaran jualan saya" tak punya jawaban tunggal, padahal §8.1 &
§8.2 justru dibangun agar pertanggungjawaban tiap desa jelas.

**How to apply:** di Settings 9.1 role bisnis dipilih sebagai **satu nilai**
(dropdown/radio), bukan centang jamak. Perangkapan di lapangan diselesaikan
dengan akun terpisah, bukan dengan melonggarkan aturan ini.

**Jalan keluar bila kelak terasa terlalu kaku** (belum diputuskan): tambahkan
**role gabungan bernama sendiri** — mis. `Sales-CSM` — dengan cakupan datanya
ditetapkan eksplisit, BUKAN mengizinkan rangkap. Bedanya menentukan: role
gabungan tetap satu nilai, sehingga "desa ini binaan atau sasaran jualan saya"
punya satu jawaban yang tertulis di definisi rolenya. Mengizinkan rangkap
membuat jawabannya bergantung pada gabungan izin yang kebetulan menempel — dan
itu berbeda tiap orang, tak bisa ditinjau, serta melemahkan
pertanggungjawaban per-desa di §8.1 & §8.2.

### Role bisnis disimpan per-KEANGGOTAAN, bukan per-user

Implikasi rancangan (belum diimplementasikan): kolomnya menempel pada
`memberships` (user × tenant), bukan `users` — sejalan dengan
[0003](../decisions/0003-membership-multi-workspace.md). Satu orang bisa jadi
Manager di satu workspace dan CSM di workspace lain; menyimpannya di `users`
mengunci dia pada satu role untuk semua workspace.

### Dua sumbu izin — jangan digabung

Spec §9.2 sudah memisahkan keduanya, dan pemisahan itu harus dipertahankan:

| Sumbu | Pertanyaan | Contoh |
|---|---|---|
| **Permission Set** | boleh melakukan **apa** | Sales: create deal ✓, delete ✕ |
| **Ownership Rule** | atas data **siapa** | Sales: desa yang ditugaskan padanya |

Tanpa pemisahan ini, "Sales boleh lihat deal" jadi ambigu — deal siapa?

### Cakupan Sales = penugasan manual per desa

Bukan wilayah administratif. Admin/Manager menetapkan siapa memegang desa mana.

**Konsekuensi yang harus ditangani, bukan diabaikan:** desa baru **tidak punya
pemilik** sampai seseorang menugaskannya. Karena itu wajib ada tempat yang
menampilkan **"desa belum bertuan"** — tanpa itu, desa hilang diam-diam dari
radar semua orang, dan kegagalannya senyap (tak ada error, hanya desa yang tak
pernah dihubungi siapa pun).

> **Gap ini SUDAH dibereskan** (board disentuh pada 2026-08-10). Board Penpot
> `Settings — 9.2 Roles & Permissions` kini konsisten dengan model penugasan
> manual: Record Ownership Rule Sales = `Assigned` (bukan `Territory`), deskripsi
> Sales = *"Desa yang ditugaskan padanya + desa asal (baca)"*, legenda =
> *"Hanya desa tugasnya / binaannya"*, dan config FLS = *"per penugasan desa"*.
> Tidak ada lagi kata "territory"/"wilayah" di seluruh board (147 teks disisir).

## 4. Matriks akses per modul

Notasi: **✓** penuh · **◐** hanya miliknya/binaannya · **👁** lihat saja · **✕** tidak ada

Untuk Sales, **◐** berarti desa yang ditugaskan padanya; ia juga **👁** atas desa
tempat ia tercatat sebagai Sales asal (§8.2). Bagi Manager, **✓** berarti seluruh
cakupan data tapi tunduk pada Field-Level Security kecuali ARR (§8.3).

| Modul | Admin | Manager | Sales | CSM | Support |
|---|---|---|---|---|---|
| 1 Dashboard | ✓ | ✓ semua tim | ◐ sales | ◐ CS | ◐ support |
| 2 Accounts (Villages) | ✓ | ✓ | ◐ ubah | ◐ ubah | 👁 saat buka tiket |
| 3 Contacts | ✓ | ✓ | ◐ ubah | ◐ ubah | 👁 nomor tersamar |
| 4.1 Leads | ✓ | ✓ | ◐ ✓ | ✕ | ✕ |
| 4.2 Deals | ✓ | ✓ approve | ◐ ✓ | 👁 desa binaan | ✕ |
| 4.3 Quotes | ✓ | ✓ approve diskon | ◐ buat | ✕ | ✕ |
| 4.4 Sales Activity Log | ✓ | ✓ semua sales | ◐ miliknya | ✕ | ✕ |
| 5.1 Active Subscriptions | ✓ | ✓ | 👁 | 👁 | ✕ |
| 5.2 Renewals | ✓ | ✓ | 👁 | 👁 *(bertindak lewat 6.6)* | ✕ |
| 5.3 Plans & Pricing | ✓ | 👁 | 👁 | 👁 | ✕ |
| 5.4 Churn / Cancellations | ✓ | ✓ | 👁 | ◐ isi alasan | ✕ |
| 6.1 Health Score | ✓ | ✓ | 👁 desanya | ◐ ✓ | 👁 |
| 6.2 Journey / Onboarding | ✓ | ✓ | 👁 | ◐ ✓ | ✕ |
| 6.3 Success Plans | ✓ | ✓ | ✕ | ◐ ✓ | ✕ |
| 6.4 Product Adoption | ✓ | ✓ | ✕ | ◐ ✓ | ✕ |
| 6.5 Engagements | ✓ | ✓ | ✕ | ◐ ✓ | ✕ |
| 6.6 Renewal Management | ✓ | ✓ approve | 👁 + upsell | ◐ ✓ **pemilik** | ✕ |
| 6.7 Playbooks | ✓ | ✓ | ✕ | ◐ ✓ | ✕ |
| 6.8 Voice of Customer | ✓ | ✓ | ✕ | ◐ ✓ | ✕ |
| 6.9 Tickets / Cases | ✓ | ✓ | 👁 desanya | 👁 desa binaan | ✓ semua |
| 6.10 Knowledge Base | ✓ | ✓ | 👁 | 👁 | ✓ tulis |
| 6.11 SLA Management | ✓ | ✓ atur | ✕ | 👁 | 👁 |
| 7 Activities | ✓ | ✓ | ◐ | ◐ | ◐ |
| 8 Reports | ✓ | ✓ lintas-tim | ◐ sales | ◐ CS | ◐ support |
| 9 Settings | ✓ | ✕ | ✕ | ✕ | ✕ |

**6.12 Customer Portal SENGAJA tidak ada di matriks** — lihat §7.

## 5. Field-Level Security

Sesuai spec §9.2 dan sudah tergambar di board 9.2:

| Field | Aturan | Alasan |
|---|---|---|
| Nilai Kontrak / ARR | Disembunyikan dari Support Agent | Agen tiket tak perlu tahu nilai komersial |
| Nomor HP Kontak | Utuh untuk Sales & Admin; tersamar untuk Manager, CSM, Support | Batasi sebaran PII; Admin = pengelola workspace yang perlu verifikasi/perbaiki kontak |
| Catatan Internal | Hanya Admin & CSM | Isinya penilaian jujur tentang pelanggan |
| Health Score | Terbuka untuk semua role | Justru harus dilihat bersama |

Aturan ini **juga mengikat Manager**, kecuali Nilai Kontrak / ARR — lihat §8.3.

## 6. Alur renewal — CSM memimpin, Sales untuk upsell

```
  90 hari sebelum jatuh tempo
        │
  5.2 Renewals memunculkan sinyal
        │
        ▼
  6.6 Renewal Management  ←── CSM PEMILIK
  • lihat health score desa
  • jalankan playbook bila kritis
        │
   ┌────┴────┐
   │         │
paket sama  naik paket
   │         │
CSM selesai  Sales masuk (upsell)
                │
          diskon > batas?
                │
          Manager approve
```

**Aturan yang membuat ini bekerja: CSM tidak pernah mengubah angka kontrak.** Ia
menandai "perpanjang paket sama", dan sistem yang menyalin nilainya. Begitu paket
berubah, Sales yang memegang — sebab itu penjualan baru, bukan perpanjangan.

Ini konsisten dengan pola dua-rumah di §2: CSM **bertindak** di 6.6, sementara
angkanya tetap **disimpan** di 5.x.

## 7. Batas lingkup

**6.12 Customer Portal ditunda.** Spec sendiri menandainya OPSIONAL. Ia bukan
varian Support Agent melainkan **role EKSTERNAL** — perangkat desa, bukan tim
internal, yang hanya boleh melihat datanya sendiri. Menggambarnya berarti
merancang seluruh sisi luar (login desa, isolasi data antar-desa) yang cakupannya
setara aplikasi kedua. Dikerjakan setelah sisi internal utuh.

## 8. Kepemilikan desa

### 8.1 Satu desa, satu CSM

**Satu pemilik**, plus satu **CSM cadangan** opsional untuk cuti/pengalihan.

Alasannya pertanggungjawaban: bila health score sebuah desa jatuh, harus jelas
itu pekerjaan siapa. Kepemilikan bersama membuatnya jadi pekerjaan tak seorang
pun — dan itu justru menimpa desa yang paling butuh perhatian.

Alasan teknis yang menguatkan: dengan pemilik tunggal, **"desa tanpa CSM" adalah
keadaan yang bisa dideteksi**. Dengan relasi banyak-ke-banyak, nol-CSM tak bisa
dibedakan dari sedang-dialihkan — dan kegagalan senyap seperti itu tak pernah
ketahuan sampai desanya berhenti berlangganan.

CSM cadangan boleh **membaca dan bertindak**, tapi TIDAK memindahkan
kepemilikan. Kalau ia bisa, pengalihan diam-diam jadi mungkin dan jejak
tanggung jawabnya kabur.

Bentuk field di Accounts: `Assigned CSM` (lookup tunggal) + `Backup CSM`
(lookup tunggal, nullable). Bukan relasi banyak-ke-banyak.

### 8.2 Sales asal tetap melihat desanya — baca saja

Sales yang menutup deal tetap terhubung sebagai **Sales asal** dengan akses
baca, meski kepemilikan operasional berpindah ke CSM.

Tiga alasan:

- Alur renewal (§6) menempatkan Sales untuk upsell. Ia mustahil melakukannya
  pada desa yang tak bisa ia lihat.
- Orang yang meneken kontrak mengenal kepala desanya. Memutus itu membuang modal
  hubungan yang sudah dibayar.
- Tanpa ini `4.4 Sales Activity Log` jadi timpang: aktivitas masa lalu Sales di
  desa itu tetap tercatat, tapi desanya sendiri tak terlihat olehnya.

Batasnya tegas: Sales asal boleh mengubah objek **komersial** (deal, quote baru),
TIDAK objek CS (health score, success plan, playbook).

Risiko yang diterima secara sadar: cakupan data Sales melebar seiring waktu —
sales lama melihat makin banyak desa. Diterima karena aksesnya baca-saja dan
Field-Level Security (§5) tetap berlaku.

Bentuk field: `Sales Pemilik Asal` bertahan setelah konversi. Aturan cakupan
Sales jadi: desa yang **ditugaskan padanya** (tulis) **ATAU** desa tempat ia
tercatat sebagai Sales asal (baca).

### 8.3 Manager melihat semua — menyetujui, bukan mengerjakan

Karena dipilih **satu Manager lintas-fungsi** (§3), "hanya timnya" tak punya
arti: timnya adalah semua orang. Yang perlu dipertajam adalah jenis aksesnya,
bukan luasnya.

| Manager **boleh** | Manager **tidak** |
|---|---|
| Melihat seluruh desa & seluruh tim | Mengubah health score |
| Menyetujui diskon & renewal | Menugaskan ulang tiket |
| Melihat semua laporan lintas-tim | Mengubah konfigurasi sistem (itu Admin) |

Dua konsekuensi yang diambil sekalian:

- **Field-Level Security tetap berlaku bagi Manager**, KECUALI Nilai Kontrak /
  ARR — ia membutuhkannya untuk menyetujui diskon. Nomor HP kontak tetap
  tersamar, catatan internal tetap tertutup. **Melihat luas tidak sama dengan
  melihat segalanya.**
- Tindakan Manager **wajib terekam di audit log**. Bukan karena curiga, tapi
  karena wewenang seluas itu harus punya jawaban untuk "siapa & kapan".

## 8b. Modul 2 Accounts (Villages) — catatan rancangan

Hasil diskusi atas spec modul 2 (hal. 2–3). Accounts adalah **hub** sistem:
grup 2.D (Subscription) & 2.E (Customer Success) bertanda `rollup — read only`,
menarik ringkasan dari Modul 5 & 6 — perwujudan pola dua-rumah (§2).

**Submenu 2.1–2.3 = view/filter objek yang SAMA** (kata spec sendiri), bukan tiga
objek berbeda:
- **2.1 All Villages** — semua desa (cakupan mengikuti role: Sales lihat yang
  ditugaskan, Manager semua).
- **2.2 My Villages** — BUKAN board terpisah, melainkan **filter "Punya Saya"** di
  dalam 2.1. Board POV Sales/CSM otomatis jadi perwujudan "My Villages"-nya.
  Menggemakan pola Dashboard: satu objek, beberapa pandangan.
- **2.3 Territory / Region** — pengelompokan per wilayah kerja internal.

**Dua field kepemilikan cocok dengan §8** — spec memberi keduanya terpisah:
- `Account Owner` (2.A, Lookup User) = **Sales asal** (§8.2).
- `CSM (Success Manager)` (2.E, Lookup User) = **Assigned CSM** (§8.1).

**Desa yatim (§8.1) ditempatkan di sini:** filter **"Belum ada Owner"** + badge
merah di baris desa yang `Account Owner`-nya kosong, di dalam 2.1 All Villages.
Terlihat oleh Manager/Admin yang berwenang menugaskan — menutup celah "desa
hilang senyap" yang jadi alasan §8.1 memilih pemilik tunggal.

**Field sensitif di modul ini (tunduk §5):**
- `Contact Phone` (2.C) — nomor HP, **tersamar untuk non-Sales** (termasuk
  Support saat membuka tiket, termasuk Manager).
- `Village Budget / APBDes` (2.C) — indikator daya beli, sifatnya seperti Nilai
  Kontrak: **dibatasi dari Support** (disembunyikan/tersamar). Diputuskan
  2026-08-07.

**Penggambaran: 3 POV** (Sales · CSM · Support) untuk List & Detail — Accounts
adalah tempat beda-akses paling kentara:
- **Sales** → 2.1 terfilter desa tugasnya, bisa edit; badge desa-yatim tak tampak
  (bukan wewenangnya menugaskan).
- **CSM** → terfilter desa binaan; fokus grup 2.E; grup komersial read-only.
- **Support** → hanya baca, muncul saat menelusuri tiket; `Contact Phone`
  tersamar; grup 2.D komersial disembunyikan.

## 8c. Modul 3 Contacts — catatan rancangan

Hasil diskusi atas spec modul 3 (hal. 4–5). Contacts adalah **kembaran struktural
Accounts**: submenu `3.1 All / 3.2 My` = view/filter objek yang sama (§8b berlaku
sama — My = filter, bukan board terpisah), grup field A–F dengan `3.E Engagement
Summary` bertanda `rollup — read only` (pola dua-rumah §2) dan `3.F System/Audit`.
Keputusan modul 2 diwariskan; yang dicatat di sini hanya yang KHAS Contacts.

**Tiga hal yang membedakannya dari Accounts** — alasan ia digambar sendiri:
- **3.B Role & Authority** adalah inti CRM vertikal ini. `Contact Role` (Decision
  Maker / Influencer / User / Finance / Gatekeeper), `Is Primary Contact` (Kepala
  Desa, pengambil keputusan), `Is Technical Contact` (operator yang benar-benar
  memakai software — spec: *"krusial untuk Customer Success"*). Menjawab **"siapa
  meneken vs siapa memakai"** — dua orang berbeda di satu desa.
- **`Reports To` (Lookup Contact)** = hierarki INTERNAL desa (Operator → Sekdes),
  bukan relasi ke user CRM. Mini org-chart per desa.
- **`Term Period`** (masa jabatan, mis. 2021–2027) = penanda risiko churn khas
  produk ini: pergantian perangkat desa memutus relasi.

**Field sensitif — modul PALING padat PII** (tunduk §5, dengan satu pengecualian
yang diputuskan 2026-08-07):
- `Mobile Phone` & `WhatsApp Number` (3.C) — PII PRIBADI melekat ke orang;
  **tersamar untuk non-Sales** (termasuk Manager & Support). WhatsApp adalah kanal
  utama di desa, jadi masking-nya paling terasa di sini.
- `Office Phone` (3.C) — **TETAP TAMPAK untuk semua role.** Ini nomor INSTITUSI
  (telepon kantor desa, semi-publik — ada di papan pengumuman), beda sifat dari
  nomor pribadi. Menyamarkan nomor yang toh publik = friksi tanpa manfaat privasi.
  Pembeda yang dipakai: **PII pribadi disamarkan, nomor institusi tidak** — bukan
  "semua field telepon" mentah.
- `Email` (3.C) — tampak (kanal kerja, bukan PII sensitif setara HP).

**Penggambaran: 3 POV** (Sales · CSM · Support) untuk List & Detail — konsisten
modul 2, dan justru di sinilah beda Sales vs non-Sales paling kentara (3 field
telepon):
- **Sales** → kontak desa tugasnya, bisa edit; Mobile/WhatsApp tampak penuh.
- **CSM** → kontak desa binaan; menekankan `Is Technical Contact` (operator =
  lawan bicara utamanya); Mobile/WhatsApp tersamar; grup lain read-only.
- **Support** → read-only saat menelusuri tiket; Mobile/WhatsApp tersamar; hanya
  butuh tahu siapa yang bisa dihubungi via kanal institusi (Office/WhatsApp desa).

## 8d. Modul 4 Sales — catatan rancangan

Hasil diskusi atas spec modul 4 (hal. 5–8). Modul 4 adalah **rumahnya Sales** —
satu-satunya modul tempat Sales `✓` penuh sementara peran lain nyaris hanya
menonton (matriks §4: Leads cuma Sales; Deals & Quotes = Sales kerjakan, CSM `👁`,
Support `✕`). Karena itu modul ini digambar **sekali dari POV Sales**, dengan satu
pengecualian yang dijelaskan di bawah.

**Submenu = filter/view objek yang SAMA** (mewarisi §8b/§8c, kata spec sendiri):
- **4.1** `All Leads / My Leads / Unqualified Leads` → **tab filter** dalam satu
  board Leads List (My = "Punya Saya", Unqualified = filter `Lead Status`). Bukan
  tiga board.
- **4.2** `Pipeline View / All Deals / My Deals` → Pipeline (Kanban) adalah **view
  berbeda** dari objek yang sama, jadi tetap satu board dengan **toggle
  Kanban⇄Tabel** + filter All/My. Bukan board tabel terpisah.
- **4.3** Quotes → satu objek (Header + Line Items + Terms).

**Submenu sidebar Sales = 4 entri, satu per OBJEK** (Leads · Deals · Quotes ·
Activities). Ini berbeda dari All/My/Pipeline/Tabel di atas: yang itu **view/tab
di dalam halaman** (objek sama), sementara Leads/Deals/Quotes/Activities memang
**objek berbeda** — jadi wajar jadi submenu, bukan tab. Board turunan (Lead
Detail, Deal Detail, Quote Approval) **bukan** entri sidebar: mereka dibuka dari
list/approval, bukan dari menu.
- **`4.4 Activities` = submenu Sales (bukan menu global Activities)** — keputusan
  eksplisit (opsi A). `4.4 Sales Activities Log` sudah digambar sebagai log
  **khusus proses jual**, scope-nya beda dari item sidebar global **Activities**
  (aktivitas lintas-modul). Keduanya hidup berdampingan: submenu Sales →
  aktivitas sales; menu global → semua modul. Menaruhnya sebagai filter di menu
  global akan mengaburkan scope yang sudah sengaja dipisah di §8d.

**Status submenu: digambar.** Submenu Sales ter-expand di sidebar ke-7 board
Modul 4 (`sales-submenu` disisipkan tepat setelah `nav-Sales`) — 4 sub-item
ber-indent (Leads · Deals · Quotes · Activities), sub-item aktif di-highlight
`indigoSoft`+teks `indigo` bold sesuai board (Leads List/Detail → Leads; Deal
Pipeline/Detail → Deals; Quote Builder/Approval → Quotes; Activities Log →
Activities). `spacer` (vt=fill) menyerap tinggi tambahan → footer tetap terpaku.

**Empat grup jembatan yang mengunci modul ini ke tetangganya** (bukan sekadar
field):
- `Account (Village)` di Deal & Quote (Lookup Account) = **penghubung ke Modul 2**.
- `Primary Contact` di Deal (Lookup Contact) = **penghubung ke Modul 3**.
- Grup **4.2.D Subscription Link** (`Plan Requested`, `Subscription Term`,
  `Created Subscription`) = **jembatan ke Modul 5**: saat Deal `Closed Won`,
  langganan terbentuk. Inilah titik "MENJUAL → MENAGIH" di §2.
- `Converted` (4.1) = penanda lead sudah jadi Deal+Account+Contact — pintu masuk
  ke seluruh siklus.

**POV: Manager satu-satunya role kedua yang BERTINDAK di sini** (§9.4 Approval
Process — diskon > batas & renewal deal). Semua peran lain hanya `👁`. Maka
satu-satunya fork POV yang nyata di modul 4 adalah **Quote digambar 2 POV**:
- **4.3 Quote Builder (POV: Sales)** — menyusun baris item, hitung Grand Total,
  kirim penawaran.
- **4.3 Quote Approval (POV: Manager)** — layar sama, fokus **diskon & margin**,
  tombol **Setujui/Tolak**. Terekam audit (§8.3: wewenang approval wajib punya
  jawaban "siapa & kapan").

**Field-Level Security (§5) tak ada yang perlu digambar di modul 4:** Support `✕`
di seluruh 4.x (masking ARR tak relevan — layarnya tak pernah dibuka Support), dan
POV utamanya Sales (Mobile/WhatsApp tampak penuh). Modul ini justru KEBALIKAN
modul 3 — di sini beda-akses muncul lewat **tombol** (approve), bukan lewat
**masking field**.

**Penggambaran: 7 board** (4 board lama `Sales Flow — N` di-rename agar selaras &
urut nomor; 3 board baru menutup gap §10):
- `4.1 Leads List` (rename) — tab All/My/Unqualified.
- `4.1 Lead Detail` (baru) — grup A–D + tombol **Konversi**.
- `4.2 Deal Pipeline` (rename) — Kanban + toggle tabel.
- `4.2 Deal Detail` (rename) — grup A–E, termasuk Subscription Link.
- `4.3 Quote Builder (POV: Sales)` (rename).
- `4.3 Quote Approval (POV: Manager)` (baru).
- `4.4 Sales Activities Log` (baru) — **secara mekanis memakai objek Activities
  (Modul 7)** difilter konteks sales; digambar di modul 4 karena di situ ia
  dibaca. Grup A Activity Detail + grup B Schedule & Outcome.

**Status: selesai.** 7 board digambar & diverifikasi (satu baris di y=5600,
gap seragam 70px, nol overlap AABB, urut nomor). Detail per-board yang menutup
gap §10: Lead Detail = grup A Identity/Core + B Qualification & Status (badge
Status/Rating, Estimated Value, Converted) + C Location & Contact + D
System/Audit + tombol Konversi (→ Deal+Account+Contact). Deal Detail dilengkapi
grup C Outcome & Analysis (Win/Loss Reason, Competitor, Actual Close Date, Loss
Notes) + grup E System/Audit. Deal Pipeline dapat toolbar toggle Kanban⇄Tabel +
filter All/My. Quote Approval memuat blok **Analisis Diskon & Margin** (Total
Diskon, Margin Estimasi, Ambang Diskon terlampaui) + panel **Keputusan
Persetujuan** (aturan ambang, reviewer, SLA, tombol Setujui/Tolak).

## 8e. Modul 5 Subscriptions — catatan rancangan

**POV: Manager.** Subscription tak punya staff-owner (§ dashboard 575–588):
MRR/ARR/churn adalah data pengelolaan, bukan pekerjaan harian satu peran. Sales &
CSM hanya `👁` (5.1/5.2), Admin `✓` penuh (5.3 master data harga), Support `✕`.
Maka modul ini digambar **satu POV** — tak ada fork POV seperti Quote di §8d.

**Submenu = 2 entri, bukan tab.** Sidebar `subs-submenu` (disisipkan setelah
`nav-Subscriptions`) berisi **Semua Langganan** & **Plans & Pricing** — dua **objek
berbeda** (langganan pelanggan vs katalog harga), jadi wajar jadi submenu. Sub-item
pertama TAK boleh mengulang nama induk `nav-Subscriptions` (dulu "Subscriptions" →
dua entri kembar, menyesatkan) — beda dari Sales yang induknya bukan nama objek;
maka dipakai **"Semua Langganan"** (= daftar seluruh langganan), sejajar Plans &
Pricing. Di dalam objek Semua Langganan, Active/Renewals/Churn adalah **tab/view** (objek sama,
lensa beda) — konsisten dengan konvensi §8b–§8d (submenu = objek beda; tab = view
objek sama). Board turunan (Subscription Detail, Plan Detail) dibuka dari list,
bukan entri sidebar.

**Churn = board tersendiri (5.2), sejajar Renewals.** Semula churn dirancang cukup
**tab** di 5.1; keputusan itu di-override — churn dinaikkan jadi board sendiri
karena ia **view lifecycle** dari objek Subscriptions yang setara Renewals (dua
sisi keluar-masuk retensi), bukan sekadar filter daftar. Board **5.2 Churn**
memuat KPI kehilangan (Churn Rate, Churned MRR, Desa Churn, Avg Tenure) + tabel
desa berhenti dengan **alasan** & **MRR hilang** + tab periode. Tab `Churn` di 5.1
tetap ada sebagai filter cepat; analisis kohort/tren mendalam tetap di **Reports
8.4**. Sidebar: `Semua Langganan` aktif (Churn = view objek Subscriptions, bukan
entri submenu sendiri). Dua-rumah: **analisis churn di sini · winback di Customer
Success** (banner eksplisit).

**Dua-rumah 5.2 ↔ 6.6 (§2).** Board **5.2 Renewals** MENAMPILKAN status renewal
(Renewal Date, Days Left, Type Auto/Manual, Status, Prev→Current) + banner
eksplisit: **aksi perpanjangan dikerjakan di Customer Success → Renewal
Management**. "Subscriptions MENYIMPAN · Customer Success BERTINDAK." Grup Renewal
di Subscription Detail memakai judul `Renewal · (aksi di Customer Success)` — nota
yang sama, di titik data-nya.

**6 board (y=6888, urut nomor, gap 70px).** Dibangun ulang dari nol (3 board
`Subscription Flow` lama dihapus): (1) **5.1 Subscriptions List** — KPI Total
MRR/ARR/Active/Churn rate + tab Active/Renewals/Churn + tabel; (2) **5.1
Subscription Detail** — grup Identitas & Langganan · Financials · Status &
Lifecycle · Renewal · System & Audit; (3) **5.2 Renewals** — lensa-renewal +
banner dua-rumah; (4) **5.2 Churn** — KPI kehilangan + tabel desa berhenti
(alasan, MRR hilang, tenure) + tab periode + banner winback-di-CS; (5) **5.3 Plans
& Pricing** — katalog (Plan/SKU/Category/Base Price/Billing Freq/Status), submenu
**Plans & Pricing** aktif; (6) **5.3 Plan Detail** — grup Plan Detail + Pricing
(Setup Fee, Trial, Tax) + Included Features.

**Jembatan ke tetangga:** langganan lahir dari Deal `Closed Won` (grup 4.2.D
Subscription Link, §8d) — titik "MENJUAL → MENAGIH" (§2); Renewals menyerahkan
aksi ke 6.6 — titik "MENAGIH → MENJAGA".

## 8f. Modul 6 Customer Success — catatan rancangan

**POV: CSM** untuk mayoritas board. CS adalah rumah "menjaga" (§2) dan CSM
pemakainya sehari-hari; hanya modul tempat role lain benar-benar melihat beda
yang di-fork (§ POV 657–669): 6.1 Health Score dua POV (CSM · Sales-baca), 6.6
Renewal Management tiga POV di matriks (Sales · CSM · Manager). Sisanya (6.2–6.5,
6.7, 6.8) digambar sekali dari POV CSM — sesuai matriks §4 (`◐ ✓` CSM, `✕` untuk
Sales/Support di board-board itu). Sidebar: `nav-Customer Success` aktif, **tanpa
submenu ter-expand** (baris A & B seragam).

**Sembilan board, dua baris (y=9488, urut nomor, gap 70px).** Dikloning dari
board acuan 6.2 Journey lalu Content dikosongkan & diisi ulang dengan helper.

- **Baris A (h=1010, x=324→6204):** (1) **6.1 Health Score (POV: CSM)** ·
  (2) **6.1 Health Score (POV: Sales-baca)** · (3) **6.2 Journey / Onboarding** ·
  (4) **6.3 Success Plans** · (5) **6.4 Product Adoption**.
- **Baris B (h=900, x=7674→12084):** (6) **6.5 Engagements** — KPI interaksi +
  tabel Call/Email/QBR/Kunjungan dengan Tindak Lanjut & Hasil; (7) **6.6 Renewal
  Management** — pasangan aksi 5.2, banner dua-rumah + tabel Aksi CSM
  (Playbook/Perpanjang/→Sales); (8) **6.7 Playbooks** — daftar skenario intervensi
  `pemicu → langkah` dengan Kategori (At-Risk/Onboarding/Adopsi/Renewal), metrik
  Berjalan/Sukses & Status (Aktif/Draf); (9) **6.8 Voice of Customer** — NPS +
  segmen Promoter/Pasif/Detraktor + tabel umpan balik dengan Kategori & Tindak
  Lanjut (→ Support, Playbook, Minta Testimoni/Ajak Referral).

**Dua-rumah 6.6 ↔ 5.2 (§2 & §6).** 6.6 Renewal Management adalah tempat CSM
**bertindak**; 5.2 Renewals hanya **menyimpan** status. Banner eksplisit:
"Subscriptions MENYIMPAN status · Customer Success BERTINDAK. Perpanjangan paket
sama disalin CSM; naik paket → Sales (upsell)." CSM **pemilik** renewal (matriks
§4: 6.6 = `◐ ✓ pemilik`) tetapi **tak pernah mengubah angka kontrak** (§6): paket
sama → disalin sistem; ganti paket → diserahkan ke Sales sebagai penjualan baru.

**Winback pasca-churn ada di sini, analisis churn di 5.2 (§8e).** Playbook
"Winback Pasca-Churn" (6.7) & tindak lanjut Detraktor (6.8) adalah sisi CS dari
dua-rumah churn; angkanya tetap di Subscriptions 5.2 Churn.

- **Baris C (h=900, x=13554→16494, POV: Support):** (10) **6.9 Tickets / Cases**
  — KPI antrean (Terbuka/SLA Berisiko/Terlanggar/Selesai) + tab status + tabel
  tiket per desa (Prioritas · SLA countdown · Agen · Status); (11) **6.10
  Knowledge Base** — artikel bantuan mandiri (Terbit/Dilihat/Ter-deflect/Perlu
  Review) + tabel artikel dengan Kategori · Rating "Membantu %" · Status
  Terbit/Review/Draf; (12) **6.11 SLA Management** — KPI kepatuhan + banner aturan
  jam-kerja + tabel kebijakan SLA per prioritas (Target Respon/Selesai · Jam
  Berlaku · Kepatuhan). Support ada DI DALAM Customer Success (§2 baris 62): satu
  sidebar tanpa `nav-Sales`/`nav-Subscriptions`, `nav-Customer Success` aktif.

**POV Support ≠ POV CSM di sidebar.** Baris C dikloning dari 6.5 lalu dua item
nav dibuang (Sales, Subscriptions) — Support tak menyentuh penjualan/langganan;
ia menjaga janji layanan (tiket, KB, SLA). Matriks §4: 6.9 Support `✓ semua` ·
CSM/Sales `👁`; 6.10 Support `✓ tulis`; 6.11 Manager `✓ atur` · Support/CSM `👁` ·
Sales `✕`.

**6.12 Customer Portal ditunda (§7)** — portal sisi-desa (eksternal), bukan
board internal CRM.

## 8g. Modul 7 Activities — catatan rancangan

Satu tabel `activities` yang **polymorphic** melayani 6 jenis interaksi lewat
diskriminator `kind` (task/meeting/call/chat/email/note — lihat `skema.md` §7).
Di DATA ia satu tabel; di UI ia **6 submenu**, dan tiap submenu butuh list +
detail. Karena itu board dipecah jadi **12** (6 × 2), bukan dipaksakan ke satu
board polymorphic yang dulu jadi ⚠️ di §10.

**Kenapa dipecah, bukan satu board dengan tab.** Tiap `kind` punya field khas
yang tak berbagi bentuk: Task (7.1) `due_date`/`priority`/`reminder_at`; Meeting
(7.2) `start_at`/`end_at`/`all_day`/`location`/`meeting_type` + daftar
`activity_attendees`; Call (7.3) `direction`/`duration_min`/`call_result`; Chat
(7.4) `channel`/`direction` + transkrip percakapan; Email (7.5)
`email_from`/`email_to`/`email_status` + isi email; Note (7.6)
`title`/`visibility`. Satu tabel bersama akan memaksa kolom kosong yang
menyesatkan; submenu terpisah membuat tiap detail menampilkan HANYA field yang
relevan bagi jenisnya.

**Panel "Terkait (Related To)" = wujud polymorphism `related_to`/`target_type`.**
Tiap detail punya kartu kanan yang menautkan aktivitas ke objek induknya
(account/contact/deal/ticket/subscription). Ini yang membuat satu aktivitas bisa
"menempel ke desa, deal, maupun tiket mana pun" (§46–47) tanpa tabel penghubung
per-jenis.

**`activity_context` memisahkan lensa, bukan menduplikasi data.** Field ini
(sales/cs/general) yang membedakan `4.4 Sales Activities Log` (§8d, difilter
konteks sales) dari log CS di modul 6 — objeknya SAMA, filternya beda. Board 7.x
adalah rumah kanonik objek Activities; modul 4/6 hanya me-lens-nya.

**Layout detail seragam** (mengikuti pola `4.2 Deal Detail`, §8d): breadcrumb →
header (judul + badge status + subtitle + aksi kanan) → dua kolom — kiri kartu
detail + isi (catatan/transkrip/isi email), kanan kartu "Terkait" + "Riwayat".
POV tunggal (Activities sidebar, submenu jenisnya aktif) — tak ada fork POV,
sebab aktivitas dibaca sama oleh siapa pun yang berhak melihatnya; wewenang
diatur di matriks §4 (baris 7: Sales/CSM `✓`, Support/Manager `◐`), bukan lewat
board terpisah.

## 9. Batas penggambaran: hanya scope BISNIS

Yang digambar di Penpot **hanya sumbu bisnis**. Dua sumbu lain sudah berjalan di
kode template; menggambar ulang melahirkan dua versi kebenaran, dan yang di
Penpot pasti kalah mutakhir dari yang di kode.

| Sumbu | Sudah ada? | Wujudnya |
|---|---|---|
| Platform | ✅ selesai | panel `/dev` |
| Tenant | ✅ selesai | `/w/{slug}/members`, `/workspace/new`, Auth Flow |
| **Bisnis** | ❌ belum ada | **inilah CRM-nya — yang kita gambar** |

**Kecuali di Settings 9.1**, tempat dua sumbu bertemu:

- Undang / keluarkan / aktif-nonaktif anggota → sudah ada (tenant), **jangan
  digambar ulang**
- Beri label Sales / CSM / Support / Manager → belum ada (bisnis), **inilah yang
  digambar**

Board 9.1 karena itu bukan halaman berdiri sendiri melainkan **kolom tambahan**
pada daftar anggota yang sudah ada. Menggambarnya sebagai halaman terpisah
melahirkan dua tempat mengelola orang yang saling bertentangan — penyakit yang
persis dijaga [0008](../decisions/0008-daftar-anggota-hanya-pengelola.md).

Berlaku sama untuk 9.2: matriks izinnya adalah izin **bisnis** (boleh ubah health
score?), bukan izin tenant (boleh undang anggota?).

### Board 9.2 = read-only / preset, BUKAN editor izin runtime

Keputusan desain (board disentuh 2026-08-10). Board `Settings — 9.2 Roles &
Permissions` **menampilkan** kebijakan izin, ia tidak mengeditnya. Di v1 ada
**5 role tetap** (admin/manager/sales/csm/support) dengan izin lewat **Casbin
CSV (deny-default)** dan cakupan kepemilikan lewat **ownership-filter di app-layer**
(`WHERE account_owner=$uid OR assigned_csm=$uid`) — bukan RLS, bukan toggle.
Ketiganya hidup di **kode**, jadi tombol yang menjanjikan menyunting izin per-baris
saat runtime adalah **afordansi yang berbohong**: menekannya tak akan pernah
mengubah apa pun yang benar-benar menegakkan izin.

Karena itu board hanya menyisakan aksi yang jujur:

| Dibuang / dijujurkan | Dipertahankan (jujur) |
|---|---|
| `+ Role Baru`, `Duplikat` → dihapus | `Ekspor Matriks Izin` (read-only) |
| `Ubah` (per role) → `Lihat` (muted) | `Terkunci` pada Admin |
| `Atur` (per baris rule & FLS) → dihapus | `Role: … ▾` (memilih role untuk **ditinjau**) |
| `Config` (card-head) → `Preset` | pill nilai preset (`Global`/`Assigned`/`Full`/`Terbatas`/`Tersamar`/`Terbuka`) |

Yang tetap **dinamis** di sistem nyata hanyalah: **person → role** (label
keanggotaan, sudah digambar di 9.1) dan **penugasan desa** (§8.1–8.2). Bukan
matriks izinnya. Agen berikutnya: **jangan** "melengkapi" board ini dengan
menambah kembali tombol edit/duplikat/role-baru — itu memutar balik keputusan ini
dan menjanjikan fitur yang v1 tak bangun.

### Dashboard: submenu HANYA muncul bila role punya >1 dashboard

Spec §1 memberi **4 dashboard per FUNGSI** (Sales · CS · Support · Subscription),
bukan per role — buktinya Manager & Admin tak punya dashboard sendiri, mereka
justru perlu melihat keempatnya. Maka "Dashboard" adalah navigasi (submenu),
BUKAN satu tampilan yang berubah per POV. Tapi keduanya berlaku, untuk lapisan
berbeda: **submenu memilih FUNGSI mana** (navigasi), **POV menentukan seberapa
LUAS datanya** di dalamnya (mis. Sales Dashboard dibuka Sales → deal miliknya;
dibuka Manager → seluruh tim).

**Bukan submenu — SELECT.** Menu "Dashboard" adalah **link langsung untuk semua
role** (berperilaku seragam, beda dari Reports yang bersubmenu). Yang membedakan:
role dengan >1 dashboard mendapat **select** di header untuk berpindah pandangan.

| Role | Menu "Dashboard" | Select di header |
|---|---|---|
| Admin | link langsung | **ada** — "Tampilkan: [Sales ▾]" (4 pilihan) |
| Manager | link langsung | **ada** (4 pilihan) |
| Sales | link langsung | tidak — konten tunggal (Sales) |
| CSM | link langsung | tidak — konten tunggal (CS) |
| Support | link langsung | tidak — konten tunggal (Support) |

**Why select, bukan submenu:** keempat dashboard adalah **pandangan atas halaman
yang sama**, bukan halaman berbeda — mengganti pandangan = select, bukan pindah
navigasi. Select juga menghapus keanehan "sidebar berbeda bentuk per role": menu
Dashboard kini satu link untuk semua; hanya isi header yang berbeda. Submenu
berisi SATU item (nasib staf) adalah gangguan, bukan navigasi.

**Dashboard default dipilih di Settings.** Select di header hanya ganti
sementara. Pilihan "dashboard mana yang tampil pertama saat login" disimpan
sebagai preferensi di **Settings › Customization (9.3)** — satu opsi kecil
"Dashboard default: [fungsi ▾]", BUKAN dashboard builder. Isi tiap dashboard (6
kartu, sesuai tabel spec §1) tetap terkunci per spec; yang bisa dipilih hanya
dashboard mana yang dibuka & mana yang jadi default. Membuka builder (pilih kartu
sendiri) DITOLAK: melawan spec yang mengunci 6 komponen, dan menduplikasi Custom
Reports (8.5).

Komponen tiap dashboard TIDAK berubah antar-POV; yang berubah hanya cakupan
datanya (Sales Dashboard dibuka Sales → deal miliknya; dibuka Manager → seluruh
tim).

**Catatan awal (bisa berubah): Subscription Dashboard tanpa pemilik staf.** Tiga
dashboard punya role staf pemilik (Sales→Sales, CS→CSM, Support→Support), tapi
**Subscription tidak** — tak ada "role Subscription", dan data langganan (MRR,
ARR, churn, revenue per plan) adalah pengawasan pendapatan = urusan manajerial.
Konsisten dengan §4: untuk modul 5.x, Sales & CSM hanya `👁`, Support `✕`; hanya
Admin & Manager `✓`. Konsekuensinya:

- Board POV staf hanya **tiga** (Sales, CSM, Support) — tak ada "Subscription
  (POV staf)". Bukan kelalaian; tak ada staf yang membukanya.
- Subscription muncul di **select "Tampilkan"** hanya bagi Manager/Admin.
- Pilihan "Subscription" sebagai **dashboard default** (Settings 9.3) hanya
  tersedia untuk Manager/Admin.

Ditandai sebagai catatan awal: bila kelak ada role finance/billing, Subscription
Dashboard bisa mendapat pemilik staf-nya sendiri.

Yang TIDAK digambar: login, OAuth, pilih workspace, panel `/dev`, undang &
keluarkan anggota.

### POV: peta akses per role + POV berganda hanya di modul yang berbeda

Sebelumnya semua board memakai sidebar 9 menu penuh — yaitu POV Admin, tanpa
pernah diputuskan. Akibatnya matriks §4 tak pernah terbukti di layar.

**Yang dipakai sekarang, dua lapis:**

**(a) Lima board "Peta Akses"** — satu per role. Menampilkan sidebar sesuai
haknya + daftar seluruh modul dengan tiga penanda: *bisa kelola* · *lihat saja* ·
*tak terlihat*. Inilah yang menjawab "sebagai Support Agent saya melihat apa?"
dalam satu layar.

**(b) POV berganda HANYA di modul tempat role benar-benar melihat beda:**

| Modul | POV yang digambar |
|---|---|
| 2 Accounts (list + detail) | Sales · CSM · Support |
| 5.2 Renewals / 6.6 Renewal Mgmt | Sales · CSM · Manager |
| 6.1 Health Score | CSM · Sales (baca) |
| 6.9 Tickets | Support · CSM · Sales |
| 8 Reports | Manager · Sales · CSM · Support |
| 9.1 User Management | Admin |

Modul lain digambar **sekali** dari POV pemakai utamanya: 4.x → Sales · 6.2–6.8 →
CSM · 6.10–6.11 → Support · 5.x → Admin · 7 → CSM · 9.x → Admin.

**Why tidak digambar penuh 5× semua layar** (≈157 board): dari 150 board, sekitar
90 akan berisi halaman "tidak punya akses" atau salinan identik — sebab sebagian
besar modul hanya punya SATU role yang mengaksesnya (4.1 Leads cuma Sales; 6.3,
6.4, 6.5, 6.7, 6.8 cuma CSM). Halaman "tak punya akses" bukan rancangan melainkan
**ketiadaan** rancangan; menggambarnya berulang tak menambah pengetahuan, dan
wireframe yang mayoritas isinya duplikat berhenti dibaca — yang tak dibaca tak
pernah dikoreksi. Pengetahuan justru lahir di 6 modul tempat dua role membuka
layar SAMA dengan tombol BERBEDA; di situlah keputusan desain sesungguhnya.

**Konsekuensi yang diterima:** sidebar berbeda antar-board (Sales tak melihat
menu Customer Success). Itu **bukan inkonsistensi melainkan bukti** penggerbangan
role bekerja. Board lama yang terlanjur bersidebar penuh diberi **chip POV** di
header lebih dulu; penyesuaian sidebarnya dikerjakan terpisah, bukan sekaligus.

## 10. Status penggambaran di Penpot

File `vanguard`, page `wireframe`. Board tersusun **urut nomor menu**, jarak
antar-baris 400px.

| Modul | Board ada | Kelengkapan |
|---|---|---|
| Auth Flow | 7 | di luar spec (tambahan) |
| 1 Dashboard | 4 | ✅ lengkap 1.1–1.4 |
| 2 Accounts | 3 | ✅ list · territory · detail |
| 3 Contacts | 6 | ✅ 3 POV list + 3 POV detail (Sales/CSM/Support) |
| 4 Sales | 7 | ✅ Leads(list+detail) · Deals(pipeline+detail) · Quotes(builder+approval) · Activities |
| 5 Subscriptions | 6 | ✅ 5.1 list+detail · 5.2 Renewals · 5.2 Churn · 5.3 Plans & Pricing+detail |
| 6 Customer Success | 12 | ✅ baris A 6.1–6.4 (5 board) · baris B 6.5–6.8 (4 board) · baris C 6.9–6.11 Support (3 board) · 6.12 Portal ditunda (§7) |
| 7 Activities | 12 | ✅ 6 submenu × (list + detail): 7.1 Tasks · 7.2 Meetings · 7.3 Calls · 7.4 Chat · 7.5 Emails · 7.6 Notes |
| 8 Reports | 5 | ✅ lengkap 8.1–8.5 |
| 9 Settings | 5 | ✅ lengkap 9.1–9.5 |

**Modul 7 kini LENGKAP.** Board polymorphic tunggal dipecah jadi 12 board
(6 submenu × list + detail) — lihat §8g untuk alasan desainnya. Modul 6 juga
LENGKAP (baris A/B/C); 6.12 Customer Portal sengaja ditunda karena portal
sisi-desa eksternal, bukan board CRM internal. Board lama `CS Engine` (3) &
`Support Flow` (3) sudah DIHAPUS — kontennya di-supersede penuh oleh baris A/B/C,
jadi menyimpannya hanya menciptakan dua versi kebenaran.
