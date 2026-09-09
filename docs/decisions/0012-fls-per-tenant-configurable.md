# 0012 — FLS HP/WhatsApp: kebijakan per-tenant, matriks jadi section di halaman Roles & Permissions

Status: **Diterima & SELESAI** (2026-09-09, branch `feat/crm-phone-fls-configurable-bl107`;
belum merge/push). Men-supersede aturan hardcode nomor HP/WhatsApp di
[§5 sistem-dan-role](../crm/sistem-dan-role.md) untuk modul Kontak/Lead/Konversi;
melengkapi [0008](0008-daftar-anggota-hanya-pengelola.md) (kelola per-workspace) &
preseden matriks-kapabilitas [BL-58 `crm:subscriptions/arr`].

> **Revisi (opsi B, 2026-09-09):** rencana awal keputusan #3 menaruh matriks di
> HALAMAN Settings tersendiri `/w/{slug}/field-security`. Atas permintaan user matriks
> itu digabung menjadi **section di bawah tabel peran pada halaman `/roles`** (Roles &
> Permissions) — SATU halaman, DUA section. Yang TIDAK berubah: penyimpanan
> (`field_security_policies`), gate (`crm:field_security`), form, dan endpoint POST
> tetap TERPISAH dari sumbu F2/Casbin. Co-lokasi di satu halaman ≠ mencampur dua sumbu
> dalam satu transaksi. Rute GET `/field-security` dihapus (section menumpang GET
> `/roles`); POST `/field-security` tetap, PRG balik ke `/roles?ok=fsec_saved`.

## Konteks

Field-Level Security (F4, sumbu ketiga di `internal/handler/fls.go:11-32`) memutuskan
field MANA yang boleh dilihat/disunting, tegak lurus dari F2 (Casbin — modul mana) &
F3 (ownership — baris siapa). Sampai BL-107, F4 untuk **nomor HP (`mobile_phone`) &
WhatsApp** di modul **Kontak, Lead, & form Konversi Lead** di-*hardcode*: lihat penuh =
Sales+Admin, sunting = Sales-only, sisanya mask `•••`. Aturan itu hidup di kode, satu
untuk semua tenant.

[BL-106](../crm/tasks.md) (MERGED) melepas F4 dari Account/Desa sepenuhnya — HP Kontak
account = data kelembagaan, bukan PII pribadi. BL-107 menangani kasus SEBALIKNYA untuk
Kontak/Lead/Konversi: kontrol PII tetap diinginkan, **tapi tiap workspace ingin
menentukan sendiri** peran bisnis mana yang boleh melihat/menyunting nomor. Template ini
di-clone jadi banyak deployment dengan struktur tim berbeda; satu aturan hardcode tak
lagi memadai.

Menjadikan F4 sebagai DATA (bukan kode) adalah pergeseran arsitektur → ADR ini.

**Keputusan desain (dikonfirmasi user 2026-09-09):**

1. **Gabung** — SATU kebijakan berlaku identik untuk Kontak + Lead + Konversi. Helper
   `fls.go` memang sudah dibagi lintas ketiganya; memisahnya per-modul menambah tiga
   matriks tanpa kebutuhan nyata.
2. **Dua sumbu terpisah** — "lihat nomor penuh" & "boleh sunting" dikonfigurasi sendiri-
   sendiri, mempertahankan realita lama Admin bisa LIHAT tapi read-only. Backend meng-
   *coerce* `view = view || edit` (nomor tak terlihat mustahil disunting).
3. **Matriks jadi section di `/roles`** (opsi B — lihat revisi di atas) — matriks
   `business_role × {lihat, sunting}` dirender di bawah tabel peran, TAPI dengan form,
   endpoint POST (`/field-security`), penyimpanan, dan gate SENDIRI. Rencana awal
   menaruhnya di halaman tersendiri agar tak mencampur sumbu F2/Casbin (granularitas
   modul) dengan F4 (granularitas field); co-lokasi opsi B tetap menghormati pemisahan
   itu — dua section berdampingan, dua transaksi, dua gate — sekaligus menaruh keduanya
   di tempat yang sama-sama dicari admin ("siapa boleh apa"). Yang haram bukan satu
   halaman, melainkan satu form/tabel/transaksi yang mengaduk kedua sumbu.
4. **Default = pertahankan sekarang** — tenant tanpa baris konfigurasi ⇒ lihat=Sales+
   Admin, sunting=Sales. Nol perubahan perilaku sampai admin sengaja menyimpan matriks
   (kontrak sama `code_formats`).

## Keputusan

### Penyimpanan: tabel per-tenant ber-RLS, bukan `platform_settings` global, bukan matriks Casbin

Kebijakan disimpan di **`field_security_policies`** (migrasi `00044`): satu baris per
`(tenant_id, business_role)`, dua boolean (`can_view_phone`, `can_edit_phone`),
ENABLE+FORCE RLS + POLICY isolasi tenant (pola `code_formats` 00006) + GRANT eksplisit
`app_rw`. CHECK `NOT can_edit_phone OR can_view_phone` menegakkan edit⇒view di DB; FK
komposit `(tenant_id, business_role) → business_roles(tenant_id, name) ON DELETE
CASCADE` menjaga baris tak menunjuk peran hantu.

Alternatif yang **ditolak**:

- **`platform_settings` global** — kebijakan ini per-tenant, bukan per-platform.
- **Matriks Casbin `business_role_permissions` (F2)** — F4 punya DUA sumbu (view/edit)
  yang tak memetakan bersih ke satu `act` Casbin, dan menaruhnya di sana melanggar
  pemisahan tiga sumbu. Preseden `crm:subscriptions/arr` (BL-58) memang menaruh
  visibilitas ARR di Casbin — TAPI ARR itu satu sumbu (lihat/tidak) yang cocok jadi satu
  `act`. Phone beda: dua sumbu + kebutuhan halaman matriks per-tenant → tabel khusus.
  Catatan opsi B: section F4 kini DIRENDER di halaman `/roles` yang sama, tapi
  penyimpanannya tetap tabel `field_security_policies` terpisah — tak sebaris pun masuk
  `business_role_permissions`. Berbagi halaman, bukan berbagi tabel.

### Gate: objek bisnis `crm:field_security` write, admin-only, TAK grantable

Section & POST dijaga `canManageFieldSecurity(ctx) = authz.CanBusiness(ctx,
"crm:field_security", "write")` — **gate SENDIRI**, bukan `canManageRoles`. Admin
tercakup lewat glob `crm:*` (`business_defaults`); peran lain deny-default. **Sengaja
TIDAK dimasukkan `authz.CRMModules()`** → tak jadi kolom editor `/roles`, jadi tak bisa
diberikan ke non-admin lewat panel (perlakuan sama `crm:roles`). Tanpa baris CSV/seed
baru. Opsi B: halaman `/roles` dijaga `canManageRoles` (`crm:roles`); section F4 di
dalamnya dinilai ULANG dengan `canManageFieldSecurity` (`fieldSecurityViewFor` → `nil`
bila tak berwenang, section tak dirender). Keduanya admin-only & selalu selaras dalam
praktik, tapi gate-nya tetap dua — halaman tak mengasumsikan yang membuka `/roles`
otomatis berhak atas matriks F4.

### Cache in-proses + reload seketika (`internal/fls`)

Kebijakan di-cache `map[tenantID]map[role]Policy` + `sync.RWMutex` (pola enforcer bisnis
F2 & `internal/settings`). Kunci tenant ADA = **terkonfigurasi**; absen = **default
terkunci**. `Load(rows)` saat startup (sumber `ListAllFieldSecurityPolicies` dalam
`db.WithSuper` — RLS menyembunyikan tenant lain di tx ber-scope, jadi konteks super
wajib memindai semua). `ReloadTenant(tenantID, map)` setelah tulis; peta kosong-tapi-
hadir tetap menandai terkonfigurasi (admin uncheck semua ≠ default).

Helper `fls.go` diubah membaca cache lewat ctx: `canSeeFullPhone(ctx)`,
`canEditPhone(ctx)`, `maskPhone(ctx, phone)`. Default terkunci hidup di SATU tempat
(`internal/fls`, `defaultView = sales||admin`, `defaultEdit = sales`).

### POST replace-all

POST menghapus semua baris tenant lalu menulis satu baris untuk **SETIAP** peran DB
(termasuk all-false) — agar "terkonfigurasi all-false" beda nyata dari "belum
dikonfigurasi" (= nol baris). Set peran diambil dari `ListBusinessRoles` (closed set);
field peran asing/dikarang diabaikan diam-diam (FK DB jaring terakhir). Ter-audit
(`workspace.field_security`, metadata jumlah peran — bukan PII); native POST → 303
`?ok=fsec_saved` ke `/roles` (gotcha #16, JANGAN `sse.Redirect`). Kode `fsec_saved`
(bukan `saved` milik simpan peran) agar alert di `/roles` spesifik tak tertukar.

## Konsekuensi

- **Tiap workspace mengatur sendiri** siapa yang melihat/menyunting nomor, tanpa deploy.
  Berlaku seketika (cache reload), satu kebijakan untuk Kontak+Lead+Konversi.
- **Fail-closed sekali terkonfigurasi.** Peran tanpa baris pada tenant terkonfigurasi →
  tersembunyi, TIDAK jatuh ke default. Konsekuensinya: **peran baru** yang dibuat
  SETELAH konfigurasi ikut fail-closed sampai admin membuka ulang section Field Security
  di `/roles` & menyimpan — didokumentasikan di section & di sini.
- **Eventual-consistency antar-instance.** Cache in-proses: pada deployment multi-
  instance, `ReloadTenant` hanya menyegarkan instance yang melayani POST; instance lain
  menyegarkan pada restart/muat berikut. Tradeoff sama enforcer F2 & `internal/settings`
  — diterima karena ini kontrol keamanan yang mengetat perlahan, bukan gerbang auth per-
  request; jalur eskalasi bila perlu = Redis pub/sub (belum diperlukan).
- **Admin tetap read-only secara default** (view tanpa edit) — realita lama dipertahankan
  lewat dua sumbu.
- Default pertahankan-sekarang ⇒ **nol perubahan perilaku** untuk tenant mana pun yang
  tak menyentuh halaman baru.

## Di luar cakupan

- Field FLS lain (ARR, catatan internal, health score) — tetap seperti sekarang, tak
  disentuh.
- Account/Desa `contact_phone` — sudah dilepas total di BL-106, bukan dikonfigurasi.
- Konfigurabel per-field granular (HP terpisah dari WhatsApp) — keduanya di-gate identik
  di tiap site, jadi satu kebijakan "phone" mencakup keduanya; memisah menambah kolom
  tanpa kebutuhan.
