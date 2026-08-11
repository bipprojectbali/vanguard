# MCP server read-only (`internal/mcpserver`) — akses runtime untuk agent AI

Memberi agent AI "mata" ke runtime dev/staging/prod (bug di staging tak lagi buta) — SEMUA read-only.
SDK resmi `modelcontextprotocol/go-sdk` v1.7.0.

- **ADAPTER TIPIS, bukan pintu baru.** Tiap tool panggil ULANG fungsi baca yang sudah aman
  (`erd.Introspect`, `db.CheckRLS`, `preflight.Run`, query `List*`/`Count*`/`Presence*`), tak menulis
  logika/query baru. Tambah tool = petakan satu fungsi read-only ke satu handler. DILARANG: tool
  SQL/shell mentah, embed `maintenance`/`MigrateWithLock`/`Create*`, mengalirkan nilai rahasia (lapor
  keberadaan/panjang, bukan nilai — Rule 7/12).
- **`platform_stats` menyaring settings lewat ALLOWLIST** (`exposedSettings` di `tools_health.go`),
  bukan mengekspos seluruh `platform_settings`. Sebab masa-depan: `platform_settings` bisa menampung
  key baru kapan saja (kredensial SMTP, secret webhook), dan denylist/tanpa-filter membocorkannya
  diam-diam (penambahnya mengurus fitur lain, tak memikirkan MCP). Allowlist → key baru TAK muncul
  sampai sengaja didaftarkan (di titik itu penambahnya menimbang "aman dibaca agent?"). Menambah key =
  keputusan sadar; JANGAN kembalikan ke "ekspos semua". Dikunci
  `TestPlatformStats_AllowlistMenyaringKeySensitif`.
- **Read-only STRUKTURAL, bukan disiplin.** Semua akses DB lewat `db.WithSuper(ctx, pool, fn)` → `SET
  LOCAL ROLE app_rw` → **DDL ditolak DB**. `h.q(ctx)` TAK BISA dipakai (panic tanpa Scope middleware)
  — MCP tanpa request context. Test `TestReadOnly_NolTulis` hitung baris sebelum/sesudah tiap tool =
  harus SAMA.
- **BUKAN service/proses terpisah.** `mcpserver.Handler()` = `http.Handler` biasa, dipasang rute
  `/mcp` di app yang sudah jalan (Streamable HTTP, Stateless+JSONResponse). Image/container/deploy/
  reverse-proxy SAMA. Bukan refleks "MCP = binary terpisah" (benar untuk stdio subprocess, SALAH untuk
  HTTP remote).
- **Rute opt-in, dijaga Bearer.** `routes.go` daftarkan `/mcp` HANYA bila `cfg.MCPToken != ""`. Dijaga
  `mw.RequireBearer` (constant-time compare) — BUKAN RequireAuth (klien agent/program, bukan manusia
  ber-session). Sejajar `/healthz`, di luar auth sesi.
- **Satu server, dua transport.** `build()` merakit `*mcp.Server` sekali; dipakai HTTP (`Handler`) &
  stdio (`ServeStdio`, subcommand `./app mcp` untuk dev). Di stdio, logger WAJIB ke STDERR — stdout
  milik protokol JSON-RPC (satu baris log merusak framing).
- **Fase berikutnya (belum ada):** tool tulis terjaga (migrasi/purge) dev+staging saja, gated
  `!IsProduction()`, tiap aksi → `audit_logs` (aktor "agent"). Produksi tetap read-only.
