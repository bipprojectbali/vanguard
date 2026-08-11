// Command go_starter — entry point: config, wiring, start server.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata" // embed database tzdata: LoadLocation gagal di container minimal (CGO_ENABLED=0) tanpa ini

	"go_starter/internal/appmode"
	"go_starter/internal/assets"
	"go_starter/internal/authz"
	"go_starter/internal/config"
	"go_starter/internal/database"
	"go_starter/internal/db"
	"go_starter/internal/handler"
	"go_starter/internal/maintenance"
	"go_starter/internal/mcpserver"
	"go_starter/internal/oauth"
	"go_starter/internal/preflight"
	"go_starter/internal/session"
	"go_starter/internal/settings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
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

func run() (err error) {
	// Config (dev: muat .env dulu bila ada).
	_ = config.LoadDotEnv(".env")

	// config.MustLoad PANIC untuk env yang salah/kurang — sengaja (§ "gagal
	// keras"), dan pesannya sudah menjelaskan sebabnya. Yang tak berguna adalah
	// BENTUKNYA: stack trace lima baris mengubur satu kalimat yang justru harus
	// dibaca, dan orang berhenti membacanya setelah kali kedua.
	//
	// Ditangkap di sini lalu dijadikan error biasa — nilainya tetap sama (boot
	// gagal, tak ada yang berjalan setengah), tapi yang tampil di terminal adalah
	// kalimatnya, bukan jejak goroutine.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v\n\nJalankan `make doctor` untuk memeriksa seluruh lingkungan sekaligus", r)
		}
	}()
	cfg := config.MustLoad()

	log := newLogger(cfg)
	slog.SetDefault(log)

	// ctx dibatalkan saat SIGINT/SIGTERM → memicu graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Preflight: periksa lingkungan SEBELUM menyentuh apa pun, dan kalau ada yang
	// kurang, katakan apa yang harus dilakukan.
	//
	// Tanpa ini boot gagal dengan `database "x" does not exist (SQLSTATE 3D000)` —
	// benar secara harfiah, tapi menyembunyikan yang justru dibutuhkan
	// penerimanya: database mirip mana yang ADA di server itu (salah ketik nyaris
	// selalu beda tipis), dan perintah persis untuk membereskannya.
	//
	// AutoCreateDB HANYA di dev. Di production, membuat database yang belum ada
	// mengubah DSN salah ketik jadi database kosong yang tampak sehat — aplikasi
	// mulai melayani seolah datanya hilang. Itu kegagalan senyap, dan § "gagal
	// keras" ada justru untuk menolaknya.
	if rep := preflight.Run(ctx, preflight.Opts{
		DatabaseURL:  cfg.DatabaseURL,
		RedisAddr:    cfg.RedisAddr,
		AutoCreateDB: !cfg.IsProduction(),
		// .env hanya dicek di dev (boot dari direktori repo). Di production env
		// datang dari container/Portainer — .env tak ada & tak relevan; mengeceknya
		// menolak boot karena file yang memang tak seharusnya ada.
		FromFile: !cfg.IsProduction(),
	}); !rep.OK() {
		return errors.New(rep.String())
	}

	// Postgres — SATU pool, satu DSN. Koneksi terbuka sebagai owner (migrasi butuh
	// ALTER/CREATE POLICY), lalu setiap transaksi aplikasi menurunkan haknya ke
	// app_rw lewat SET LOCAL ROLE (db.WithTenant/WithSuper) sehingga RLS mengikat.
	//
	// Dulu ini dua pool dengan dua DSN, karena owner selalu bypass RLS. Harganya:
	// satu env lagi yang bisa lupa diisi, satu password lagi yang bisa bocor, satu
	// entri lagi di userlist.txt PgBouncer — dan bila DSN kedua diisi sama dengan
	// yang pertama, semuanya lolos sambil tetap membocorkan data.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Redis (rueidis) — satu klien untuk session store.
	rc, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{cfg.RedisAddr},
	})
	if err != nil {
		return err
	}
	defer rc.Close()

	// Session manager (scs + rueidis store). Opsi keamanan DITURUNKAN dari
	// ENV=production — bukan env tersendiri yang bisa lupa diisi.
	sm := session.NewManager(rc, session.Options{
		Secure:     cfg.IsProduction(),
		CookieName: cfg.SessionCookieName(),
	})
	session.Init(sm)

	// Auto-migrate dengan advisory lock (aman multi-instance). Memakai pool
	// LANGSUNG — bukan lewat WithTenant/WithSuper — supaya haknya tetap owner:
	// ALTER/CREATE POLICY di luar jangkauan app_rw, dan memang harus begitu.
	if cfg.AutoMigrate {
		if err := database.MigrateWithLock(ctx, pool, migrationsEmbed); err != nil {
			return err
		}
		log.Info("migrations applied")
	}

	// Model role 2-bidang: super_admin = ENV-ONLY (SUPER_ADMIN_EMAILS), nol baris
	// DB, di-overlay RefreshIdentity per-request. Tak ada reconcile boot ke DB —
	// role users.role hanya owner/admin/member (CHECK). Kehadiran super_admin tak
	// pernah ditulis; sepenuhnya diturunkan dari env saat identity di-resolve.

	// Static server dengan cache-busting untuk app.css (berubah tiap `make css`).
	staticSub, err := fs.Sub(staticEmbed, "static")
	if err != nil {
		return err
	}
	assetSrv, err := assets.New(staticSub, "app.css")
	if err != nil {
		return err
	}
	// Workspace primer + mode tenancy, keduanya dari DATABASE (0007). Mode tak
	// lagi datang dari env: env bisa dibalik, dan menurunkannya kembali ke single
	// setelah ada banyak workspace menyembunyikan sisanya. Nilainya kini dijaga
	// trigger DB yang menolak penurunan — mustahil, bukan sekadar dicegah.
	//
	// Dijalankan setelah migrasi (tabelnya harus ada) dan sebelum route
	// didaftarkan (menu bergantung mode).
	mode, err := handler.BootstrapPrimary(ctx, pool, cfg.AppName)
	if err != nil {
		return err
	}
	appmode.Set(mode)
	log.Info("mode tenancy", "mode", mode.String())

	// BUKTIKAN isolasi tenant mengikat — jangan percaya konfigurasi. Dijalankan
	// SETELAH migrasi (tabel & policy harus sudah ada), SEBELUM melayani request.
	if err := verifyTenantIsolation(ctx, pool, log); err != nil {
		return err
	}

	// Alamat publik aplikasi — sumber redirect_uri OAuth & tautan undangan.
	// Di-set SEBELUM wiring OAuth di bawah (yang merakit redirect_uri darinya).
	handler.SetAppBaseURL(cfg.AppBaseURL)
	handler.SetAppName(cfg.AppName)              // brand sidebar & judul halaman
	handler.SetCSSPath(assetSrv.Path("app.css")) // inject path ber-hash ke Layout
	handler.SetDevMode(!cfg.IsProduction())      // password auth = dev-only
	handler.SetSuperAdminChecker(cfg.IsSuperAdminEmail)
	handler.SetAppTimezone(cfg.Location()) // TZ agregasi panel logs (tampilan jam lokal)

	// Pengaturan platform: env jadi FALLBACK (dipakai bila baris DB belum ada,
	// mis. deployment baru), DB jadi sumber kebenaran yang bisa diubah operator
	// saat jalan. Fail-soft: gagal baca → jalan dgn fallback env, sebab kuota
	// yang sedikit basi jauh lebih baik daripada aplikasi menolak start.
	settings.SetFallback(settings.KeyWorkspaceQuotaDefault, strconv.Itoa(cfg.MaxWorkspacesPerUser))
	if kv, err := loadSettings(ctx, pool); err != nil {
		log.Error("settings: gagal memuat, pakai fallback env", "err", err)
	} else {
		settings.Load(kv)
	}

	// Authz (Casbin) — enforcer in-memory dari model+policy embed.
	enforcer, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		return err
	}
	authz.Init(enforcer)

	// Sumbu BISNIS (CRM) — enforcer TERPISAH atas business_role. Sengaja bukan
	// menumpang enforcer di atas: god-mode root & warisan platform→tenant di sana
	// akan membocorkan izin CRM ke owner/super_admin (§3).
	//
	// SEJAK 00007 peran bisa diedit per-workspace → policy dibaca dari DB, bukan
	// CSV embed. Subject di-fold `t<id>:<role>` di authz agar peran senama di dua
	// workspace tak saling memberi izin. Load-nya WAJIB WithSuper: di tenant-tx RLS
	// menyembunyikan tenant lain → enforcer deny-all senyap bagi mereka.
	bEnforcer, err := authz.NewBusinessEmpty()
	if err != nil {
		return err
	}
	perms, err := loadBusinessPerms(ctx, pool)
	if err != nil {
		return err
	}
	if err := authz.LoadBusiness(bEnforcer, perms); err != nil {
		return err
	}
	authz.InitBusiness(bEnforcer)

	// Google OAuth — di-wire bila kredensial tersedia. Di dev tanpa kredensial,
	// app tetap start (tombol Google membalas 503 saat diklik).
	if cfg.GoogleEnabled() {
		// redirect_uri DIRAKIT dari base URL + path route (handler.PathGoogleCallback),
		// bukan dibaca dari env tersendiri: path yang sama tak boleh hidup di dua
		// tempat, sebab salah ketik di salah satunya hanya muncul sebagai
		// `redirect_uri_mismatch` dari Google — pesan yang tak menyebut sebabnya.
		gp, err := oauth.New(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, handler.GoogleRedirectURL())
		if err != nil {
			return err
		}
		handler.SetGoogleOAuth(gp)
		log.Info("google oauth enabled")
	} else {
		log.Warn("google oauth disabled: kredensial GOOGLE_* tidak lengkap")
	}

	// Wiring handler + router.
	h := handler.New(pool, log)
	r := chi.NewRouter()
	// MCP server read-only, dirakit di sini (tempat cfg & pool ada) lalu dioper
	// sebagai http.Handler — routes.go tak perlu tahu isinya. Token kosong =
	// rute tak didaftarkan (opt-in; lihat mcpRoute di routes.go).
	mcpRt := mcpRoute{Token: cfg.MCPToken}
	if cfg.MCPToken != "" {
		mcpRt.Handler = mcpserver.Handler(pool, cfg, log)
		log.Info("MCP read-only route aktif di /mcp")
	}
	registerRoutes(r, h, assetSrv.Handler(), log, !cfg.IsProduction(), mcpRt)

	// Bungkus: CSRF (terluar) → session LoadAndSave → router.
	// CrossOriginProtection butuh Go ≥1.25.1 (CVE-2025-47910 di 1.25.0).
	csrf := http.NewCrossOriginProtection()
	handlerChain := csrf.Handler(sm.LoadAndSave(r))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handlerChain,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // 0 = tanpa batas; SSE butuh koneksi panjang
		IdleTimeout:       60 * time.Second,
	}

	// Pemeliharaan berkala: buang jejak audit kedaluwarsa & purge workspace yang
	// masa tenggangnya habis. Keduanya sudah punya fungsinya sejak lama tapi tak
	// pernah punya PEMICU — jadi audit_logs tumbuh selamanya dan workspace
	// terhapus menumpuk beserta slug-nya yang tak pernah bebas.
	//
	// In-process, sebab ini single-binary: menuntut cron di host berarti
	// pemeliharaan yang "seharusnya sudah dipasang", yaitu yang tak pernah
	// dipasang. Antar-instance dikunci advisory lock, jadi yang menang lomba
	// mengerjakan dan sisanya lewat.
	//
	// ctx yang sama dengan sinyal shutdown → berhenti sendiri saat SIGTERM.
	go (&maintenance.Runner{
		Log: log,
		Tasks: []maintenance.Task{
			{Name: "purge_audit_logs", Run: maintenance.PurgeAuditLogs(pool)},
			{Name: "purge_expired_tenants", Run: maintenance.PurgeExpiredTenants(
				pool, handler.GracePeriodDays*24*time.Hour, log)},
		},
	}).Start(ctx)

	// Jalankan server di goroutine; error dikirim ke channel.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Tunggu: error server ATAU sinyal shutdown.
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		stop() // berhenti tangkap sinyal (SIGTERM kedua = kill paksa)
		log.Info("shutdown: draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		log.Info("shutdown: done")
		return nil
	}
}

// verifyTenantIsolation MEMBUKTIKAN isolasi tenant mengikat — dengan bertanya ke
// database, bukan dengan memeriksa env.
//
// Pemeriksaan dilakukan di dalam WithSuper, artinya pada transaksi yang sudah
// menurunkan haknya ke app_rw — persis keadaan setiap query aplikasi. Memeriksa
// pool telanjang akan menjawab pertanyaan yang salah: di sana koneksi memang
// masih owner, dan memang seharusnya (migrasi butuh itu).
//
// Berlaku SAMA di dev maupun production, dan itu perubahan dari sebelumnya.
// Dulu dev sengaja dilonggarkan karena mengikat RLS menuntut role & DSN kedua —
// gesekan yang tak sepadan untuk `make dev`. Dengan SET LOCAL ROLE gesekan itu
// nol, jadi tak ada lagi alasan membiarkan dev berbeda. Justru sebaliknya:
// query yang lupa `WHERE tenant_id` sekarang gagal di laptop, bukan di produksi.
func verifyTenantIsolation(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	var st db.RLSStatus
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		var e error
		st, e = db.CheckRLSTx(ctx, q, "audit_logs")
		return e
	})
	if err != nil {
		return err
	}
	if st.Binds() {
		log.Info("isolasi tenant: RLS mengikat", "role", st.User)
		return nil
	}
	// Tak mengikat = menolak start, tanpa pengecualian. Bukan kekakuan: penyebab
	// yang mungkin tinggal sedikit sejak APP_DATABASE_URL hilang, dan semuanya
	// berarti jaringnya memang tak terpasang.
	return fmt.Errorf("isolasi tenant TIDAK mengikat: %s.\n"+
		"Setiap transaksi aplikasi menurunkan haknya ke app_rw (SET LOCAL ROLE), jadi ini "+
		"berarti role itu tak terpasang benar. Periksa: migrasi sudah jalan sampai selesai? "+
		"`GRANT app_rw TO CURRENT_USER` ada (dibutuhkan bila DATABASE_URL memakai owner "+
		"non-superuser)? FORCE ROW LEVEL SECURITY masih aktif di tabel ber-tenant?",
		st.Reason())
}

// loadSettings membaca seluruh pengaturan platform jadi map siap-cache.
// platform_settings TANPA RLS (pengaturan berlaku lintas-workspace), jadi
// WithSuper aman — dan memang perlu: saat boot belum ada tenant untuk di-scope.
func loadSettings(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	kv := map[string]string{}
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		rows, e := q.ListSettings(ctx)
		if e != nil {
			return e
		}
		for _, s := range rows {
			kv[s.Key] = s.Value
		}
		return nil
	})
	return kv, err
}

// loadBusinessPerms membaca SELURUH izin CRM (semua tenant) untuk membangun
// enforcer bisnis saat startup. WithSuper WAJIB: business_role_permissions ber-RLS,
// jadi di tenant-tx tenant lain tak terbaca → peran mereka deny-all senyap.
// Baris sqlc dipetakan ke authz.BusinessPerm agar paket authz tetap DB-free.
func loadBusinessPerms(ctx context.Context, pool *pgxpool.Pool) ([]authz.BusinessPerm, error) {
	var out []authz.BusinessPerm
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		rows, e := q.ListAllBusinessRolePermissions(ctx)
		if e != nil {
			return e
		}
		out = make([]authz.BusinessPerm, 0, len(rows))
		for _, r := range rows {
			out = append(out, authz.BusinessPerm{
				TenantID: r.TenantID, Role: r.Role, Obj: r.Obj, Act: r.Act,
			})
		}
		return nil
	})
	return out, err
}

func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
