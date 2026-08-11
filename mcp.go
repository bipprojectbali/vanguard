package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go_starter/internal/config"
	"go_starter/internal/mcpserver"

	"github.com/jackc/pgx/v5/pgxpool"
)

// mcp.go — `./app mcp`: MCP server READ-ONLY lewat stdio, untuk DEV LOKAL.
//
// Di produksi MCP dipakai lewat HTTP (rute /mcp, dijaga Bearer token) — bukan
// subcommand ini. Stdio di sini adalah kenyamanan lokal: agent menjalankan
// `go run . mcp` / `./app mcp` langsung, tanpa server hidup, tanpa token, tanpa
// port. Server MCP yang dirakit SAMA dengan yang dipasang di /mcp (mcpserver.build),
// jadi kemampuan keduanya mustahil berbeda.

// runMCPStdio memuat env minimal, membuka pool, dan menjalankan MCP server lewat
// stdin/stdout sampai klien memutus atau SIGINT/SIGTERM.
//
// Boot ringan cermin runMigrate: hanya butuh DATABASE_URL (LoadMigrateConfig,
// bukan MustLoad yang mewajibkan SESSION_KEY/Google) — MCP baca tak menyentuh
// session/OAuth/HTTP.
func runMCPStdio() error {
	_ = config.LoadDotEnv(".env")
	cfg, err := config.LoadMigrateConfig()
	if err != nil {
		return err
	}
	// LoadMigrateConfig hanya mengisi DATABASE_URL. Tool MCP juga membaca
	// RedisAddr (preflight) & AppTimezone (activity_kpis) — diisi dari env
	// langsung agar tool akurat di jalur stdio; keduanya fail-soft bila kosong
	// (preflight lapor Redis absen, window jatuh ke UTC), jadi tetap jalan.
	cfg.RedisAddr = os.Getenv("REDIS_ADDR")
	if tz := os.Getenv("APP_TIMEZONE"); tz != "" {
		cfg.AppTimezone = tz
	} else {
		cfg.AppTimezone = "Asia/Jakarta"
	}

	// KRITIS: logger ke STDERR. stdout adalah kanal protokol MCP (framing
	// JSON-RPC); satu baris log ke sana merusak pesan dan klien gagal parse.
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Info("mcp: stdio server siap (read-only)")
	return mcpserver.ServeStdio(ctx, pool, cfg, log)
}
