// Command go_starter — entry point: config, wiring, start server.
package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // embed database tzdata: LoadLocation gagal di container minimal (CGO_ENABLED=0) tanpa ini

	"go_starter/internal/config"
	"go_starter/internal/database"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed static
var staticEmbed embed.FS

//go:embed migrations/*.sql
var migrationsEmbed embed.FS

func main() {
	// Dispatch subcommand SEBELUM run(). `./app migrate` = jalankan migrasi lalu
	// exit — dipakai container migrate one-shot (compose service_completed_successfully)
	// SEBELUM app start. Pola stdlib (switch os.Args), tanpa framework CLI.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			if err := runMigrate(); err != nil {
				slog.Error("migrate: fatal", "err", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "doctor":
			os.Exit(runDoctor())
		case "mcp":
			if err := runMCPStdio(); err != nil {
				// Log ke STDERR — stdout milik protokol MCP.
				slog.Error("mcp: fatal", "err", err)
				os.Exit(1)
			}
			os.Exit(0)
		default:
			slog.Error("unknown subcommand", "arg", os.Args[1])
			os.Exit(2)
		}
	}
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// runMigrate menjalankan HANYA migrasi lalu keluar. Dipakai container
// `./app migrate` (compose: condition service_completed_successfully) SEBELUM app
// start. Sengaja TIDAK memanggil config.MustLoad(): migrate hanya butuh
// DATABASE_URL, sedang MustLoad mewajibkan SESSION_KEY/Google di production.
// Tak buka Redis/OAuth/HTTP — reuse migrationsEmbed + MigrateWithLock (advisory lock).
func runMigrate() error {
	_ = config.LoadDotEnv(".env")
	cfg, err := config.LoadMigrateConfig()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.MigrateWithLock(ctx, pool, migrationsEmbed); err != nil {
		return err
	}
	slog.Info("migrations applied")
	return nil
}
