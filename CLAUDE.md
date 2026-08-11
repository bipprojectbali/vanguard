# CLAUDE.md — panduan agen untuk go_starter

Baca ini + [`STARTER.md`](STARTER.md) (spec & alasan arsitektur) sebelum kode.
README = cara pakai; file ini = konvensi + **gotcha yang mahal ditemukan ulang**.

## Alur kerja wajib

- **`make check` = gerbang** (sqlc · vet · gofmt · build · test) — harus hijau
  sebelum lapor selesai.
- **Tiap paket test punya SCHEMA Postgres sendiri** (`internal/testdb`) → `go test
  ./...` paralel. Paket ber-DB wajib `TestMain` yang panggil `testdb.Pool(ctx,
  "<nama>")` + `testdb.Drop`; test ambil pool dari var paket (`pkgPool`), JANGAN
  buka `pgxpool.New` sendiri (DSN mentah → `public`, tabel tak ada di sana).
  `search_path` sengaja TANPA `public`: dengan `public`, goose menemukan
  `goose_db_version` DB utama → kira "sudah versi terakhir" → schema baru tetap
  KOSONG. Migrasi `00004` GRANT `app_rw` ke `current_schema()` (bukan `public`
  harfiah) agar RLS mengikat di schema mana pun.
- Tooling (`sqlc`/`goose`/`air`) di `$(go env GOPATH)/bin`; Makefile panggil via
  path absolut (`$(GOBIN)/sqlc`) — GNU Make 3.81 macOS exec via `execvp`, `export
  PATH` tak terbaca. Jangan ubah ke pemanggilan telanjang.
- Ubah schema → tulis migrasi goose → jalankan lokal → `sqlc generate` → perbaiki
  ripple. Jangan edit `internal/db/*` (generated).
- **Skema hidup di SATU migrasi** (`00001_schema.sql`, 11 migrasi disatukan saat
  belum ada deployment). Perubahan berikutnya inkremental (`00002_...`); jangan
  sunting `00001` setelah repo di-clone orang, jangan satukan ulang setelah produksi.
- Fitur baru wajib disertai test dalam pekerjaan yang sama. Test pakai
  `TEST_DATABASE_URL` (Postgres, jangan tukar engine), `skip` bila kosong.

## Arsitektur & konvensi

- **`routes.go` = single source of truth** semua route. Middleware terproteksi:
  `RequireAuth` → `Scope` → `RefreshIdentity` → `TrackPresence` →
  `RequireEnforce(obj, act)`. `Scope` buka tx ber-tenant SEBELUM
  `RefreshIdentity`/`TrackPresence` (keduanya pakai `h.q(ctx)`).
- **Ini TEMPLATE yang di-clone.** Nama project/app tak boleh di-hardcode: module
  path via `make rename name=X`, nama tampil dari `APP_NAME` via
  `handler.SetAppName` (`devBrand()` sidebar `/dev`, `LayoutData.Brand` header).
  Test brand pakai `devBrand()`, BUKAN string harfiah. Target `dev`/`css`
  bergantung `tailwind` (binary 76MB, gitignored) agar clone baru langsung jalan.
- **Query yang saring `table_schema` WAJIB `current_schema()`**, bukan `'public'`
  harfiah (sama seperti GRANT migrasi 00004). Pernah bikin ERD kosong di schema
  non-public.
- **Config dibaca HANYA di `internal/config`** — jangan `os.Getenv` tersebar.
- **Handler tak simpan config**; inject via setter global saat startup
  (`SetCSSPath`, `SetDevMode`, `SetGoogleOAuth`, `SetSuperAdminChecker`,
  `SetAppTimezone`, `session.Init`, `authz.Init`). Ikuti pola ini.
- **Atribut Datastar via helper bertipe** (`internal/ui/dsx.go`): `ClassOn`
  (class-toggle auto-quote), `FormPostSelect` (@post form-valued + `<form>`),
  `PostAction`/`DeleteAction`. Menutup gotcha #5 & #6 struktural — jangan tulis
  `data.Class`/`@post` mentah di view.
- **View murni-data**: gomponents terima data siap-render; jangan panggil
  `authz.Can`/session dari dalamnya. Precompute flag di handler, oper ke view
  (`ui.When`, `quickLinksFor`).
- **Dua jalur render**: `renderPage` (Layout landing/app) vs `renderShell`
  (AppShell + sidebar). `headNodes()` dibagi keduanya.

## Desain mobile-first (WAJIB — template di-clone, kelalaian menular ke turunan)

- **Mobile-first**: kelas dasar (tanpa prefix) = MOBILE; naikkan `sm:`/`md:`/`lg:`.
  ✅ `grid-cols-1 md:grid-cols-2` ❌ `grid-cols-2 max-md:grid-cols-1`.
- **Breakpoint** Tailwind: `sm` 640 · `md` 768 · `lg` 1024. Sidebar = drawer di
  `<md`, tetap di `md:`+. **AppShell (`internal/ui/appshell.go`) = pola acuan**
  (`-translate-x-full` + `md:translate-x-0`). Jangan bikin mekanisme baru.
- **Nol overflow horizontal** di 320–375px. Grid/flex → 1 kolom di mobile. Konten
  lebar pakai `truncate`/`break-words`.
- **SETIAP `<table>` bungkus `ui.TableScroll`** (ada test regresi
  `tablescroll_test.go`). Dua hal WAJIB bersama: (a) `overflow-x-auto` di
  pembungkus LANGSUNG tabel (di `.card-body` tak menahan), (b) `min-w-0`
  (card/flex-item default `min-width:auto` menolak menyusut). Tanpa ini satu
  tabel telanjang meluber 439px di viewport 375px.
- **Baris tombol horizontal wajib `flex-wrap`** (pagination dorong halaman di
  375px; `flex-wrap` di wrapper luar tak menurun ke baris dalam).
- **Diagnosis overflow**: ukur `documentElement.scrollWidth` vs `clientWidth`,
  daftar elemen `getBoundingClientRect().right > vw` yang TANPA
  `closest('.overflow-x-auto')` — itu pelakunya.
- **Tap target ≥ 44px**; input `text-base` (≥16px) agar iOS tak auto-zoom.
- **VERIFIKASI 3 LEBAR WAJIB** sebelum lapor selesai untuk perubahan UI: skill
  **ego-browser**, set 375/768/1280 + screenshot tiap lebar. Tak ada
  `set_viewport` — pakai CDP `Emulation.setDeviceMetricsOverride` lalu
  `clearDeviceMetricsOverride`.

## Identitas panel (/w/{slug} · /dev)

Semua halaman pakai AppShell SAMA. **`/dev` menampilkan data LINTAS-workspace**
(`ListUsers` lihat semua tenant) → salah kira panel = salah baca cakupan data.

- **Sumber `internal/ui/panelkind.go`** (`Panel` bertipe + `panelStyles` array
  berindeks enum → tambah panel tanpa gaya = compile error). `panelOf(ctx,
  currentPath)` menurunkannya.
- **Hanya `/dev` ditentukan PATH.** `/w/{slug}` melayani semua role → chip
  diturunkan dari ROLE (`panelForRole`, sumber sama dengan `navFor`). Owner/admin
  → `ADMIN`, member → `RUANG KERJA`.
- **DUA penanda sengaja**: chip TEKS + aksen warna tepi atas sidebar. Saat sidebar
  collapse jadi rail 4rem, `.app-brand` disembunyikan (chip hilang) → aksen tepi
  bertahan. Jangan hapus salah satu.
- **Warna WAJIB token semantik daisyUI** (`primary`/`secondary`/`warning`), bukan
  absolut. `/dev` pakai `warning` sebagai peringatan.
- Chip MENGGANTIKAN sub-label panel, tak menumpuk. Brand = NAMA WORKSPACE atau
  `go_starter /dev`.

## Gotcha (mahal — jangan temukan ulang)

1. **CSP wajib `unsafe-eval`.** Datastar eval ekspresi `data-*` via `new
   Function()`. Tanpa `script-src 'self' 'unsafe-eval'`, SELURUH Datastar mati
   senyap. Ada test regresi di `mw`.
2. **scs + Datastar SSE cookie.** `NewSSE` flush header lebih dulu, bypass scs →
   `Set-Cookie` tak terkirim. Panggil `session.WriteCookie` SEBELUM `NewSSE`.
3. **SameSite=Lax, bukan Strict.** Callback OAuth = navigasi top-level cross-site;
   Strict menahan cookie → "state tidak valid".
4. **daisyUI = plugin Tailwind** (satu pipeline). `input.css` `@import
   "tailwindcss"` + `@plugin "./daisyui.js"` → `make css` → `app.css`. Semua class
   komponen & token tema dari daisyUI, TAPI tree-shaken (pakai di markup `.go`
   dulu). daisyUI TAK punya `bg-sidebar`/`text-muted-foreground`/
   `--color-destructive` (Basecoat lama) → padanan: sidebar=`bg-base-200`,
   muted=`text-base-content/70`, destructive=`error`. `daisyui.js` wajib ada saat build.
5. **`data.Class` key ber-hyphen wajib di-quote** — **ditutup `ui.ClassOn`**. Pakai
   helper, bukan `data.Class` mentah.
6. **`@post {contentType:'form'}` butuh `<form>` terdekat** — **ditutup
   `ui.FormPostSelect`**. Pakai helper, bukan `h.FormEl`+`@post` mentah.
7. **Toast wajib `pointer-events:none`** — `opacity:0` pun tetap tangkap klik.
8. **Super-admin env-override**: email di `SUPER_ADMIN_EMAILS` = super_admin
   efektif walau `role` DB = `user`. Reconcile boot promote-only.
   `RefreshIdentity` load role/status segar per-request (jangan cache di session).
9. **Panel dev-only** (`/dev/health`, `/dev/erd`) gated `devMode` di DUA tempat:
   route (`if devMode`) DAN menu (`devNav()`).
10. **Tema: daftar selaras 2 tempat** — `themeList` (`ui/theme.go`) & blok
    `themes:` (`static/input.css`). Tambah tema = edit KEDUANYA lalu `make css`.
11. **Hierarki permukaan warna**: latar halaman `bg-base-200`, permukaan
    (card/sidebar) `bg-base-100`. Token daisyUI RELATIF — kalau sama, kartu
    menyatu. Active-state pakai `bg-primary text-primary-content`. Jangan warna
    absolut (`bg-white`, `bg-gray-*`).
12. **Chart = ECharts vendored + init eksternal** (BUKAN go-echarts — render
    `<script>` inline, diblokir CSP). Pola: `echarts.min.js` vendored + `charts.js`
    eksternal; option dirakit di Go (`internal/activity/charts.go`), ditanam via
    `<script type="application/json">` (CSP-safe), JS `JSON.parse`+`setOption`.
13. **Presence tracking (no write-storm).** `TrackPresence` UPSERT bucket 15-menit
    ke `activity_presence` (`hits+1`) + throttle 60 dtk/user, fail-soft, dipasang
    SETELAH `RefreshIdentity`. **Presence ("kapan orang ada", agregat) ≠ audit
    ("siapa melakukan apa", per-peristiwa) — jangan digabung**; dua tabel terpisah
    di `/dev/logs`. Retensi audit di `internal/maintenance` (harian,
    `settings.KeyAuditRetentionDays`, default 365 min 30). Angka di luar batas →
    tolak & tak hapus apa pun; nilai tak terparse → default & tetap jalan.
    **`target_type` menentukan tabel di-JOIN** saat baca
    (`user`/`session`→users, `workspace`→tenants, `platform`→tak JOIN) → salah =
    NAMA ORANG keliru. Pakai `h.audit`/`h.auditWorkspace`/`h.auditPlatform`, bukan
    `auditLog` telanjang. Kalimat peristiwa di `internal/activity/trail.go` (pure);
    action tak dikenal → kalimat netral + kode, jangan string kosong. Nama di-JOIN
    saat BACA, tak pernah disalin ke `metadata` (bebas PII).
14. **Timezone: simpan UTC, agregasi `AT TIME ZONE`.** Semua `created_at`/
    `bucket_at` TIMESTAMPTZ. Konversi lokal via `AT TIME ZONE $tz` di SQL;
    `APP_TIMEZONE` (default Asia/Jakarta) via `handler.SetAppTimezone`. **`import _
    "time/tzdata"` di `main.go` WAJIB** (`CGO_ENABLED=0` tak punya tzdata OS). sqlc
    agregasi: `SUM/COUNT` bungkus `COALESCE(...)::bigint`; hindari `AT TIME ZONE`
    di SELECT list (sqlc emit `interface{}`) — kembalikan timestamptz mentah,
    format di Go.
15. **Response `content-type: text/javascript` = eksekusi JS** (fitur Datastar).
    Kita BERSIH (0 handler set itu). Jangan set content-type itu, jangan patch
    `<script>` berisi data user. Aturan Datastar dipatuhi: (a) escape SEMUA input
    user (`g.Text`, bukan `g.Raw`; satu-satunya `g.Raw` = chart JSON dari
    `json.Marshal`); (b) signal user-modifiable → validasi backend
    (`ValidRoleName`, `TrimSpace`+empty-check); (c) jangan taruh data sensitif di signal.
16. **`sse.Redirect` DIBLOKIR CSP — pakai HTTP 303 untuk NAVIGASI.** `sse.Redirect`/
    `ExecuteScript`/`ReplaceURL` (datastar-go v1.2.2) menyuntik `<script>` inline →
    diblokir (`script-src` tanpa `unsafe-inline`) → redirect MATI. **Aturan**: aksi
    NAVIGASI (logout, login/register-sukses) → native form POST →
    `http.Redirect(w,r,url,303)`. Native redirect commit cookie via scs
    `LoadAndSave` (tak perlu ritual #2). Gagal validasi → PRG `?err=CODE`
    (`authErrMsg`). SSE tetap untuk update FRAGMENT parsial ter-escape. `unsafe-eval`
    izinkan `new Function` TAPI `<script>` inline tetap diblokir → `unsafe-inline`
    BUKAN solusi.

## Mode tenancy: single → multi (ratchet) — keputusan 0006 & 0007

Template melayani DUA bentuk, **bukan dua jalur kode**. Lahir **single**, boleh
**naik** ke multi; turun **tak pernah**.

- **Single = multi-tenant N=1.** Tetap ada TEPAT SATU tenant; RLS/memberships/audit
  jalan. Yang hilang cuma chrome. Jangan buang `tenant_id`.
- **SATU bentuk URL** (`/w/{slug}`); aplikasi tunggal di **`/w/app`**.
  `SingleAppPrefix` DIHAPUS. `wsPath`/`slugFromRequest` tanpa cabang mode.
- **Mode di DATABASE** (`platform_settings.tenancy_mode`), bukan env. Nol baris =
  single. `APP_MODE` DIHAPUS. Penurunan ditolak DUA trigger: `BEFORE UPDATE`
  (multi→lain) & `BEFORE DELETE` (baris absen = single, hapus = penurunan menyamar).
- **Route SELALU didaftarkan** (bukan bersyarat mode). Zona bahaya dijaga
  `tenants.is_primary` di handler DAN SQL (`AND NOT is_primary`).
- **Workspace PRIMER = rumah aplikasi** (`is_primary` + unique partial index). Tak
  bisa diarsip/dihapus, tak makan kuota (kuota batasi yang DIBUAT). Pengecualian
  kuota wajib SAMA di sidebar & penegakan.
- **super_admin = OWNER workspace primer**, dipasang saat LOGIN (`ensurePrimaryOwner`,
  idempotent promote-only) — bukan boot (belum ada baris `users`). Jalur RLS dari
  ROLE, tak pernah dari keanggotaan. Cabut email dari `SUPER_ADMIN_EMAILS` →
  turun jadi owner biasa, bukan user biasa.
- **Pendaftar mode single = `member`** (`placeNewUser`), bukan owner.
- **Wewenang single**: super_admin(env)→fundamental; admin→operasional (termasuk
  nama app); member→pakai. `canEditWorkspace` longgar untuk admin di single (admin
  = pembantu operasional; ganti nama app tak boleh menuntut sunting `.env`+restart).
- **`GuardSetRole` pakai `>=`** (bukan `>`) — cegah admin angkat sesama admin.
- **Boot** (`BootstrapPrimary`): workspace primer dibuat bila belum ada, mode dari
  DB. Cek lama ">1 workspace tapi single → tolak" DIHAPUS (keadaan itu mustahil).
- **Test WAJIB kedua mode** untuk jalur bergantung-mode (`withMode` + `t.Cleanup`
  di `appmode_test.go`). `setupTest` set MULTI eksplisit; default paket Single.
- **Naikkan mode di `/dev/settings`** (gate `platform:settings`). Konfirmasi =
  MENGETIK nama app (bukan checkbox). Setelah naik form HILANG diganti keterangan.
  Wajib ter-audit.

## Multi-tenancy (RLS + membership + role 2-bidang) — keputusan 0002, 0003 & 0004

Isolasi tenant = **Postgres RLS**, bukan cuma `WHERE tenant_id`. Keanggotaan =
**membership** (1 user ↔ banyak workspace, role beda). Workspace aktif di **PATH**
(`/w/{slug}`), bukan session.

- **URL workspace HANYA lewat `wsPath`/`wsRedirect`** (`internal/handler/wspath.go`)
  — satu-satunya tempat literal `/w/` boleh muncul. Slug kosong → `/workspace/new`.
- **`/admin` & `/user` TIDAK ADA** — dilebur `/w/{slug}`. Beda role = beda AKSI di
  halaman sama, BUKAN beda ALAMAT. Pembatasan di handler
  (`canEditWorkspace`/`canManageMembers`), bukan route.
- **Slug asing → `http.NotFound`**, jangan 403 (konfirmasi ada) / redirect (bocor
  data workspace lain).
- **Role PLATFORM pun wajib ikut slug** — cabang platform di `Scope` bypass RLS
  tapi TETAP panggil `adoptTenantBySlug`. Keputusan bypass dari ROLE, tak dari data.
- **`/dev`, `/notifications`, `/invite/{token}`, `/workspace/new` SENGAJA tanpa
  slug** — cakupan data bukan satu tenant.
- **DUA pintu ubah role harus SAMA.** `/w/{slug}/members` & `/dev/users` panggil
  `UpdateMemberRole` + `authz.GuardSetRole` sama; keduanya WAJIB `h.notify(...,
  "member.role.changed")`. Di `/dev` tenant notifikasi = workspace TARGET (form),
  bukan aktor. **Status & soft-delete SENGAJA tanpa notifikasi** (menutup pintu
  login → kabar in-app tak terbaca).
- **Daftar panjang wajib punya JALAN ke halaman berikutnya**, bukan cuma `LIMIT`.
  Pola: ambil `pageSize+1` → `splitPage` → cursor `?after=`
  (`internal/handler/pagecursor.go`). **Keyset, bukan OFFSET** (`created_at DESC`,
  baris baru di atas → OFFSET menggeser). Cursor rusak → halaman pertama, JANGAN
  kosong. Navigasi = link `<a>` biasa (bisa bookmark/reload, lolos #16).
- **Daftar anggota HANYA untuk pengelola** (owner/admin/platform), kedua mode
  (0008 supersede 0004 §3). Gerbang di HANDLER (`canManageMembers` baris pertama
  `MembersPage`), bukan route. Ditolak **403 + penjelasan**, BUKAN 404 (penerima
  terbukti anggota). Menu `Anggota` pakai izin SAMA; `workspaceNav` terima
  `canMembers` & `canSettings` terpisah (tak identik di single).
- **PII disamarkan di HANDLER, bukan view** (kalau di view, alamat asli tetap
  dioper → bocor di view-source). `maskEmail` dipertahankan (0008 §5): aturan
  tampilan ≠ aturan akses. Domain DIPERTAHANKAN, panjang lokal TIDAK dibocorkan,
  email sendiri utuh.
- **Penanda orang = NAMA (`users.name`)**, email cadangan. Nama refresh tiap login
  (`UpdateUserProfile`, COALESCE: NULL = "provider diam" bukan "hapus").
  User-controlled → `oauth.NormalizeDisplayName` (potong pada batas RUNE), TAK
  PERNAH penanda unik/otorisasi (`id` yang dipakai). **Kelak** "edit profil" akan
  tertimpa refresh → nama pilihan sendiri harus kolom terpisah yang diutamakan.
- **View TAK BOLEH rakit path workspace sendiri** — oper `base` dari handler
  (`panel.Members`, `panel.WorkspaceView.Base`). Pelanggaran senyap (render rapi,
  ketahuan saat SUBMIT). **Verifikasi UI harus mencakup submit.**

## Siklus hidup workspace (suspend · archive · delete) — keputusan 0005

`tenants.status` = `active|suspended|archived` + `deleted_at` (soft-delete, tenggang
30 hari). Ditegakkan `gateLifecycle` (dipanggil `Scope`).

- **Bedanya KEWENANGAN**: `suspended` = tindakan PLATFORM (owner tak bisa batalkan
  sendiri); `archived` = keputusan OWNER (bisa dibuka lagi). Guard `status='active'`
  di `ArchiveTenant` cegah owner keluar suspensi lewat arsip→unarsip.
- **Kode status**: bukan-anggota → 404 · suspended → 403 + alasan · archived → GET
  lolos non-GET 403 · deleted → 404.
- **Platform SENGAJA tembus gerbang** (cabang platform di `Scope` tak panggil
  `gateLifecycle`). Dikunci `TestPlatformTembusSuspensi`.
- **Unarchive DI LUAR `/w/{slug}`** (`/workspace/{slug}/unarchive`; gerbang
  read-only blok POST di dalam). Handler cari tenant sendiri, **wajib lewat
  `tenantBySlug` untuk platform** (bukan `resolveTenantBySlug` yang syaratkan
  keanggotaan → platform bukan anggota → 404 salah).
- **`audit_logs.tenant_id` NULLABLE + `ON DELETE SET NULL`** (dulu NOT NULL tanpa
  CASCADE → `DELETE FROM tenants` gagal di tengah).
- **Kuota**: terhapus TAK dihitung; terarsip TETAP dihitung (arsip bukan celah kuota).
- **Slug tak dilepas saat terhapus** (kalau dilepas → restore mustahil).
- **Purge TERJADWAL** (`internal/maintenance`, dari `main.go`). Runner IN-PROCESS.
  Dikunci `pg_try_advisory_lock` (id 4243, beda dari migrasi 4242) — `try` bukan
  blocking. Purge SATU PER SATU (bukan DELETE massal). Ditunda 5 menit setelah boot,
  berhenti saat SIGTERM.
- **`h.q(ctx)`, JANGAN `h.DB`** (dihapus). `h.q` ambil `*db.Queries` ber-tenant dari
  `Scope`. Lupa Scope = **panic keras** (bug wiring ketahuan seketika). Jalur
  pre-identity (auth/oauth/boot) pakai `db.WithSuper` eksplisit.
- **`WithTenant`/`WithSuper` = SATU tx dgn GUC `set_config(...,true)` TRANSACTION-
  LOCAL + `SET LOCAL ROLE app_rw`** (`internal/db/tenant.go`). `,true` wajib (plain
  `SET` bocor ke peminjam pool berikutnya — kebocoran tenant #1). Keputusan bypass
  dari ROLE (`isPlatformRole`), tak pernah dari data.
- **SATU DSN, hak diturunkan PER-TRANSAKSI (0007).** `FORCE` RLS wajib (tanpanya
  owner tabel bypass diam-diam). `dropPrivileges` jalankan `SET LOCAL ROLE app_rw`
  di dalam tx (dulu butuh `APP_DATABASE_URL`, DIHAPUS). **Migrasi WAJIB `GRANT
  app_rw TO CURRENT_USER`**. Risiko diterima: injection berhasil bisa `RESET ROLE`
  — semua query digenerate sqlc; **timbang ulang bila ada SQL mentah dari user.**
- **super_admin = ENV-ONLY, nol baris DB.** Role efektif overlay `RefreshIdentity`
  per-request (env → `platform_staff` → `memberships.role`). `platform_staff`
  TANPA RLS. Tak ada `PromoteSuperAdmins`.
- **MEMBERSHIP: 1 user ↔ banyak workspace.** Role di `memberships` (user × tenant ×
  role), BUKAN `users`. `users` = tabel GLOBAL (identitas murni, tanpa
  `tenant_id`/`role`, KELUAR dari RLS) → `ListUsers` (/dev) lintas-workspace; daftar
  anggota pakai `ListMembersByTenant`.
- **`notifications` TANPA RLS — SEMANTIK**: notifikasi milik USER & lintas-workspace
  (undangan datang dari workspace yang belum jadi miliknya). `tenant_id` = konteks
  tampilan (nullable). **Jangan `h.q(ctx)`** — pakai `db.WithSuper` + `WHERE
  user_id/email`, test isolasi antar-user sebagai pengganti RLS.
- **`memberships`/`invites` TANPA RLS** — dibaca untuk MENENTUKAN scope
  (chicken-and-egg); invite di jalur publik. Keamanan dari filter query, bukan RLS.
- **`Scope` MEMVALIDASI keanggotaan** sebelum `WithTenant` (`resolveActiveTenant`)
  — tenant di session user-controlled. Tak valid → fallback workspace pertama;
  tanpa workspace → `/workspace/new`. Validasi HARUS di Scope.
- **Kuota = DUA lapis** (`internal/settings`): default global (`platform_settings`,
  ubah dari `/dev/settings` tanpa restart) + override per-user
  (`users.workspace_quota`). **`NULL` = ikut global**, angka = hak khusus kebal
  perubahan global. `MAX_WORKSPACES_PER_USER` = fallback saat baris DB belum ada.
  Hitung HANYA lewat `settings.EffectiveWorkspaceQuota` (penegakan & tampilan
  sumber sama). Hanya workspace ber-role **owner** belum terhapus dihitung.
  `CountTenantOwners` cegah owner terakhir diturunkan.
- **`/dev/settings` gate `platform:settings`, BUKAN `dev:users`** (grup `/dev`
  gated `dev:users` milik staff; tanpa objek Casbin tersendiri, staff bisa ubah
  aturan semua user). `devNav` juga cek izin ini. Deny-default; hanya super_admin lolos.
- **Cache settings = PER-PROSES** (instance lain menyusul saat boot). Kalau kelak
  butuh serempak: pub/sub Redis.
- **Register/OAuth = user + workspace + membership owner** dalam SATU tx `WithSuper`
  (atomik). `startIdentity(preferTenant)` pilih workspace aktif.
- **Audit di tx `WithSuper` TERPISAH** dari Scope tx (fail-soft struktural).
  `tenant_id` audit = tenant aktor.
- **Test isolasi RLS** (`rls_test.go`) konek `app_rw` non-superuser via `SET ROLE`
  di `AfterConnect`. Test handler lain konek superuser (RLS di-bypass). Seed pakai
  owner-pool.
- **Casbin CSV TAK dukung komentar inline** di akhir baris `g,`/`p,` (jadi bagian
  nilai → link mati). Komentar di baris `#` tersendiri.

## Client-side JS (CSP-safe)

Datastar bukan untuk manipulasi tabel (filter/paginate/copy), zoom, chart, atau
state UI persisten (collapse sidebar, tema). Untuk itu file terpisah di `static/`
(`sidebar.js`, `theme.js`, `health.js`, `erd.js`, `charts.js`) — same-origin lolos
`script-src 'self'`, BUKAN inline. Muat via `<script src>`, SINKRON bila perlu set
state sebelum paint (no-FOUC): `sidebar.js`/`theme.js` set atribut `<html>`
(`data-sidebar`/`data-theme`) sebelum body render. Data untuk JS ditanam via
`<script type="application/json">` (CSP-safe), JS `JSON.parse`.

## Aset vendored

`static/` = aset vendored (datastar.js, daisyui.js, mermaid.min.js, echarts.min.js);
checksum di `static/VENDOR.md`. `daisyui.js` = plugin Tailwind (di-`@plugin`,
dikonsumsi `make css`, bukan dimuat browser). Tailwind CLI di-download `make setup`
(gitignored, verifikasi integritas). Prinsip no-CDN: semua di-embed.

## Konfigurasi produksi: gagal keras, jangan andalkan ingatan

Prinsip: **yang berbahaya bila salah harus menggagalkan boot; yang bisa diturunkan
otomatis jangan diminta ke manusia.** Warning di log bukan pengaman.

- **Gagal dengan PETUNJUK** (`internal/preflight`, dipanggil awal `run()` & `make
  doctor` — sumber SAMA). Tiap `Problem` WAJIB punya `Fix` (nama yang dicari, DB
  mirip yang ada, perintah persis). Semua masalah dikumpulkan sekaligus.
- **DB dibuat otomatis HANYA di dev** (`AutoCreateDB: !cfg.IsProduction()`). Di
  produksi, DSN salah ketik jadi DB kosong yang tampak sehat. `make doctor` tak
  pernah membuatnya (alat diagnosis tak boleh ubah keadaan).
- **Isolasi tenant DIBUKTIKAN** (`db.CheckRLSTx`, `verifyTenantIsolation` di
  `main.go`) DI DALAM `WithSuper` (tx yang sudah turun hak). Periksa
  `rolsuper`/`rolbypassrls`/pemilik-tabel + `FORCE RLS`. Sama di dev & produksi.
  Sejarah: dulu sekadar "env `APP_DATABASE_URL` terisi" → DSN sama seperti
  `DATABASE_URL` lolos sambil bocor (82 baris dari 15 tenant).
- **Produksi tak butuh persiapan role manual.** Migrasi buat `app_rw` + GRANT +
  `ALTER DEFAULT PRIVILEGES` + `GRANT app_rw TO CURRENT_USER`. Role `NOLOGIN` (tak
  ada password/entri PgBouncer). `GENERATED ALWAYS AS IDENTITY` tak butuh GRANT
  sequence terpisah.
- **`SESSION_KEY` divalidasi PANJANG** (min 32, `config.MinSessionKeyLen`), bukan
  cuma keberadaan (kunci lemah lebih bahaya dari kosong). Kini DIPAKAI: turunkan
  nama cookie sesi (`Config.SessionCookieName`, pakai HASH-nya) agar dua deployment
  di host sama tak saling timpa sesi.
- **`Cookie.Secure` diturunkan dari `ENV=production`**, bukan env sendiri.
  Konsekuensi: produksi tanpa HTTPS = login mati (gagal keras, bukan bocor senyap).
- **Jangan tambah env baru untuk hal yang bisa diturunkan** dari env yang ada.

## MCP server read-only (`internal/mcpserver`) — akses runtime untuk agent AI

Memberi agent AI "mata" ke runtime dev/staging/prod (bug di staging tak lagi
buta) — SEMUA read-only. SDK resmi `modelcontextprotocol/go-sdk` v1.7.0.

- **ADAPTER TIPIS, bukan pintu baru.** Tiap tool memanggil ULANG fungsi baca yang
  sudah aman (`erd.Introspect`, `db.CheckRLS`, `preflight.Run`, query `List*`/
  `Count*`/`Presence*`), tak menulis logika/query baru. Menambah tool = memetakan
  satu fungsi read-only ke satu handler. DILARANG: tool SQL/shell mentah, embed
  `maintenance`/`MigrateWithLock`/`Create*`, mengalirkan nilai rahasia (lapor
  keberadaan/panjang, bukan nilai — Rule 7/12).
- **`platform_stats` menyaring settings lewat ALLOWLIST** (`exposedSettings` di
  `tools_health.go`), bukan mengekspos seluruh `platform_settings`. Sebabnya
  masa-depan: `platform_settings` bisa menampung key baru kapan saja (kredensial
  SMTP, secret webhook), dan denylist/tanpa-filter akan membocorkannya diam-diam
  ke agent — penambahnya sedang mengurus fitur lain, tak memikirkan MCP. Dengan
  allowlist, key baru TAK muncul sampai sengaja didaftarkan, dan di titik itu
  penambahnya menimbang "aman dibaca agent?". Menambah key ke allowlist =
  keputusan sadar; JANGAN kembalikan ke "ekspos semua". Dikunci
  `TestPlatformStats_AllowlistMenyaringKeySensitif`.
- **Read-only STRUKTURAL, bukan disiplin.** Semua akses DB lewat
  `db.WithSuper(ctx, pool, fn)` → `SET LOCAL ROLE app_rw` → **DDL ditolak DB**.
  `h.q(ctx)` TAK BISA dipakai (panic tanpa Scope middleware) — MCP tanpa request
  context. Test `TestReadOnly_NolTulis` menghitung baris sebelum/sesudah tiap
  tool = harus SAMA; kalau kelak ada tool menulis tak sengaja, itu yang menangkap.
- **BUKAN service/proses terpisah.** `mcpserver.Handler()` = `http.Handler` biasa,
  dipasang sebagai rute `/mcp` di app yang sudah jalan (Streamable HTTP, Stateless+
  JSONResponse). Image/container/deploy/reverse-proxy SAMA. Ini keputusan yang
  membedakannya dari refleks "MCP = binary terpisah" (benar untuk stdio subprocess,
  SALAH untuk HTTP remote).
- **Rute opt-in, dijaga Bearer.** `routes.go` mendaftarkan `/mcp` HANYA bila
  `cfg.MCPToken != ""` (fitur yang membuka runtime ke AI tak menyala karena lupa).
  Dijaga `mw.RequireBearer` (constant-time compare) — BUKAN RequireAuth (kliennya
  agent/program, bukan manusia ber-session). Sejajar `/healthz`, di luar auth sesi.
- **Satu server, dua transport.** `build()` merakit `*mcp.Server` sekali; dipakai
  HTTP (`Handler`) & stdio (`ServeStdio`, subcommand `./app mcp` untuk dev). Jadi
  kemampuan keduanya mustahil berbeda. Di stdio, logger WAJIB ke STDERR — stdout
  milik protokol JSON-RPC (satu baris log ke sana merusak framing).
- **Fase berikutnya (belum ada):** tool tulis terjaga (migrasi/purge) dev+staging
  saja, gated `!IsProduction()`, tiap aksi → `audit_logs` (aktor "agent"). Produksi
  tetap read-only.

## Batasan

- Semua interaktivitas = **Datastar** (satu paradigma). Jangan tambah framework JS.
- Single-binary — jangan tambah dependency yang butuh runtime kedua / Node.
- Password auth = dev-only; jangan aktifkan di produksi.
- **`unsafe-eval` = keputusan sadar, BUKAN bug**: Datastar `new Function()` ekspresi
  `data-*`; nonce tak bisa menggantikan. Risiko rendah (ekspresi cuma ID integer +
  literal internal, tak pernah input user). Jangan "perbaiki" dengan `unsafe-inline`
  atau tukar runtime tanpa diskusi. Footgun authoring ditutup helper `dsx.go` (#5–6).
