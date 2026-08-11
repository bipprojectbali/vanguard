package mcpserver

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/preflight"
	"go_starter/internal/settings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tools_health.go — tool yang menjawab "apakah lingkungan ini sehat & terpasang
// benar". Semua read-only, semua memanggil ulang fungsi yang sudah ada.

// noInput = struct kosong untuk tool tanpa argumen. SDK tetap butuh tipe input;
// struct kosong menghasilkan schema "tak ada parameter".
type noInput struct{}

// registerHealthTools mendaftarkan runtime_health, preflight, migration_version,
// platform_stats.
func registerHealthTools(s *mcp.Server, d *deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "runtime_health",
		Description: "Cek kesehatan runtime: konektivitas DB (ping) dan apakah RLS benar-benar mengikat koneksi ini (role app_rw, FORCE aktif). Read-only.",
	}, d.runtimeHealth)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "preflight",
		Description: "Periksa lingkungan seperti `make doctor`: .env, DSN, Postgres, database, Redis. Melaporkan semua masalah + cara perbaikan. TIDAK membuat database apa pun. Read-only.",
	}, d.preflight)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "migration_version",
		Description: "Versi migrasi database saat ini (goose_db_version tertinggi). Berguna memastikan skema staging/prod sudah sesuai. Read-only.",
	}, d.migrationVersion)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "platform_stats",
		Description: "Ringkasan angka platform: jumlah workspace, user dengan kuota khusus, dan pengaturan platform aktif. Read-only.",
	}, d.platformStats)
}

// --- runtime_health ---

type healthOut struct {
	DBReachable  bool   `json:"db_reachable" jsonschema:"true bila database bisa di-ping"`
	RLSBinds     bool   `json:"rls_binds" jsonschema:"true bila Row-Level Security mengikat koneksi (isolasi tenant aktif)"`
	ConnRole     string `json:"conn_role" jsonschema:"role Postgres yang dipakai transaksi aplikasi"`
	RLSReason    string `json:"rls_reason,omitempty" jsonschema:"penjelasan bila RLS tidak mengikat"`
	OverallReady bool   `json:"overall_ready" jsonschema:"true bila DB terjangkau DAN RLS mengikat"`
}

func (d *deps) runtimeHealth(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, healthOut, error) {
	ctx, cancel := d.ctxWith(ctx)
	defer cancel()
	var out healthOut

	// Ping lebih dulu — ini logika Readiness yang sama (handler/health.go).
	if err := d.pool.Ping(ctx); err != nil {
		out.RLSReason = "database tak bisa di-ping: " + err.Error()
		return nil, out, nil // bukan error tool: "tak sehat" adalah jawaban yang sah
	}
	out.DBReachable = true

	// Probe RLS pada tabel ber-tenant. CheckRLS memakai pool langsung — tak butuh
	// tenant/scope, cocok untuk MCP tanpa request context.
	st, err := db.CheckRLS(ctx, d.pool, "audit_logs")
	if err != nil {
		out.RLSReason = "gagal memeriksa RLS: " + err.Error()
		return nil, out, nil
	}
	out.ConnRole = st.User
	out.RLSBinds = st.Binds()
	if !out.RLSBinds {
		out.RLSReason = st.Reason()
	}
	out.OverallReady = out.DBReachable && out.RLSBinds
	return nil, out, nil
}

// --- preflight ---

type preflightOut struct {
	OK       bool             `json:"ok" jsonschema:"true bila lingkungan siap"`
	Problems []preflightIssue `json:"problems" jsonschema:"daftar masalah beserta cara perbaikannya"`
}

type preflightIssue struct {
	What string   `json:"what"`
	Why  string   `json:"why,omitempty"`
	Fix  []string `json:"fix,omitempty"`
}

func (d *deps) preflight(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, preflightOut, error) {
	ctx, cancel := d.ctxWith(ctx)
	defer cancel()
	// AutoCreateDB:false WAJIB — ini diagnosis, bukan perbaikan. Alat baca yang
	// diam-diam membuat database tak bisa lagi dipakai menjawab "apa sebenarnya
	// keadaan di sini". (Pola sama dengan `make doctor`.)
	rep := preflight.Run(ctx, preflight.Opts{
		DatabaseURL:  d.cfg.DatabaseURL,
		RedisAddr:    d.cfg.RedisAddr,
		AutoCreateDB: false,
	})
	// Slice non-nil sejak awal → JSON "problems":[] saat kosong, bukan null.
	// Klien (agent) tak perlu membedakan "tak ada masalah" dari "field absen".
	out := preflightOut{OK: rep.OK(), Problems: []preflightIssue{}}
	for _, p := range rep.Problems {
		out.Problems = append(out.Problems, preflightIssue{What: p.What, Why: p.Why, Fix: p.Fix})
	}
	return nil, out, nil
}

// --- migration_version ---

type migrationOut struct {
	Version int64 `json:"version" jsonschema:"version_id migrasi tertinggi yang sudah diterapkan; 0 bila belum ada migrasi"`
}

func (d *deps) migrationVersion(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, migrationOut, error) {
	ctx, cancel := d.ctxWith(ctx)
	defer cancel()
	var out migrationOut
	// Satu-satunya query baru di seluruh paket — tak ada helper Go yang membaca
	// versi goose, dan sqlc tak menyentuh goose_db_version (tabel bookkeeping).
	// Dibaca dari pool langsung: tabel ini BUKAN ber-RLS (di-exclude ERD juga),
	// jadi tak perlu WithSuper. Tetap read-only — hanya SELECT.
	//
	// COALESCE(max,0): tabel ada tapi kosong = "belum migrasi", bukan NULL yang
	// menggagalkan Scan.
	err := d.pool.QueryRow(ctx,
		"SELECT COALESCE(max(version_id), 0) FROM goose_db_version").Scan(&out.Version)
	if err != nil {
		return nil, out, err // error tool tepat di sini: query gagal = ada yang salah
	}
	return nil, out, nil
}

// --- platform_stats ---

type statsOut struct {
	Tenants        int64             `json:"tenants" jsonschema:"jumlah workspace (termasuk suspended/archived)"`
	QuotaOverrides int64             `json:"quota_overrides" jsonschema:"jumlah user dengan kuota khusus"`
	Settings       map[string]string `json:"settings" jsonschema:"pengaturan platform aktif (key→value)"`
}

// exposedSettings = ALLOWLIST key platform_settings yang aman dibaca agent MCP.
//
// Allowlist, BUKAN "ekspos semua". Bedanya menentukan apa yang terjadi saat
// KEY BARU ditambahkan ke tabel kelak: dengan allowlist ia otomatis TAK muncul
// (aman by default) sampai seseorang sengaja menaruhnya di sini — dan di titik
// itulah ia menimbang "apakah nilai ini aman dibaca agent?". Tanpa allowlist,
// menambah mis. kredensial SMTP ke platform_settings akan membocorkannya lewat
// tool ini secara diam-diam, dan penambahnya (yang sedang mengurus fitur email,
// bukan MCP) takkan pernah sadar. Ini pelanggaran Rule 7/12 yang menunggu terjadi.
//
// Arah aman jadi default — pola yang sama dengan RLS deny-default, Casbin
// deny-default, dan rute MCP opt-in.
var exposedSettings = map[string]bool{
	settings.KeyWorkspaceQuotaDefault: true,
	settings.KeyAuditRetentionDays:    true,
}

func (d *deps) platformStats(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, statsOut, error) {
	ctx, cancel := d.ctxWith(ctx)
	defer cancel()
	out := statsOut{Settings: map[string]string{}}
	err := db.WithSuper(ctx, d.pool, func(q *db.Queries) error {
		n, err := q.CountTenantsForPlatform(ctx)
		if err != nil {
			return err
		}
		out.Tenants = n

		ov, err := q.CountQuotaOverrides(ctx)
		if err != nil {
			return err
		}
		out.QuotaOverrides = ov

		rows, err := q.ListSettings(ctx)
		if err != nil {
			return err
		}
		for _, s := range rows {
			// Hanya yang di-allowlist — key tak dikenal (mungkin sensitif, mungkin
			// ditambahkan setelah kode ini ditulis) SENGAJA dilewati.
			if exposedSettings[s.Key] {
				out.Settings[s.Key] = s.Value
			}
		}
		return nil
	})
	return nil, out, err
}
