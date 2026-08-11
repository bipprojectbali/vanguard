# go_starter

Starter web full-stack Go: **satu bahasa, satu binary, satu perintah build**.
Cepat, ringan, modern (2026), agent-friendly. Output = single binary (static
assets, migrations, dan template di-embed via `embed.FS`).

> Runtime tetap butuh **PostgreSQL** + **Redis** — "nol artefak tambahan", bukan
> "nol infra". Lihat [`STARTER.md`](STARTER.md) untuk spesifikasi & alasan arsitektur.

## Stack

| Lapis | Teknologi |
|-------|-----------|
| Router | `net/http` + [chi](https://github.com/go-chi/chi) |
| View | [gomponents](https://maragu.dev/gomponents) (HTML sebagai fungsi Go) |
| Interaktivitas | [Datastar](https://data-star.dev) (hypermedia, SSE-native) |
| CSS | Tailwind v4 (standalone CLI, no-Node) + [daisyUI](https://daisyui.com) (plugin, zero-JS komponen, multi-tema light/dark) |
| Chart | [ECharts](https://echarts.apache.org) vendored (panel aktivitas, init CSP-safe eksternal) |
| DB | PostgreSQL via [pgx](https://github.com/jackc/pgx) + [sqlc](https://sqlc.dev) (type-safe) |
| Migrasi | [goose](https://github.com/pressly/goose) (advisory-lock, aman multi-instance) |
| Session | [scs](https://github.com/alexedwards/scs) + [rueidis](https://github.com/redis/rueidis) (Redis store) |
| Auth | Password (argon2id, dev-only) + Google OAuth/OIDC |
| Otorisasi | [Casbin](https://casbin.org) RBAC — 2 bidang: platform (super_admin/staff) · tenant (owner/admin/member) |
| Multi-tenancy | Postgres RLS + membership (1 user ↔ banyak workspace) — `docs/decisions/0002`, `0003` |

## Prasyarat

- **Go 1.26+**
- **PostgreSQL** & **Redis** berjalan
- macOS/Linux (Makefile mendeteksi OS/arch untuk unduh Tailwind CLI)

## Mulai cepat

```bash
# 1. Ganti nama project (module path + import + nama DB contoh)
make rename name=nama-project-anda

# 2. Install tooling (sqlc, goose, air) + unduh aset vendored (sekali saja)
make setup

# 3. Salin konfigurasi; sesuaikan DATABASE_URL, REDIS_ADDR, dan APP_NAME
#    (APP_NAME menentukan nama di halaman depan & sidebar)
cp .env.example .env

# 4. Buat database dev & test
createdb nama-project-anda && createdb nama-project-anda_test

# 5. Periksa lingkungan — melaporkan SEMUA yang kurang sekaligus
make doctor

# 6. Jalankan (live-reload; migrasi auto saat boot)
make dev
```

Buka <http://localhost:8080>. Aplikasi memigrasi DB otomatis (`AUTO_MIGRATE=true`).

## Konfigurasi (`.env`)

| Variabel | Wajib | Keterangan |
|----------|-------|------------|
| `DATABASE_URL` | ✓ | DSN Postgres |
| `REDIS_ADDR` | ✓ | alamat Redis (session store) |
| `TEST_DATABASE_URL` | untuk test | DB terpisah untuk `make test` |
| `PORT` | — | default `8080` |
| `ENV` | — | `dev` (default) \| `production` |
| `AUTO_MIGRATE` | — | default `true` |
| `SESSION_KEY` | prod | kunci sesi (wajib di production) |
| `GOOGLE_CLIENT_ID` / `_SECRET` / `_REDIRECT_URL` | prod | OAuth Google (wajib di production) |
| `SUPER_ADMIN_EMAILS` | — | email super-admin "root", dipisah koma |
| `APP_TIMEZONE` | — | zona waktu (IANA) agregasi panel aktivitas, default `Asia/Jakarta` |
| `MAX_WORKSPACES_PER_USER` | — | kuota workspace per user (default `3`); diundang tak memakan kuota |
| `APP_NAME` | — | nama aplikasi; dipakai membuat workspace primer saat boot pertama |

> Isolasi tenant & bentuk aplikasi **tidak punya env**. Hak DB diturunkan
> per-transaksi lalu dibuktikan saat boot; mode tenancy hidup di
> `platform_settings` dengan ratchet yang menolak penurunan
> (`docs/decisions/0007`).

## Autentikasi & role

- **Google OAuth** = jalur login utama. **Password** hanya di dev (`ENV != production`)
  untuk mempermudah agen/dev masuk.
- **Model role 2-bidang** (multi-tenancy). Dua bidang tegak-lurus:
  - **PLATFORM** (lintas-tenant): `super_admin` (dari `SUPER_ADMIN_EMAILS`, immutable,
    nol baris DB) + `staff` (tabel `platform_staff`, mutable, akses terbatas).
  - **WORKSPACE** (isolasi RLS): `owner > admin > member` — role **per-workspace**.
- **Satu user, banyak workspace** (model membership, seperti Slack/Notion/GitHub):
  register (isi **Nama Workspace**) otomatis membuat workspace pertama + jadi owner;
  user bisa membuat workspace lain (dibatasi `MAX_WORKSPACES_PER_USER`) **dan**
  diundang membantu workspace orang lain dengan role berbeda. Pindah workspace lewat
  dropdown di sidebar. Lihat `docs/decisions/0003`.
- **Undangan**: owner/admin mengundang lewat email di `/admin/members` → tautan
  bertoken (berlaku 7 hari, sekali pakai). Penerima yang belum punya akun otomatis
  bergabung setelah register.
- **Notifikasi** (`/notifications`, ada di semua panel): undangan masuk muncul
  sendiri di sini — penerima yang **sudah punya akun tak perlu tautan sama sekali**,
  cukup klik Terima/Tolak. Ditambah kabar keanggotaan (role diubah, dikeluarkan).
  Badge di sidebar menghitung yang belum dibaca + undangan yang belum ditindak.
  *Email keluar belum otomatis — untuk penerima yang BELUM punya akun, tautan di
  `/admin/members` masih disalin manual.*
- Setelah login, redirect per-role: platform (super_admin/staff)→`/dev`,
  owner/admin→`/admin`, member→`/user`. Landing `/` dapat diakses semua.
- Data tiap workspace terisolasi **Postgres RLS** (bukan cuma filter app) — lihat
  `docs/decisions/0002`. Satu DSN: koneksi dibuka sebagai owner (migrasi), lalu
  tiap transaksi aplikasi menurunkan haknya ke `app_rw` sehingga RLS mengikat —
  termasuk di dev (`docs/decisions/0007`).
- **Workspace** = nama tampilan tenant (kode tetap `tenant`). Nama boleh duplikat,
  slug unik & immutable (`acme`, `acme-2`). Owner ganti nama di `/admin/workspace`.

## Panel

| Rute | Akses | Isi |
|------|-------|-----|
| `/dev/users` | platform (super_admin/staff) | kelola role/status/hapus user + audit trail |
| `/dev/logs` | platform (super_admin/staff) | aktivitas user (presence "aktif jam berapa"), KPI + chart (harian/mingguan/bulanan) + event login/logout |
| `/dev/health` | platform, **dev-only** | scan kesehatan file `.go` (baris/karakter vs ambang) |
| `/dev/erd` | platform, **dev-only** | diagram ERD dari katalog live Postgres (Mermaid) |
| `/admin` | admin+ | dashboard admin (stub) |
| `/admin/members` | owner/admin kelola, admin+ lihat | anggota workspace + undangan |
| `/admin/workspace` | owner ubah, admin+ lihat | pengaturan workspace (ganti nama) |
| `/notifications` | semua user login | undangan masuk + kabar keanggotaan (lintas-workspace) |
| `/workspace/new` | semua user login | buat workspace baru (dibatasi kuota) |
| `/invite/{token}` | publik | terima undangan bergabung |
| `/user` | user+ | beranda user (stub) |

Panel dev-only (`/dev/health`, `/dev/erd`) tak terdaftar di production karena
butuh source/tooling yang tak ada di build single-binary.

## Perintah (Makefile)

| Perintah | Aksi |
|----------|------|
| `make doctor` | periksa lingkungan (.env, Postgres, Redis, database, nama project) |
| `make rename name=X` | ganti nama project setelah clone (module path, import, DB contoh) |
| `make setup` | install tooling + unduh aset vendored |
| `make dev` | live-reload (air): regenerate CSS → sqlc → build → run |
| `make check` | **gerbang wajib**: sqlc · vet · gofmt · build · test |
| `make test` | jalankan test (butuh `TEST_DATABASE_URL`) |
| `make build` | single binary (`./app`) — regenerate CSS + sqlc dulu |
| `make run` | build lalu jalankan |
| `make css` | generate `static/app.css` dari class di file `.go` |

Subcommand binary (di luar Makefile): `./app migrate` (migrasi lalu exit),
`./app doctor` (periksa lingkungan), `./app mcp` (server MCP read-only via stdio,
untuk dev — lihat [MCP](#mcp-server-read-only-akses-runtime-untuk-agent-ai)).
| `make migrate-new name=x` | buat file migrasi goose baru |

## Struktur

```
main.go            # entry point: config, wiring, graceful shutdown
routes.go          # SEMUA route (single source of truth)
migrations/        # goose *.sql (di-embed)
queries/           # sumber sqlc *.sql
internal/
  config/          # env → struct typed (fail-fast)
  db/              # GENERATED sqlc — jangan edit tangan
  database/        # migrasi (advisory lock)
  session/         # scs + rueidis, accessor typed
  auth/            # argon2id
  oauth/           # Google OIDC flow (PKCE, nonce, verify)
  authz/           # Casbin RBAC (model.conf + policy.csv embed) + guard
  mw/              # middleware (request-id, recover, log, security, auth, authz)
  handler/         # HTTP handler, satu file per fitur
  ui/              # gomponents: layout, komponen, halaman, helper Datastar bertipe (dsx)
  activity/        # agregasi presence + option ECharts (panel /dev/logs)
  health/          # scanner file-health (dev)
  erd/             # introspeksi katalog → teks Mermaid (dev)
  assets/          # cache-busting aset ber-hash
static/            # aset vendored + generated (di-embed)
```

## Testing

```bash
createdb go_starter_test
make test    # atau: make check (test + lint + build)
```

Test yang butuh DB memakai `TEST_DATABASE_URL` dan di-`skip` bila kosong.

## Deploy

`make build` menghasilkan satu binary `./app` (assets & migrasi ter-embed).
Set `ENV=production` + variabel wajib (lihat tabel konfigurasi). Binary butuh
Postgres + Redis yang dapat dijangkau. Lihat [`CHANGELOG.md`](CHANGELOG.md).

### Deploy produksi (Docker + Portainer)

Single image (`Dockerfile` multi-stage → distroless, ~33MB) dipakai untuk **dua
peran** via subcommand: `app migrate` (migrasi lalu exit) & `app` tanpa argumen
(server). Ini memisahkan **lifecycle migrasi dari deploy app** — aman & fail-safe.

**Alur:**

1. **Build & push image** → GHCR via workflow `publish.yml` (`workflow_dispatch`,
   pilih `stack_env=prod` + `tag`). Menghasilkan `ghcr.io/<owner>/<repo>:prod-<tag>`
   + `:prod-latest`.
2. **Deploy stack** di Portainer pakai [`docker-compose.yml`](docker-compose.yml).
   Set env (`DATABASE_URL`, `REDIS_ADDR`, `SESSION_KEY`, `GOOGLE_*`) via Portainer
   stack env. Re-pull/update via workflow `re-pull.yml`.
3. **Runtime (otomatis, urutan dijaga compose):**
   - Service `migrate` (`app migrate`) jalan **sekali**: ambil `pg_advisory_lock`
     (aman multi-instance) → `goose up` → exit 0.
   - Service `app` **menunggu** `migrate` sukses (`service_completed_successfully`)
     baru start, dengan **`AUTO_MIGRATE=false`** (app tak migrate sendiri).
   - **Fail-safe:** migrate gagal (exit ≠ 0) → app **tak pernah start**.

**Aturan `AUTO_MIGRATE`:** `true` hanya di dev (migrate saat boot). **Produksi
WAJIB `false`** — migrasi dikerjakan container `migrate` terpisah.

**Rollback:**
- **App:** re-pull tag image lama via Portainer.
- **Schema:** migrasi hanya maju (goose up, tak auto-down). Rancang migrasi
  **expand-contract** agar app versi lama tetap jalan di schema baru. Bila perlu
  turun, jalankan `goose down` manual — **backup DB dulu**.

**Health:** `/healthz` (liveness) & `/readyz` (cek DB) untuk probe reverse-proxy /
uptime monitor. Image distroless tanpa shell → probe dari **luar** container.

## MCP server read-only (akses runtime untuk agent AI)

Server [MCP](https://modelcontextprotocol.io) **read-only** memberi agent AI
"mata" ke runtime dev/staging/produksi — health, jejak audit, KPI presence,
skema DB, versi migrasi, `doctor` — **tanpa akses tulis**. Berguna saat bug di
staging/prod: agent bisa melihat keadaan nyata, bukan menebak dari lokal.

**Bukan service/proses terpisah.** Ia rute `/mcp` di aplikasi `app` yang sudah
jalan (`http.Handler`, Streamable HTTP) — image sama, container sama, di belakang
reverse proxy yang sudah ada. Deploy tak berubah; cukup tambah **satu env**.

**Aktifkan (staging/prod):** set `MCP_TOKEN` (≥32 char) di service `app` via
Portainer. Kosong = rute **tak didaftarkan** (opt-in — fitur yang membuka runtime
ke AI tak menyala karena lupa). Rute dijaga **Bearer token**; beda per-lingkungan
(bocor staging tak boleh membuka prod). Endpoint hanya di jaringan internal +
reverse proxy — **jangan expose port MCP** ke host.

**Client** (`.mcp.json`, lihat [`.mcp.json.example`](.mcp.json.example)): token di
**header** (`Authorization: Bearer ${VAR}`, bukan di URL — query string bocor ke
access-log), nilai via env, nama env beda per-lingkungan.

**Dev lokal:** `./app mcp` — stdio, tanpa server/token.

**Read-only itu struktural, bukan janji:** tiap tool memanggil ulang jalur baca
yang ada lewat `db.WithSuper` → `SET LOCAL ROLE app_rw` → **DDL ditolak database**.
Tak ada tool SQL/shell mentah; rahasia tak pernah dialirkan (dilaporkan
keberadaan/panjang, bukan nilai).
