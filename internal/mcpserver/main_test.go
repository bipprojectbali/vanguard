package mcpserver

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go_starter/internal/config"
	"go_starter/internal/testdb"

	"github.com/jackc/pgx/v5/pgxpool"
)

// main_test.go — schema Postgres milik paket ini sendiri (pola internal/testdb).
// MCP server read-only diuji terhadap DB nyata di schema terisolasi, jadi test
// paralel antar-paket tak saling menimpa.

var (
	pkgPool *pgxpool.Pool
	pkgCfg  *config.Config
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, err := testdb.Pool(ctx, "mcpserver")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
	pkgPool = pool
	// Config minimal yang cukup untuk tool: DSN (dipakai preflight) + TZ (window).
	pkgCfg = &config.Config{AppTimezone: "Asia/Jakarta"}
	if pool != nil {
		pkgCfg.DatabaseURL = os.Getenv("TEST_DATABASE_URL")
	}

	code := m.Run()

	if pool != nil {
		pool.Close()
		testdb.Drop(ctx, "mcpserver")
	}
	os.Exit(code)
}

// testDeps merakit deps untuk memanggil handler tool langsung (tanpa transport).
func testDeps(t *testing.T) *deps {
	t.Helper()
	if pkgPool == nil {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test yang butuh DB")
	}
	return &deps{pool: pkgPool, cfg: pkgCfg, log: discardLogger()}
}
