# CLAUDE.md — panduan agen untuk go_starter

Baca ini + [`STARTER.md`](STARTER.md) (spec & arsitektur) sebelum kode.
README = cara pakai; file ini = konvensi + **gotcha mahal**.

## Alur kerja wajib

- **`make check` = gerbang** (sqlc · vet · gofmt · build · test) — hijau sebelum lapor selesai.
- **Tiap paket test punya SCHEMA Postgres sendiri** (`internal/testdb`) → `go test ./...`
  paralel. Paket ber-DB wajib `TestMain` panggil `testdb.Pool(ctx,"<nama>")` +
  `testdb.Drop`; ambil pool dari var paket (`pkgPool`), JANGAN buka `pgxpool.New`
  sendiri (DSN mentah → `public`, tabel tak ada). `search_path` sengaja TANPA `public`:
  dgn `public`, goose temukan `goose_db_version` DB utama → kira "versi terakhir" →
  schema baru KOSONG. Migrasi `00004` GRANT `app_rw` ke `current_schema()` (bukan
  `public` harfiah) agar RLS mengikat di schema mana pun.
- Tooling (`sqlc`/`goose`/`air`) di `$(go env GOPATH)/bin`; Makefile via path absolut
  (`$(GOBIN)/sqlc`) — GNU Make 3.81 macOS exec via `execvp`, `export PATH` tak terbaca.
  Jangan ubah ke pemanggilan telanjang.
- Ubah schema → migrasi goose → jalankan lokal → `sqlc generate` → perbaiki ripple.
  Jangan edit `internal/db/*` (generated).
- **Skema di SATU migrasi** (`00001_schema.sql`, 11 migrasi disatukan saat belum ada
  deployment). Perubahan berikutnya inkremental (`00002_...`); jangan sunting `00001`
  setelah repo di-clone / disatukan ulang setelah produksi.
- Fitur baru wajib test dalam pekerjaan sama. Test pakai `TEST_DATABASE_URL` (Postgres,
  jangan tukar engine), `skip` bila kosong.

## Arsitektur & konvensi

- **`routes.go` = single source of truth** semua route. Chain terproteksi: `RequireAuth`
  → `Scope` → `RefreshIdentity` → `TrackPresence` → `RequireEnforce(obj,act)`. `Scope`
  buka tx ber-tenant SEBELUM `RefreshIdentity`/`TrackPresence` (pakai `h.q(ctx)`).
- **TEMPLATE yang di-clone.** Nama project/app tak boleh hardcode: module path via
  `make rename name=X`, nama tampil dari `APP_NAME` via `handler.SetAppName` (`devBrand()`
  sidebar `/dev`, `LayoutData.Brand` header). Test brand pakai `devBrand()` BUKAN string
  harfiah. Target `dev`/`css` bergantung `tailwind` (binary 76MB, gitignored) agar clone
  baru langsung jalan.
- **Query saring `table_schema` WAJIB `current_schema()`**, bukan `'public'` harfiah
  (pernah bikin ERD kosong di schema non-public).
- **Config dibaca HANYA di `internal/config`** — jangan `os.Getenv` tersebar.
- **Handler tak simpan config**; inject via setter global saat startup (`SetCSSPath`,
  `SetDevMode`, `SetGoogleOAuth`, `SetSuperAdminChecker`, `SetAppTimezone`, `session.Init`,
  `authz.Init`).
- **Atribut Datastar via helper bertipe** (`internal/ui/dsx.go`): `ClassOn`,
  `FormPostSelect`, `PostAction`/`DeleteAction`. Menutup gotcha #5 & #6 struktural —
  jangan tulis `data.Class`/`@post` mentah di view.
- **View murni-data**: gomponents terima data siap-render; jangan panggil `authz.Can`/
  session di dalamnya. Precompute flag di handler (`ui.When`, `quickLinksFor`).
- **Dua jalur render**: `renderPage` (Layout landing/app) vs `renderShell` (AppShell +
  sidebar). `headNodes()` dibagi keduanya.

## Desain mobile-first (WAJIB — template di-clone, kelalaian menular ke turunan)

- **Mobile-first**: kelas dasar (tanpa prefix) = MOBILE; naikkan `sm:`/`md:`/`lg:`.
  ✅ `grid-cols-1 md:grid-cols-2` ❌ `grid-cols-2 max-md:grid-cols-1`.
- **Breakpoint** Tailwind: `sm` 640 · `md` 768 · `lg` 1024. Sidebar = drawer `<md`, tetap
  `md:`+. **AppShell (`internal/ui/appshell.go`) = pola acuan** (`-translate-x-full` +
  `md:translate-x-0`). Jangan bikin mekanisme baru.
- **Nol overflow horizontal** 320–375px. Grid/flex → 1 kolom di mobile; konten lebar
  `truncate`/`break-words`.
- **SETIAP `<table>` bungkus `ui.TableScroll`** (test regresi `tablescroll_test.go`). WAJIB
  bersama: (a) `overflow-x-auto` di pembungkus LANGSUNG tabel (di `.card-body` tak menahan),
  (b) `min-w-0` (card/flex-item default `min-width:auto` menolak menyusut). Tanpa ini tabel
  telanjang meluber di viewport 375px.
- **Baris tombol horizontal wajib `flex-wrap`** (pagination dorong halaman di 375px;
  `flex-wrap` di wrapper luar tak menurun ke baris dalam).
- **Diagnosis overflow**: ukur `documentElement.scrollWidth` vs `clientWidth`, daftar elemen
  `getBoundingClientRect().right > vw` TANPA `closest('.overflow-x-auto')` — itu pelakunya.
- **Tap target ≥ 44px**; input `text-base` (≥16px) agar iOS tak auto-zoom.
- **Verifikasi 3 lebar (375/768/1280) via ego-browser HANYA saat user memerintahkan
  eksplisit** ("buka browser dan cek...", "verifikasi pakai ego-browser..." — pola sama
  larangan Playwright global §1 CLAUDE.md user). Tanpa perintah eksplisit, review
  statis kelas Tailwind (mobile-first, `flex-wrap`, `TableScroll`, dsb. — poin di atas)
  cukup untuk lapor selesai; sebutkan bahwa verifikasi visual belum dijalankan. Jika
  dijalankan: skill **ego-browser**, 375/768/1280 + screenshot tiap lebar; CDP
  `Emulation.setDeviceMetricsOverride` lalu `clearDeviceMetricsOverride` (tak ada
  `set_viewport`).

## Identitas panel (/w/{slug} · /dev)

Semua halaman pakai AppShell SAMA. **`/dev` = data LINTAS-workspace** (`ListUsers` lihat
semua tenant) → salah kira panel = salah baca cakupan data.

- **Sumber `internal/ui/panelkind.go`** (`Panel` bertipe + `panelStyles` array berindeks
  enum → tambah panel tanpa gaya = compile error). `panelOf(ctx, currentPath)` menurunkannya.
- **Hanya `/dev` ditentukan PATH.** `/w/{slug}` semua role → chip dari ROLE (`panelForRole`,
  sumber sama dengan `navFor`). Owner/admin → `ADMIN`, member → `RUANG KERJA`.
- **DUA penanda sengaja**: chip TEKS + aksen warna tepi atas sidebar. Sidebar collapse rail
  4rem → `.app-brand` disembunyikan (chip hilang), aksen tepi bertahan. Jangan hapus salah satu.
- **Warna WAJIB token semantik daisyUI** (`primary`/`secondary`/`warning`), bukan absolut.
  `/dev` pakai `warning` sebagai peringatan.
- Chip MENGGANTIKAN sub-label panel, tak menumpuk. Brand = NAMA WORKSPACE atau `go_starter /dev`.

## Gotcha (mahal — jangan temukan ulang)

Ringkasan + trigger di bawah; **detail lengkap + rasional tiap poin di
[`docs/agent-notes.md#gotcha`](docs/agent-notes.md).**

1. **CSP wajib `unsafe-eval`** — Datastar eval `data-*` via `new Function()`; tanpanya mati senyap.
2. **scs + SSE cookie** — `session.WriteCookie` SEBELUM `NewSSE` (NewSSE flush header, bypass scs).
3. **SameSite=Lax bukan Strict** — callback OAuth cross-site; Strict → "state tidak valid".
4. **daisyUI = plugin Tailwind** — bukan `bg-sidebar`/`text-muted-foreground`; padanan sidebar=`bg-base-200`, muted=`text-base-content/70`, destructive=`error`. Class tree-shaken (pakai di `.go` dulu).
5. **`data.Class` key ber-hyphen wajib quote** — pakai `ui.ClassOn`, bukan `data.Class` mentah.
6. **`@post {contentType:'form'}` butuh `<form>` terdekat** — pakai `ui.FormPostSelect`.
7. **Toast wajib `pointer-events:none`** — `opacity:0` pun tetap tangkap klik.
8. **Super-admin env-override** — `SUPER_ADMIN_EMAILS` menang atas role DB; `RefreshIdentity` load segar per-request, jangan cache di session.
9. **Panel dev-only gated `devMode` di DUA tempat** — route (`if devMode`) + menu (`devNav()`).
10. **Tema selaras 2 tempat** — `themeList` (`ui/theme.go`) & `themes:` (`static/input.css`), lalu `make css`.
11. **Hierarki warna** — halaman `bg-base-200`, card/sidebar `bg-base-100`; token relatif, jangan absolut. Active `bg-primary text-primary-content`.
12. **Chart = ECharts vendored + `charts.js` eksternal** — bukan go-echarts (script inline diblokir CSP); option via `<script type="application/json">`.
13. **Presence ≠ audit** — dua tabel terpisah; `target_type` menentukan JOIN (salah = nama keliru); pakai `h.audit*` bukan `auditLog` telanjang.
14. **Timezone: simpan UTC, agregasi `AT TIME ZONE`** — `import _ "time/tzdata"` WAJIB; hindari `AT TIME ZONE` di SELECT list sqlc (emit `interface{}`).
15. **`content-type: text/javascript` = eksekusi JS** — escape SEMUA input user (`g.Text`), validasi signal user-modifiable di backend, jangan taruh data sensitif di signal.
16. **`sse.Redirect` DIBLOKIR CSP** — NAVIGASI pakai native POST → `http.Redirect(...,303)`; gagal validasi PRG `?err=CODE`; SSE hanya untuk fragment parsial ter-escape.

## Tenancy (single↔multi · RLS · membership · siklus hidup) — keputusan 0002–0008

> **Detail lengkap tiap butir + rasional di [`docs/agent-notes.md`](docs/agent-notes.md)**
> (section Mode tenancy, Multi-tenancy, Siklus hidup). ADR formal:
> [`docs/decisions/0002`](docs/decisions/0002-multi-tenancy-rls-role-2-bidang.md)–
> [`0008`](docs/decisions/0008-daftar-anggota-hanya-pengelola.md). Baca sebelum menyentuh
> `Scope`/RLS/membership/lifecycle — di bawah hanya aturan yang paling sering dilanggar.

- **Isolasi = Postgres RLS + FORCE**, bukan cuma `WHERE tenant_id`. `h.q(ctx)` (tx ber-tenant dari
  `Scope`), JANGAN `h.DB`. Lupa Scope = panic keras. Jalur pre-identity → `db.WithSuper` eksplisit.
- **`WithTenant`/`WithSuper` = SATU tx**, GUC `set_config(...,true)` TX-LOCAL + `SET LOCAL ROLE
  app_rw`. `,true` WAJIB (plain `SET` bocor ke peminjam pool). Bypass dari ROLE (`isPlatformRole`),
  tak pernah dari data. Migrasi WAJIB `GRANT app_rw TO CURRENT_USER`.
- **Single = multi-tenant N=1**; jangan buang `tenant_id`. Mode di DB (`platform_settings.tenancy_mode`,
  nol baris = single), bukan env. Ratchet: naik boleh, turun ditolak trigger. Naikkan di `/dev/settings`
  (gate `platform:settings`, konfirmasi ketik nama app, wajib ter-audit).
- **Workspace di PATH `/w/{slug}`** — HANYA lewat `wsPath`/`wsRedirect` (`internal/handler/wspath.go`).
  `/admin` & `/user` TIDAK ADA (beda role = beda AKSI, bukan beda alamat). Slug asing → `http.NotFound`
  (bukan 403/redirect). Platform TETAP `adoptTenantBySlug` walau bypass RLS.
- **Workspace PRIMER (`is_primary`) = rumah aplikasi** — tak bisa diarsip/hapus, tak makan kuota;
  dijaga di handler DAN SQL (`AND NOT is_primary`). super_admin = owner primer, dipasang saat LOGIN
  (`ensurePrimaryOwner`, promote-only).
- **super_admin = ENV-ONLY (nol baris DB)**; overlay `RefreshIdentity` per-request (env →
  `platform_staff` → `memberships.role`). Role di `memberships` (user × tenant × role), BUKAN `users`
  (global, keluar RLS). `GuardSetRole` pakai `>=` (cegah admin angkat sesama admin).
- **Siklus hidup** `tenants.status active|suspended|archived` + `deleted_at` (tenggang 30 hari),
  ditegakkan `gateLifecycle`. suspended = tindakan PLATFORM · archived = keputusan OWNER. Kode:
  bukan-anggota/deleted → 404 · suspended → 403+alasan · archived → GET lolos, non-GET 403. Platform
  SENGAJA tembus gerbang. Unarchive DI LUAR `/w/{slug}` (pakai `tenantBySlug`). Purge terjadwal
  `internal/maintenance` (advisory-lock 4243, satu-per-satu).
- **Daftar panjang wajib keyset `?after=`** (`pageSize+1` → `splitPage`, `created_at DESC`), bukan
  OFFSET; cursor rusak → halaman pertama. Navigasi link `<a>` biasa (lolos gotcha #16).
- **Daftar anggota HANYA pengelola** (owner/admin/platform, 0008); gerbang di HANDLER
  (`canManageMembers`), ditolak 403+penjelasan bukan 404. **PII di-mask di HANDLER bukan view**
  (`maskEmail`: domain utuh, panjang lokal tak bocor, email sendiri utuh). **View tak rakit path
  sendiri** — oper `base` dari handler; verifikasi UI wajib mencakup SUBMIT.
- **DUA pintu ubah role SAMA** (`/w/{slug}/members` & `/dev/users`): `UpdateMemberRole` +
  `GuardSetRole` + `h.notify("member.role.changed")`. Status/soft-delete SENGAJA tanpa notifikasi.
- **Kuota = DUA lapis** (`internal/settings`): global (`platform_settings`) + override
  `users.workspace_quota` (`NULL` = ikut global). Hitung HANYA via `settings.EffectiveWorkspaceQuota`;
  hanya owner belum-terhapus dihitung (arsip TETAP dihitung, terhapus TIDAK). `CountTenantOwners`
  cegah owner terakhir turun.
- **TANPA RLS (sengaja)**: `notifications` (milik user lintas-workspace, pakai `db.WithSuper` +
  `WHERE user_id/email`), `memberships`/`invites` (dibaca untuk MENENTUKAN scope). Keamanan dari
  filter query + test isolasi antar-user.
- **Audit di tx `WithSuper` TERPISAH** (fail-soft); `audit_logs.tenant_id` NULLABLE `ON DELETE SET
  NULL`. Cache settings PER-PROSES. Casbin CSV TAK dukung komentar inline (baris `#` tersendiri).

## Client-side JS (CSP-safe)

Datastar bukan untuk manipulasi tabel (filter/paginate/copy), zoom, chart, atau state UI persisten
(collapse sidebar, tema). Untuk itu file terpisah di `static/` (`sidebar.js`, `theme.js`, `health.js`,
`erd.js`, `charts.js`) — same-origin lolos `script-src 'self'`, BUKAN inline. Muat via `<script src>`,
SINKRON bila perlu set state sebelum paint (no-FOUC): `sidebar.js`/`theme.js` set atribut `<html>`
(`data-sidebar`/`data-theme`) sebelum body render. Data untuk JS ditanam via `<script
type="application/json">` (CSP-safe), JS `JSON.parse`.

## Aset vendored

`static/` = aset vendored (datastar.js, daisyui.js, mermaid.min.js, echarts.min.js); checksum di
`static/VENDOR.md`. `daisyui.js` = plugin Tailwind (di-`@plugin`, dikonsumsi `make css`, bukan dimuat
browser). Tailwind CLI di-download `make setup` (gitignored, verifikasi integritas). Prinsip no-CDN:
semua di-embed.

## Konfigurasi produksi: gagal keras, jangan andalkan ingatan

Prinsip: **yang berbahaya bila salah harus menggagalkan boot; yang bisa diturunkan otomatis jangan
diminta ke manusia.** Warning di log bukan pengaman. **Detail lengkap:
[`docs/agent-notes.md#konfigurasi-produksi-gagal-keras-jangan-andalkan-ingatan`](docs/agent-notes.md).**

- **Gagal dengan PETUNJUK** (`internal/preflight`, dipanggil `run()` & `make doctor` — sumber SAMA);
  tiap `Problem` WAJIB punya `Fix`. **DB auto-create HANYA dev** (`AutoCreateDB: !IsProduction()`).
- **Isolasi tenant DIBUKTIKAN** (`db.CheckRLSTx`/`verifyTenantIsolation` dalam `WithSuper`): cek
  `rolsuper`/`rolbypassrls`/pemilik-tabel + `FORCE RLS`. Migrasi siapkan role (`app_rw` NOLOGIN + GRANT).
- **`SESSION_KEY` divalidasi PANJANG** (min 32) & dipakai turunkan `SessionCookieName` (hash).
  **`Cookie.Secure` dari `ENV=production`** (tanpa HTTPS → login mati, gagal keras). Jangan tambah env
  baru untuk yang bisa diturunkan dari env yang ada.

## MCP server read-only (`internal/mcpserver`)

Mata read-only agent AI ke runtime dev/staging/prod. **Detail lengkap + rasional di
[`internal/mcpserver/README.md`](internal/mcpserver/README.md).**

- **ADAPTER TIPIS** — tiap tool panggil ULANG fungsi baca aman (`erd.Introspect`, `db.CheckRLS`,
  `preflight.Run`, `List*`/`Count*`/`Presence*`). DILARANG SQL/shell mentah, embed `maintenance`/
  `Create*`, alirkan nilai rahasia. `platform_stats` saring via ALLOWLIST (`exposedSettings`).
- **Read-only STRUKTURAL** — akses DB via `db.WithSuper` (`SET LOCAL ROLE app_rw` → DDL ditolak);
  `h.q(ctx)` tak bisa (panic tanpa Scope). Test `TestReadOnly_NolTulis` kunci ini.
- **Bukan proses terpisah** — `mcpserver.Handler()` dipasang rute `/mcp` (opt-in bila
  `cfg.MCPToken != ""`, dijaga `mw.RequireBearer`). Satu server dua transport (HTTP + stdio).

## Batasan

- Semua interaktivitas = **Datastar** (satu paradigma). Jangan tambah framework JS.
- Single-binary — jangan tambah dependency yang butuh runtime kedua / Node.
- Password auth = dev-only; jangan aktifkan di produksi.
- **`unsafe-eval` = keputusan sadar, BUKAN bug**: Datastar `new Function()` ekspresi `data-*`; nonce
  tak bisa menggantikan. Risiko rendah (ekspresi cuma ID integer + literal internal, tak pernah input
  user). Jangan "perbaiki" dengan `unsafe-inline` atau tukar runtime tanpa diskusi. Footgun authoring
  ditutup helper `dsx.go` (#5–6).
