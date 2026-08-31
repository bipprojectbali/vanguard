package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// config_load.go — pemuatan konfigurasi fail-fast: MustLoad (runtime penuh) &
// LoadMigrateConfig (minimal untuk subcommand migrate). Dipisah agar file di
// bawah ambang tipe Config (100). Env dibaca HANYA di paket config.

// MustLoad memuat konfigurasi dengan fail-fast: env wajib yang kosong
// menyebabkan panic saat startup, bukan error senyap di request pertama.
func MustLoad() *Config {
	c := &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          mustEnv("DATABASE_URL"),
		RedisAddr:            mustEnv("REDIS_ADDR"),
		Env:                  getEnv("ENV", "dev"),
		AutoMigrate:          getEnv("AUTO_MIGRATE", "true") == "true",
		GoogleClientID:       getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:   getEnv("GOOGLE_CLIENT_SECRET", ""),
		AppBaseURL:           strings.TrimRight(getEnv("APP_BASE_URL", ""), "/"),
		SuperAdminEmails:     parseEmailList(getEnv("SUPER_ADMIN_EMAILS", "")),
		AppTimezone:          getEnv("APP_TIMEZONE", "Asia/Jakarta"),
		MaxWorkspacesPerUser: getEnvInt("MAX_WORKSPACES_PER_USER", 3),
		AppName:              getEnv("APP_NAME", "App"),
		MCPToken:             getEnv("MCP_TOKEN", ""),
	}
	// MCP_TOKEN opsional & lintas-lingkungan (bukan cuma production): kalau diisi,
	// panjangnya divalidasi di MANA PUN — endpoint MCP yang membaca database sama
	// berbahayanya dijaga token lemah di dev maupun prod. Kosong = fitur mati, sah.
	if c.MCPToken != "" && len(c.MCPToken) < MinMCPTokenLen {
		panic(fmt.Sprintf("config: MCP_TOKEN terlalu pendek (%d karakter, minimal %d) — "+
			"kosongkan untuk mematikan MCP, atau buat dengan: openssl rand -base64 48",
			len(c.MCPToken), MinMCPTokenLen))
	}
	// Validasi TZ fail-fast: nama IANA salah → panic saat startup, bukan error
	// senyap saat render panel logs. (tzdata di-embed via import di main.)
	if _, err := time.LoadLocation(c.AppTimezone); err != nil {
		panic(fmt.Sprintf("config: APP_TIMEZONE %q tidak valid: %v", c.AppTimezone, err))
	}
	// Isolasi tenant TIDAK diurus di sini, dan tak butuh env sama sekali: hak
	// diturunkan ke app_rw di dalam setiap transaksi (db.dropPrivileges), lalu
	// DIBUKTIKAN pada koneksi yang sudah terbuka (db.CheckRLS, dipanggil main).
	// Dulu ini menuntut APP_DATABASE_URL — satu env yang bisa lupa diisi, dan
	// yang lebih buruk: bisa diisi dengan DSN yang sama seperti DATABASE_URL,
	// lolos setiap pemeriksaan sambil tetap membocorkan data.
	if c.IsProduction() {
		c.SessionKey = mustEnv("SESSION_KEY")
		// Kunci lemah = kunci yang tak ada. Divalidasi PANJANGNYA, bukan cuma
		// keberadaannya: "SESSION_KEY=rahasia" lolos mustEnv tapi tak melindungi
		// apa pun, dan justru menciptakan rasa aman yang keliru.
		if len(c.SessionKey) < MinSessionKeyLen {
			panic(fmt.Sprintf("config: SESSION_KEY terlalu pendek (%d karakter, minimal %d) — "+
				"buat dengan: openssl rand -base64 48", len(c.SessionKey), MinSessionKeyLen))
		}
		// Di production Google adalah jalur login utama — wajib ada.
		c.GoogleClientID = mustEnv("GOOGLE_CLIENT_ID")
		c.GoogleClientSecret = mustEnv("GOOGLE_CLIENT_SECRET")
		c.AppBaseURL = strings.TrimRight(mustEnv("APP_BASE_URL"), "/")
		// HTTPS wajib: cookie sesi di-set Secure di production, jadi base URL
		// http:// menghasilkan aplikasi yang tampak jalan tapi login-nya tak
		// pernah berhasil — kegagalan yang mahal dilacak.
		if !strings.HasPrefix(c.AppBaseURL, "https://") {
			panic(fmt.Sprintf("config: APP_BASE_URL harus https:// di production (got %q) — "+
				"cookie sesi Secure tak akan terkirim lewat http", c.AppBaseURL))
		}
	}
	return c
}

// LoadMigrateConfig memuat konfigurasi MINIMAL untuk subcommand `migrate`:
// HANYA DATABASE_URL. Sengaja TIDAK memakai MustLoad — migrate cuma butuh
// koneksi DB, sedang MustLoad mewajibkan SESSION_KEY/Google/REDIS di production
// (yang tak relevan untuk container migrate-only). Return error (bukan panic)
// agar caller (runMigrate) bisa exit(1) rapi. Env tetap dibaca HANYA di sini.
func LoadMigrateConfig() (*Config, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("config: env %q wajib di-set untuk migrate", "DATABASE_URL")
	}
	return &Config{DatabaseURL: url}, nil
}
