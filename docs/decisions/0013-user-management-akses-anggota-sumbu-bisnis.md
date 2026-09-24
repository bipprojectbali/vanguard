# 0013 — Akses `/members` melebar ke sumbu bisnis (`crm:members`), merevisi ADR-0008

Status: **Diterima & SELESAI** (2026-09-24, branch `feat/crm-roles-user-mgmt-permission-bl171`;
belum merge/push). Merevisi [0008](0008-daftar-anggota-hanya-pengelola.md) ("daftar
anggota hanya pengelola") — bukan supersede total, lihat Keputusan #1. Melengkapi pola
matriks [0012](0012-fls-per-tenant-configurable.md) (section per-peran di `/roles`) dan
kolom `kind` (internal/eksternal) dari BL-170.

> **Revisi terhadap rencana terkunci BL-170/171 (`docs/crm/tasks.md`):** keputusan
> arsitektur yang dikunci user 23 Sep (via AskUserQuestion) menyatakan akses `/members`
> lewat `crm:members` akan **MENGGANTIKAN** gerbang lama `canManageMembers`, dan migrasi
> **WAJIB** seed `level=Kelola` + kedua Jenis Anggota tercentang untuk role owner/admin/
> platform yang sudah ada di semua tenant. Implementasi aktual **TIDAK melakukan itu** —
> lihat Keputusan #1 untuk kenapa, dan kenapa hasil akhirnya tetap benar tanpa seed
> tersebut. Dokumen ADR ini ditulis SETELAH kode (bukan sebelum, seperti biasanya) karena
> deviasi ini baru ketahuan saat meninjau diff — user memutuskan (24 Sep): tulis ADR dulu
> baru tandai BL-171 selesai, agar deviasi terdokumentasi bukan diam-diam di-commit.

## Konteks

BL-170 menambah kolom `kind` (internal/eksternal) di `business_roles`/`memberships`/
`invites`. BL-171 melanjutkan: admin ingin bisa memberi role KUSTOM (bukan cuma
owner/admin/platform tenant) akses ke halaman Anggota (`/w/{slug}/members`) — baik
sekadar **Lihat** (mis. HR internal yang perlu melihat siapa anggota tim) maupun
**Kelola** (mis. koordinator regional yang mengurus pendaftaran akun eksternal) — dan
membatasi APA yang mereka lihat: hanya anggota internal, hanya eksternal, atau keduanya.

Sebelum BL-171, `/members` digerbangi tunggal oleh `canManageMembers` (ADR-0008): hanya
role TENANT owner/admin/platform. Tak ada jalur bagi role CRM kustom untuk melihat/
mengelola direktori anggota sama sekali.

## Keputusan

### 1. MELEBARKAN, bukan MENGGANTIKAN `canManageMembers`

Rencana terkunci semula meminta `crm:members` **menggantikan** `canManageMembers`. Kode
aktual (`internal/handler/member_access.go`) membuatnya **aditif**:

```go
func canViewMembers(ctx context.Context) bool {
    return canManageMembers(ctx) || crmMemberAccess(ctx) != memberAccessNone
}
```

`canManageMembers` (jalur tenant-axis: owner/admin/platform) **tetap utuh, tak
disentuh** — persis perilaku sebelum BL-171. `crmMemberAccess` (jalur business-axis,
module Casbin baru `crm:members`, business_modules.go) adalah tambahan jalur KEDUA.

**Kenapa menyimpang dari rencana:** mengganti gerbang lama berarti akses owner/admin/
platform tiba-tiba bergantung pada baris `member_scope_policies` yang, tanpa seed
backfill, TIDAK ADA untuk tenant existing — persis skenario yang ingin dicegah rencana
semula lewat migrasi seed WAJIB. Membuatnya aditif menghilangkan kebutuhan seed itu sama
sekali: jalur tenant-axis tak pernah membaca `member_scope_policies`
(`actorKindScope` return cakupan PENUH lebih dulu bila `canManageMembers(ctx)` true,
tanpa query ke tabel baru). Konsekuensinya migrasi 00052 **tidak punya baris
backfill/seed** — berbeda dari preseden 00050 yang membackfill. Ini konsisten, bukan
lupa: tak ada yang perlu di-backfill karena tak ada yang bergantung pada tabel itu untuk
akses yang sudah ada.

Batasan tegas yang tetap dijaga: **perubahan role TENANT** (member/admin/owner) di
`MemberSetRole` HANYA lewat `canManageMembers` murni — sub-cabang tenant-role
re-cek `canManageMembers(ctx)` secara ketat, tak ikut melebar. Business-axis actor
dengan `crm:members` Kelola tak pernah bisa mengeskalasi ke otoritas tenant.

### 2. Cakupan per role: `member_scope_policies`, default TERBUKA (kebalikan FLS)

Tabel baru (migrasi `00052`), mirip struktur `field_security_policies` (0012) — RLS
FORCE, unik per `(tenant_id, business_role)`, FK komposit ke `business_roles` — tapi
**semantik default kebalikannya**: baris tak ada (`pgx.ErrNoRows`) → KEDUA jenis
(internal+eksternal) terbuka, bukan tertutup. Alasan: fitur ini **opt-in melebarkan**
akses (role baru diberi `crm:members` mestinya langsung bisa lihat semua, lalu admin
mempersempit belakangan bila perlu) — beda dari FLS yang **opt-in mengunci** data
sensitif (default tertutup, admin membuka belakangan).

`CHECK (can_view_internal OR can_view_external)` — minimal satu harus true (kebalikan
FLS yang mengizinkan keduanya false). Ditegakkan di server (`roles_update.go`: bila
kedua checkbox terkirim false, di-**koreksi jadi true/true**, bukan ditolak) dan di UI
(`reactiveMemberScopeCheck`, `member_scope.go`: mencegah uncheck terakhir tanpa
roundtrip). Ini beda dari kata "validasi" di rencana semula, yang bisa dibaca sebagai
penolakan submit — implementasi memilih **koreksi diam-diam ke default aman** (matmatch
falsafah "default terbuka, kunci eksplisit"), bukan blocking error.

Dua pembacaan tabel ini punya sikap gagal yang **sengaja berbeda**:
- `actorKindScope` (`member_access.go`, dipakai saat MENGELUARKAN data anggota) →
  error selain `ErrNoRows` = **fail-CLOSED** (`kindScope{}`, tak ada yang terlihat).
  Kegagalan baca di sini langsung menentukan data mana yang bocor keluar.
- `memberScopeView` (`roles_page.go`, dipakai MENAMPILKAN form checkbox di `/roles`) →
  error apa pun = **fail-OPEN** (tampilkan true/true). Ini cuma memengaruhi tampilan
  form; submit tetap digerbangi ulang oleh koreksi server di atas, jadi gagal baca di
  sini tak boleh mengunci admin dari peran yang baru saja ia buat.

### 3. Gate SAMA dengan matriks, bukan gate terpisah (beda dari pola FLS 0012)

Field Security (0012) sengaja punya objek Casbin, form, dan endpoint POST **terpisah**
(`crm:field_security`) dari matriks F2. `member_scope_policies` TIDAK — section "Cakupan
Jenis Anggota" melebur ke form matriks yang sama (`mscope_present`, pola persis
`fsec_present`), digerbangi `canManageRoles` yang sama dengan tabel matriks. Alasannya:
cakupan ini murni turunan dari level `crm:members` peran ini sendiri (tak masuk akal
dikelola independen dari modul yang ia batasi), beda dari FLS yang lintas beberapa
modul (Contacts/Leads/Konversi) sekaligus.

### 4. Filter kind: pasca-fetch di Go, bukan varian query SQL baru

Rencana menyarankan varian `ListMembersByTenant`/`ListInvitesByTenant` ter-filter di
level SQL, meniru teknik row-filter F3 ownership. Implementasi aktual memfilter
**setelah fetch**, di `members_page.go`: `if !scope.Allows(m.Kind) { continue }` atas
hasil query yang SAMA (tak berubah) yang sudah dipakai jalur tenant-axis. Dipilih karena
endpoint ini sudah tanpa paginasi sama sekali (diakui eksplisit di luar cakupan BL ini)
— seluruh daftar sudah termuat ke memori sebelum baris ini dijalankan, jadi filter SQL
tambahan tak mengurangi kerja nyata, hanya menambah permukaan query. Trade-off ini perlu
ditinjau ulang bila/ketika `/members` akhirnya dipindah ke keyset pagination (pola
project, lihat CLAUDE.md).

### 5. Kolom `kind` di baris anggota hanya bisa diubah oleh cakupan PENUH

`MemberSetKind` menambah pengecekan: mengubah `kind` seorang anggota ke nilai LAIN dari
kind saat ini butuh `scope.Both()` — actor bercakupan sempit (mis. hanya internal) tak
bisa memindah siapa pun ke/dari jenis yang tak ia lihat sendiri. Persis seperti
direncanakan.

### 6. Self-edit business-axis actor dibatasi ke CRM-role "admin" saat ini

Untuk `MemberSetRole`/`MemberSetKind`, actor **business-axis murni** (bukan tenant
manager) hanya boleh mengedit BARIS DIRINYA SENDIRI bila `business_role` dia SAAT INI
sudah `"admin"`. Tenant manager (owner/admin/platform) dikecualikan dari batasan ini
(perilaku lama dipertahankan, `TestMemberBiz_SelfOptIn`). Ini mencegah actor business-
axis non-admin (mis. "sales" yang kebetulan diberi `crm:members` Kelola) memakai
dropdown yang sama untuk menaikkan/mengunci perannya sendiri secara tak sengaja — tidak
ada di rencana tertulis, ditemukan perlu saat implementasi.

### 7. `MemberRemove` melebar di gate, tapi `GuardDelete` (hierarki tenant) TETAP penjaga sesungguhnya

Gate GET/akses `MemberRemove` melebar sama seperti endpoint lain. Tapi
`authz.GuardDelete` (cek hierarki role TENANT actor vs target) **tak disentuh sama
sekali**. Konsekuensi nyata: actor business-axis murni yang role TENANT-nya "member"
tetap akan SELALU gagal `GuardDelete` saat mencoba menghapus peer ber-role TENANT
"member" lain (`GuardDelete` mensyaratkan role actor lebih tinggi). Artinya pelebaran
BL-171 pada `MemberRemove` **efektif tak berlaku** untuk kombinasi paling umum (business-
axis actor + kedua pihak role tenant "member") — ini SENGAJA: BL-171 melebarkan
visibilitas & pengelolaan DATA CRM, bukan hierarki otoritas tenant. Actor tetap perlu
role TENANT yang lebih tinggi untuk benar-benar menghapus anggota.

### 8. `maskEmail` dicabut dari `/members` (deviasi dari ADR-0008 §5)

ADR-0008 §5 mempertahankan `maskEmail` "untuk berjaga-jaga bila halaman ini dibuka ke
audiens lebih luas nanti". BL-171 justru MEMBUKA halaman itu ke audiens lebih luas
(business-axis actor) — namun implementasi **mencabut** `maskEmail` sepenuhnya dari
`members_page.go`, bukan mempertahankannya seperti diantisipasi 0008. Rasionalnya
(komentar di kode): gerbang halaman (`canViewMembers`) sudah mempersempit penonton ke
pengelola tenant ATAU role bisnis yang EKSPLISIT diberi akses "User Management" oleh
admin lewat `/roles` — populasi ini sudah terkurasi/dipercaya, beda dari model ancaman
0008 yang membayangkan "anggota biasa mana pun" bisa melihat halaman.

**Konsekuensi yang perlu disadari:** actor dengan level **Lihat** saja (bukan Kelola)
tetap melihat email PENUH tanpa masking, walau dari sisi AKSI ia setara "anggota biasa"
di bawah 0008 (tak ada tombol aksi apa pun). Ini tarik-ulur yang diterima sadar, bukan
celah tak disadari — dicatat di sini agar tak ditemukan ulang sebagai "bug" nanti.

## Konsekuensi

- Tenant existing (owner/admin/platform) **nol migrasi/backfill diperlukan** — akses
  mereka tak pernah bergantung pada `crm:members`/`member_scope_policies` sama sekali.
  Ini kebalikan dari asumsi rencana semula yang mewajibkan seed, tapi hasil akhirnya
  aman karena gerbang lama tak diganti (Keputusan #1).
- Role bisnis bawaan **"admin"** otomatis mendapat `crm:members` write lewat wildcard
  `crm:*` yang sudah ada di `business_policy.csv` (`keyMatch` + write-mencakup-read,
  `business.conf`) — tak perlu baris CSV baru. Role bawaan lain (manager/sales/csm/
  support) dan role kustom baru **tidak** otomatis dapat akses (deny-default matriks,
  sama seperti modul lain) — admin memberi lewat `/roles` bila perlu.
- Endpoint yang gate-nya melebar: `MembersPage` (GET), `MemberSetRole`, `MemberSetKind`,
  `MemberRemove`, `InviteCreate`, `InviteDelete`, plus item menu "User Management"
  (`workspace_nav_settings.go`) dan `workspaceNavCtx` (`pending_member.go`) — semua
  konsisten memakai `canViewMembers`/`crmMemberAccess`/`actorKindScope` dari
  `member_access.go` sebagai satu sumber kebenaran, tak diduplikasi ad-hoc per endpoint.
- Bug ditemukan+diperbaiki SELAMA implementasi (bukan bagian keputusan desain, dicatat
  sebagai catatan implementasi): `settingsGroup` sebelumnya menaruh atribut Field
  Security DAN Member Scope pada satu `<div>` pembungkus bersama → dua atribut
  `data-signals` di elemen HTML yang sama → browser diam-diam membuang yang kedua →
  checkbox `msp_*` selalu tampil tak tercentang meski data DB benar. Diperbaiki dengan
  memberi tiap grup `<div>`-nya sendiri (`role_edit_additional.go`).
- Nomor ADR: rencana semula menyebut "ADR-0009", tapi slot itu sudah dipakai
  (`0009-master-wilayah-administratif.md`, topik tak berkaitan, dibuat lebih dulu).
  Dokumen ini memakai **0013** (slot bebas berikutnya); referensi "ADR-0009" di
  komentar kode (`business_modules.go`, `member_access.go`) diperbaiki menunjuk ke sini.

## Di luar cakupan

- Paginasi `/members` (keyset `?after=`, konvensi project) — endpoint ini tetap tanpa
  paginasi seperti sebelum BL-171; diakui eksplisit sebagai utang terpisah.
- Memindahkan filter kind dari pasca-fetch Go ke query SQL — lihat Keputusan #4;
  relevan kembali bila/ketika paginasi ditambahkan.
