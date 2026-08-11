# Agent notes — detail yang direlokasi dari CLAUDE.md

File ini menampung detail lengkap + rasional yang di CLAUDE.md hanya diringkas
(demi menghemat context per-sesi). CLAUDE.md = ringkasan + trigger; file ini =
alasan "kenapa" + nuansa. Untuk keputusan arsitektur formal lihat juga
[`docs/decisions/`](decisions/).

## Gotcha (mahal — jangan temukan ulang)

1. **CSP wajib `unsafe-eval`.** Datastar eval ekspresi `data-*` via `new Function()`. Tanpa
   `script-src 'self' 'unsafe-eval'`, SELURUH Datastar mati senyap. Test regresi di `mw`.
2. **scs + Datastar SSE cookie.** `NewSSE` flush header dulu, bypass scs → `Set-Cookie` tak
   terkirim. Panggil `session.WriteCookie` SEBELUM `NewSSE`.
3. **SameSite=Lax, bukan Strict.** Callback OAuth = navigasi top-level cross-site; Strict
   menahan cookie → "state tidak valid".
4. **daisyUI = plugin Tailwind** (satu pipeline). `input.css`: `@import "tailwindcss"` +
   `@plugin "./daisyui.js"` → `make css` → `app.css`. Class komponen & token tema dari daisyUI
   TAPI tree-shaken (pakai di markup `.go` dulu). daisyUI TAK punya `bg-sidebar`/
   `text-muted-foreground`/`--color-destructive` (Basecoat lama) → padanan: sidebar=`bg-base-200`,
   muted=`text-base-content/70`, destructive=`error`. `daisyui.js` wajib ada saat build.
5. **`data.Class` key ber-hyphen wajib di-quote** — ditutup `ui.ClassOn`. Pakai helper, bukan
   `data.Class` mentah.
6. **`@post {contentType:'form'}` butuh `<form>` terdekat** — ditutup `ui.FormPostSelect`. Pakai
   helper, bukan `h.FormEl`+`@post` mentah.
7. **Toast wajib `pointer-events:none`** — `opacity:0` pun tetap tangkap klik.
8. **Super-admin env-override**: email di `SUPER_ADMIN_EMAILS` = super_admin efektif walau
   `role` DB = `user`. Reconcile boot promote-only. `RefreshIdentity` load role/status segar
   per-request (jangan cache di session).
9. **Panel dev-only** (`/dev/health`, `/dev/erd`) gated `devMode` di DUA tempat: route
   (`if devMode`) DAN menu (`devNav()`).
10. **Tema: daftar selaras 2 tempat** — `themeList` (`ui/theme.go`) & blok `themes:`
    (`static/input.css`). Tambah tema = edit KEDUANYA lalu `make css`.
11. **Hierarki permukaan warna**: latar halaman `bg-base-200`, permukaan (card/sidebar)
    `bg-base-100`. Token daisyUI RELATIF — kalau sama, kartu menyatu. Active-state
    `bg-primary text-primary-content`. Jangan warna absolut (`bg-white`, `bg-gray-*`).
12. **Chart = ECharts vendored + init eksternal** (BUKAN go-echarts — render `<script>` inline,
    diblokir CSP). `echarts.min.js` vendored + `charts.js` eksternal; option dirakit di Go
    (`internal/activity/charts.go`), ditanam via `<script type="application/json">` (CSP-safe),
    JS `JSON.parse`+`setOption`.
13. **Presence tracking (no write-storm).** `TrackPresence` UPSERT bucket 15-menit ke
    `activity_presence` (`hits+1`) + throttle 60 dtk/user, fail-soft, dipasang SETELAH
    `RefreshIdentity`. **Presence (agregat "kapan orang ada") ≠ audit (per-peristiwa "siapa
    melakukan apa") — jangan digabung**; dua tabel terpisah di `/dev/logs`. Retensi audit di
    `internal/maintenance` (harian, `settings.KeyAuditRetentionDays`, default 365 min 30; di luar
    batas → tolak & tak hapus; tak terparse → default & tetap jalan). **`target_type` menentukan
    tabel di-JOIN** saat baca (`user`/`session`→users, `workspace`→tenants, `platform`→tak JOIN)
    → salah = NAMA ORANG keliru. Pakai `h.audit`/`h.auditWorkspace`/`h.auditPlatform`, bukan
    `auditLog` telanjang. Kalimat peristiwa di `internal/activity/trail.go` (pure); action tak
    dikenal → kalimat netral + kode, jangan string kosong. Nama di-JOIN saat BACA, tak pernah
    disalin ke `metadata` (bebas PII).
14. **Timezone: simpan UTC, agregasi `AT TIME ZONE`.** Semua `created_at`/`bucket_at`
    TIMESTAMPTZ. Konversi lokal via `AT TIME ZONE $tz` di SQL; `APP_TIMEZONE` (default
    Asia/Jakarta) via `handler.SetAppTimezone`. **`import _ "time/tzdata"` di `main.go` WAJIB**
    (`CGO_ENABLED=0` tak punya tzdata OS). sqlc agregasi: `SUM/COUNT` bungkus
    `COALESCE(...)::bigint`; hindari `AT TIME ZONE` di SELECT list (sqlc emit `interface{}`) —
    kembalikan timestamptz mentah, format di Go.
15. **Response `content-type: text/javascript` = eksekusi JS** (fitur Datastar). Kita BERSIH
    (0 handler set itu). Jangan set content-type itu, jangan patch `<script>` berisi data user.
    Aturan Datastar: (a) escape SEMUA input user (`g.Text` bukan `g.Raw`; satu-satunya `g.Raw` =
    chart JSON dari `json.Marshal`); (b) signal user-modifiable → validasi backend (`ValidRoleName`,
    `TrimSpace`+empty-check); (c) jangan taruh data sensitif di signal.
16. **`sse.Redirect` DIBLOKIR CSP — pakai HTTP 303 untuk NAVIGASI.** `sse.Redirect`/
    `ExecuteScript`/`ReplaceURL` (datastar-go v1.2.2) suntik `<script>` inline → diblokir
    (`script-src` tanpa `unsafe-inline`) → redirect MATI. Aksi NAVIGASI (logout, login/
    register-sukses) → native form POST → `http.Redirect(w,r,url,303)` (commit cookie via scs
    `LoadAndSave`, tak perlu ritual #2). Gagal validasi → PRG `?err=CODE` (`authErrMsg`). SSE
    tetap untuk update FRAGMENT parsial ter-escape. `unsafe-inline` BUKAN solusi (`unsafe-eval`
    izinkan `new Function` tapi `<script>` inline tetap diblokir).

## Mode tenancy: single → multi (ratchet) — keputusan 0006 & 0007

ADR: [`0006-mode-aplikasi-single-vs-multi.md`](decisions/0006-mode-aplikasi-single-vs-multi.md),
[`0007-satu-dsn-dan-ratchet-tenancy.md`](decisions/0007-satu-dsn-dan-ratchet-tenancy.md).

Melayani DUA bentuk, **bukan dua jalur kode**. Lahir **single**, boleh **naik** ke multi; turun
**tak pernah**.

- **Single = multi-tenant N=1.** TEPAT SATU tenant; RLS/memberships/audit jalan. Yang hilang cuma
  chrome. Jangan buang `tenant_id`.
- **SATU bentuk URL** (`/w/{slug}`); aplikasi tunggal di **`/w/app`**. `SingleAppPrefix` DIHAPUS.
  `wsPath`/`slugFromRequest` tanpa cabang mode.
- **Mode di DATABASE** (`platform_settings.tenancy_mode`), bukan env. Nol baris = single.
  `APP_MODE` DIHAPUS. Penurunan ditolak DUA trigger: `BEFORE UPDATE` (multi→lain) & `BEFORE DELETE`
  (baris absen = single, hapus = penurunan menyamar).
- **Route SELALU didaftarkan** (bukan bersyarat mode). Zona bahaya dijaga `tenants.is_primary` di
  handler DAN SQL (`AND NOT is_primary`).
- **Workspace PRIMER = rumah aplikasi** (`is_primary` + unique partial index). Tak bisa diarsip/
  dihapus, tak makan kuota (kuota batasi yang DIBUAT). Pengecualian kuota wajib SAMA di sidebar &
  penegakan.
- **super_admin = OWNER workspace primer**, dipasang saat LOGIN (`ensurePrimaryOwner`, idempotent
  promote-only) — bukan boot (belum ada baris `users`). RLS dari ROLE, tak pernah dari keanggotaan.
  Cabut email dari `SUPER_ADMIN_EMAILS` → turun jadi owner biasa, bukan user biasa.
- **Pendaftar mode single = `member`** (`placeNewUser`), bukan owner.
- **Wewenang single**: super_admin(env)→fundamental; admin→operasional (termasuk nama app);
  member→pakai. `canEditWorkspace` longgar untuk admin di single (ganti nama app tak boleh menuntut
  sunting `.env`+restart).
- **`GuardSetRole` pakai `>=`** (bukan `>`) — cegah admin angkat sesama admin.
- **Boot** (`BootstrapPrimary`): workspace primer dibuat bila belum ada, mode dari DB. Cek lama
  ">1 workspace tapi single → tolak" DIHAPUS (mustahil).
- **Test WAJIB kedua mode** untuk jalur bergantung-mode (`withMode` + `t.Cleanup` di
  `appmode_test.go`). `setupTest` set MULTI eksplisit; default paket Single.
- **Naikkan mode di `/dev/settings`** (gate `platform:settings`). Konfirmasi = MENGETIK nama app
  (bukan checkbox). Setelah naik form HILANG diganti keterangan. Wajib ter-audit.

## Multi-tenancy (RLS + membership + role 2-bidang) — keputusan 0002, 0003 & 0004

ADR: [`0002-multi-tenancy-rls-role-2-bidang.md`](decisions/0002-multi-tenancy-rls-role-2-bidang.md),
[`0003-membership-multi-workspace.md`](decisions/0003-membership-multi-workspace.md),
[`0004-workspace-di-path-url.md`](decisions/0004-workspace-di-path-url.md),
[`0008-daftar-anggota-hanya-pengelola.md`](decisions/0008-daftar-anggota-hanya-pengelola.md).

Isolasi tenant = **Postgres RLS**, bukan cuma `WHERE tenant_id`. Keanggotaan = **membership**
(1 user ↔ banyak workspace, role beda). Workspace aktif di **PATH** (`/w/{slug}`), bukan session.

- **URL workspace HANYA lewat `wsPath`/`wsRedirect`** (`internal/handler/wspath.go`) — satu-satunya
  tempat literal `/w/` boleh muncul. Slug kosong → `/workspace/new`.
- **`/admin` & `/user` TIDAK ADA** — dilebur `/w/{slug}`. Beda role = beda AKSI di halaman sama,
  BUKAN beda ALAMAT. Pembatasan di handler (`canEditWorkspace`/`canManageMembers`), bukan route.
- **Slug asing → `http.NotFound`**, jangan 403 (konfirmasi ada) / redirect (bocor data lain).
- **Role PLATFORM pun wajib ikut slug** — cabang platform di `Scope` bypass RLS tapi TETAP panggil
  `adoptTenantBySlug`. Keputusan bypass dari ROLE, tak dari data.
- **`/dev`, `/notifications`, `/invite/{token}`, `/workspace/new` SENGAJA tanpa slug** — cakupan
  data bukan satu tenant.
- **DUA pintu ubah role harus SAMA.** `/w/{slug}/members` & `/dev/users` panggil `UpdateMemberRole`
  + `authz.GuardSetRole` sama; keduanya WAJIB `h.notify(..., "member.role.changed")`. Di `/dev`
  tenant notifikasi = workspace TARGET (form), bukan aktor. **Status & soft-delete SENGAJA tanpa
  notifikasi** (menutup pintu login → kabar in-app tak terbaca).
- **Daftar panjang wajib punya JALAN ke halaman berikutnya**, bukan cuma `LIMIT`. Ambil `pageSize+1`
  → `splitPage` → cursor `?after=` (`internal/handler/pagecursor.go`). **Keyset, bukan OFFSET**
  (`created_at DESC`, baris baru di atas → OFFSET menggeser). Cursor rusak → halaman pertama, JANGAN
  kosong. Navigasi = link `<a>` biasa (bookmark/reload, lolos #16).
- **Daftar anggota HANYA pengelola** (owner/admin/platform), kedua mode (0008 supersede 0004 §3).
  Gerbang di HANDLER (`canManageMembers` baris pertama `MembersPage`), bukan route. Ditolak **403 +
  penjelasan**, BUKAN 404 (penerima terbukti anggota). Menu `Anggota` pakai izin SAMA; `workspaceNav`
  terima `canMembers` & `canSettings` terpisah (tak identik di single).
- **PII disamarkan di HANDLER, bukan view** (di view → alamat asli tetap dioper, bocor di
  view-source). `maskEmail` dipertahankan (0008 §5): domain DIPERTAHANKAN, panjang lokal TIDAK
  dibocorkan, email sendiri utuh.
- **Penanda orang = NAMA (`users.name`)**, email cadangan. Nama refresh tiap login
  (`UpdateUserProfile`, COALESCE: NULL = "provider diam" bukan "hapus"). User-controlled →
  `oauth.NormalizeDisplayName` (potong batas RUNE), TAK PERNAH penanda unik/otorisasi (`id` yang
  dipakai). **Kelak** "edit profil" tertimpa refresh → nama pilihan sendiri harus kolom terpisah
  yang diutamakan.
- **View TAK BOLEH rakit path workspace sendiri** — oper `base` dari handler (`panel.Members`,
  `panel.WorkspaceView.Base`). Pelanggaran senyap (render rapi, ketahuan saat SUBMIT). **Verifikasi
  UI harus mencakup submit.**

## Siklus hidup workspace (suspend · archive · delete) — keputusan 0005

ADR: [`0005-siklus-hidup-workspace.md`](decisions/0005-siklus-hidup-workspace.md).

`tenants.status` = `active|suspended|archived` + `deleted_at` (soft-delete, tenggang 30 hari).
Ditegakkan `gateLifecycle` (dipanggil `Scope`).

- **Bedanya KEWENANGAN**: `suspended` = tindakan PLATFORM (owner tak bisa batalkan sendiri);
  `archived` = keputusan OWNER (bisa dibuka lagi). Guard `status='active'` di `ArchiveTenant` cegah
  owner keluar suspensi lewat arsip→unarsip.
- **Kode status**: bukan-anggota → 404 · suspended → 403 + alasan · archived → GET lolos non-GET 403
  · deleted → 404.
- **Platform SENGAJA tembus gerbang** (cabang platform di `Scope` tak panggil `gateLifecycle`).
  Dikunci `TestPlatformTembusSuspensi`.
- **Unarchive DI LUAR `/w/{slug}`** (`/workspace/{slug}/unarchive`; gerbang read-only blok POST di
  dalam). Handler cari tenant sendiri, **wajib lewat `tenantBySlug` untuk platform** (bukan
  `resolveTenantBySlug` yang syaratkan keanggotaan → platform bukan anggota → 404 salah).
- **`audit_logs.tenant_id` NULLABLE + `ON DELETE SET NULL`** (dulu NOT NULL tanpa CASCADE →
  `DELETE FROM tenants` gagal di tengah).
- **Kuota**: terhapus TAK dihitung; terarsip TETAP dihitung (arsip bukan celah kuota).
- **Slug tak dilepas saat terhapus** (kalau dilepas → restore mustahil).
- **Purge TERJADWAL** (`internal/maintenance`, dari `main.go`). Runner IN-PROCESS, dikunci
  `pg_try_advisory_lock` (id 4243, beda dari migrasi 4242) — `try` bukan blocking. Purge SATU PER
  SATU (bukan DELETE massal). Ditunda 5 menit setelah boot, berhenti saat SIGTERM.
- **`h.q(ctx)`, JANGAN `h.DB`** (dihapus). `h.q` ambil `*db.Queries` ber-tenant dari `Scope`. Lupa
  Scope = **panic keras** (bug wiring ketahuan seketika). Jalur pre-identity (auth/oauth/boot) pakai
  `db.WithSuper` eksplisit.
- **`WithTenant`/`WithSuper` = SATU tx dgn GUC `set_config(...,true)` TX-LOCAL + `SET LOCAL ROLE
  app_rw`** (`internal/db/tenant.go`). `,true` wajib (plain `SET` bocor ke peminjam pool berikutnya).
  Keputusan bypass dari ROLE (`isPlatformRole`), tak pernah dari data.
- **SATU DSN, hak diturunkan PER-TRANSAKSI (0007).** `FORCE` RLS wajib (tanpanya owner tabel bypass
  diam-diam). `dropPrivileges` jalankan `SET LOCAL ROLE app_rw` di dalam tx (dulu butuh
  `APP_DATABASE_URL`, DIHAPUS). **Migrasi WAJIB `GRANT app_rw TO CURRENT_USER`**. Risiko diterima:
  injection berhasil bisa `RESET ROLE` — semua query digenerate sqlc; **timbang ulang bila ada SQL
  mentah dari user.**
- **super_admin = ENV-ONLY, nol baris DB.** Role efektif overlay `RefreshIdentity` per-request (env
  → `platform_staff` → `memberships.role`). `platform_staff` TANPA RLS. Tak ada `PromoteSuperAdmins`.
- **MEMBERSHIP: 1 user ↔ banyak workspace.** Role di `memberships` (user × tenant × role), BUKAN
  `users`. `users` = tabel GLOBAL (identitas murni, tanpa `tenant_id`/`role`, KELUAR dari RLS) →
  `ListUsers` (/dev) lintas-workspace; daftar anggota pakai `ListMembersByTenant`.
- **`notifications` TANPA RLS — SEMANTIK**: notifikasi milik USER & lintas-workspace (undangan datang
  dari workspace yang belum jadi miliknya). `tenant_id` = konteks tampilan (nullable). **Jangan
  `h.q(ctx)`** — pakai `db.WithSuper` + `WHERE user_id/email`, test isolasi antar-user pengganti RLS.
- **`memberships`/`invites` TANPA RLS** — dibaca untuk MENENTUKAN scope (chicken-and-egg); invite di
  jalur publik. Keamanan dari filter query, bukan RLS.
- **`Scope` MEMVALIDASI keanggotaan** sebelum `WithTenant` (`resolveActiveTenant`) — tenant di session
  user-controlled. Tak valid → fallback workspace pertama; tanpa workspace → `/workspace/new`.
- **Kuota = DUA lapis** (`internal/settings`): default global (`platform_settings`, ubah dari
  `/dev/settings` tanpa restart) + override per-user (`users.workspace_quota`). **`NULL` = ikut
  global**, angka = hak khusus kebal perubahan global. `MAX_WORKSPACES_PER_USER` = fallback saat baris
  DB belum ada. Hitung HANYA lewat `settings.EffectiveWorkspaceQuota` (penegakan & tampilan sumber
  sama). Hanya workspace ber-role **owner** belum terhapus dihitung. `CountTenantOwners` cegah owner
  terakhir diturunkan.
- **`/dev/settings` gate `platform:settings`, BUKAN `dev:users`** (grup `/dev` gated `dev:users` milik
  staff; tanpa objek Casbin tersendiri, staff bisa ubah aturan semua user). `devNav` juga cek izin ini.
  Deny-default; hanya super_admin lolos.
- **Cache settings = PER-PROSES** (instance lain menyusul saat boot). Kelak butuh serempak: pub/sub
  Redis.
- **Register/OAuth = user + workspace + membership owner** dalam SATU tx `WithSuper` (atomik).
  `startIdentity(preferTenant)` pilih workspace aktif.
- **Audit di tx `WithSuper` TERPISAH** dari Scope tx (fail-soft struktural). `tenant_id` audit =
  tenant aktor.
- **Test isolasi RLS** (`rls_test.go`) konek `app_rw` non-superuser via `SET ROLE` di `AfterConnect`.
  Test handler lain konek superuser (RLS di-bypass). Seed pakai owner-pool.
- **Casbin CSV TAK dukung komentar inline** di akhir baris `g,`/`p,` (jadi bagian nilai → link mati).
  Komentar di baris `#` tersendiri.

## Konfigurasi produksi: gagal keras, jangan andalkan ingatan

Prinsip: **yang berbahaya bila salah harus menggagalkan boot; yang bisa diturunkan otomatis jangan
diminta ke manusia.** Warning di log bukan pengaman.

- **Gagal dengan PETUNJUK** (`internal/preflight`, dipanggil awal `run()` & `make doctor` — sumber
  SAMA). Tiap `Problem` WAJIB punya `Fix` (nama yang dicari, DB mirip yang ada, perintah persis).
  Semua masalah dikumpulkan sekaligus.
- **DB dibuat otomatis HANYA di dev** (`AutoCreateDB: !cfg.IsProduction()`). Di produksi, DSN salah
  ketik jadi DB kosong yang tampak sehat. `make doctor` tak pernah membuatnya (diagnosis tak boleh
  ubah keadaan).
- **Isolasi tenant DIBUKTIKAN** (`db.CheckRLSTx`, `verifyTenantIsolation` di `main.go`) DI DALAM
  `WithSuper` (tx yang sudah turun hak). Periksa `rolsuper`/`rolbypassrls`/pemilik-tabel + `FORCE
  RLS`. Sama di dev & produksi. Bukti eksplisit ini menggantikan cek lama "env `APP_DATABASE_URL`
  terisi" yang lolos sambil tetap bocor (DSN sama seperti `DATABASE_URL`).
- **Produksi tak butuh persiapan role manual.** Migrasi buat `app_rw` + GRANT + `ALTER DEFAULT
  PRIVILEGES` + `GRANT app_rw TO CURRENT_USER`. Role `NOLOGIN` (tak ada password/entri PgBouncer).
  `GENERATED ALWAYS AS IDENTITY` tak butuh GRANT sequence terpisah.
- **`SESSION_KEY` divalidasi PANJANG** (min 32, `config.MinSessionKeyLen`), bukan cuma keberadaan
  (kunci lemah lebih bahaya dari kosong). Kini DIPAKAI: turunkan nama cookie sesi
  (`Config.SessionCookieName`, pakai HASH-nya) agar dua deployment di host sama tak saling timpa sesi.
- **`Cookie.Secure` diturunkan dari `ENV=production`**, bukan env sendiri. Konsekuensi: produksi tanpa
  HTTPS = login mati (gagal keras, bukan bocor senyap).
- **Jangan tambah env baru untuk hal yang bisa diturunkan** dari env yang ada.
