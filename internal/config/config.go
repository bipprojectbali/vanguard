// Package config memuat konfigurasi dari environment ke struct typed.
// Semua env dibaca HANYA di sini — tidak tersebar ke package lain.
package config

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// MinSessionKeyLen = panjang minimum SESSION_KEY di production. 32 karakter
// setara ~192 bit bila di-generate acak (base64) — cukup jauh di atas ambang
// tebak-paksa, dan cukup rendah untuk tak menolak kunci yang sah.
const MinSessionKeyLen = 32

// MinMCPTokenLen = panjang minimum MCP_TOKEN bila diisi. Sama dengan SESSION_KEY
// dan alasannya sama: token ini membuka pembacaan runtime database ke pemegangnya,
// jadi harus tak-bisa-ditebak. Kosong (fitur mati) sah; diisi tapi lemah tidak.
const MinMCPTokenLen = 32

// Config menampung seluruh konfigurasi runtime aplikasi.
type Config struct {
	Port string // PORT, default "8080"
	// DatabaseURL = SATU-SATUNYA DSN. Role owner (migrasi butuh ALTER/CREATE
	// POLICY); hak diturunkan ke app_rw per-transaksi lewat SET LOCAL ROLE, jadi
	// RLS tetap mengikat runtime tanpa koneksi kedua. Lihat db.dropPrivileges.
	DatabaseURL string // DATABASE_URL (wajib)
	RedisAddr   string // REDIS_ADDR (wajib)
	Env         string // ENV: "dev" | "production"
	AutoMigrate bool   // AUTO_MIGRATE, default true
	SessionKey  string // SESSION_KEY (wajib di production)

	// Google OAuth. Opsional di dev (tombol Google nonaktif bila kosong),
	// WAJIB di production (Google = jalur login utama).
	GoogleClientID     string // GOOGLE_CLIENT_ID
	GoogleClientSecret string // GOOGLE_CLIENT_SECRET

	// AppBaseURL = alamat PUBLIK aplikasi tanpa trailing slash, mis.
	// "https://app.example.com". APP_BASE_URL.
	//
	// Menggantikan GOOGLE_REDIRECT_URL: path callback sudah ada di routes.go
	// (handler.PathGoogleCallback), jadi menyimpannya lagi di env berarti satu
	// kebenaran di dua tempat — dan salah ketik berujung `redirect_uri_mismatch`
	// yang tak menyebut sebabnya. Dari base ini dirakit redirect_uri OAuth DAN
	// tautan undangan (yang sebelumnya menebak host dari header request — salah
	// di belakang proxy, dan dapat dipalsukan klien).
	AppBaseURL string

	// SuperAdminEmails = super-admin "sejati" (root immutable). Email di sini
	// selalu super_admin & kebal demote/block/delete lewat app. SUPER_ADMIN_EMAILS
	// dipisah koma. Disimpan lower-case untuk perbandingan case-insensitive.
	SuperAdminEmails []string

	// AppTimezone = zona waktu untuk agregasi tampilan (mis. "jam berapa user
	// aktif" di panel logs). Data disimpan UTC; TZ ini hanya untuk konversi saat
	// baca. APP_TIMEZONE (IANA, mis. "Asia/Jakarta"). Divalidasi saat load.
	AppTimezone string

	// MaxWorkspacesPerUser = kuota DEFAULT workspace yang boleh DIMILIKI (role
	// owner) tiap user; dipakai saat user dibuat. Kolom users.workspace_quota
	// menyimpannya per-user agar platform bisa override tanpa ubah env (lever
	// monetisasi: free/pro/enterprise). MAX_WORKSPACES_PER_USER, default 3.
	MaxWorkspacesPerUser int

	// AppName = nama aplikasi; dipakai membuat workspace PRIMER saat boot pertama.
	// Slug-nya KONSTAN "app" (bukan dari nama ini) agar alamatnya sama di semua
	// deployment, dan mengganti nama aplikasi tak mematikan tautan. APP_NAME,
	// default "App".
	//
	// TIDAK ADA APP_MODE di sini — mode tenancy hidup di DATABASE (0007). Env bisa
	// dibalik; baris DB-nya dijaga trigger yang menolak penurunan.
	AppName string

	// MCPToken = rahasia Bearer yang menjaga rute /mcp (server MCP read-only).
	// MCP_TOKEN, default KOSONG.
	//
	// Kosong = rute /mcp TAK didaftarkan sama sekali (opt-in). Ini pengaman
	// utama: fitur yang membuka runtime ke agent AI tak boleh menyala hanya
	// karena lupa — ia menyala HANYA bila token sengaja diisi. Jadi dev lokal
	// yang tak mengisinya tak punya endpoint MCP HTTP terbuka (dev pakai stdio
	// `./app mcp`, bukan HTTP).
	//
	// Bila diisi, WAJIB >= MinMCPTokenLen (divalidasi di MustLoad): token lemah
	// pada endpoint yang membaca database jauh lebih berbahaya daripada tak ada
	// endpoint — pola yang sama dengan SESSION_KEY.
	MCPToken string
}

// SessionCookieName menurunkan nama cookie sesi dari SESSION_KEY, sehingga dua
// deployment di host yang sama tak saling menimpa sesi (cookie di-scope per-host,
// bukan per-port maupun per-path). Yang dipakai HASH-nya, bukan kuncinya —
// nama cookie terlihat di browser, jadi menaruh kunci di sana justru
// membocorkannya.
//
// Kosong (dev tanpa SESSION_KEY) → "" agar scs memakai default-nya.
func (c *Config) SessionCookieName() string {
	if c.SessionKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(c.SessionKey))
	return "session_" + hex.EncodeToString(sum[:4])
}

// Location mem-parse AppTimezone jadi *time.Location. Sudah divalidasi di
// MustLoad, jadi aman; fallback UTC bila entah bagaimana gagal.
func (c *Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.AppTimezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// IsSuperAdminEmail melaporkan apakah email termasuk root super-admin dari env.
func (c *Config) IsSuperAdminEmail(email string) bool {
	e := strings.ToLower(strings.TrimSpace(email))
	for _, s := range c.SuperAdminEmails {
		if s == e {
			return true
		}
	}
	return false
}

// GoogleEnabled melaporkan apakah OAuth Google terkonfigurasi. AppBaseURL ikut
// disyaratkan: tanpa alamat publik, redirect_uri tak bisa dirakit — dan Google
// menolak permintaan tanpa redirect_uri yang cocok dengan Console.
func (c *Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.AppBaseURL != ""
}

// IsProduction melaporkan apakah aplikasi berjalan di mode production.
func (c *Config) IsProduction() bool { return c.Env == "production" }

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

// LoadDotEnv memuat pasangan key=value dari file .env ke environment
// bila belum di-set. Tanpa dependency — parser sederhana untuk dev.
// Baris kosong dan yang diawali '#' diabaikan. Aman bila file tidak ada.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // .env opsional (mis. di production pakai env asli)
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), unquote(strings.TrimSpace(val))
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	return nil
}

// unquote melepas SATU pasang kutip pembungkus ("..." atau '...') dari nilai .env
// — perilaku dotenv standar. Tanpa ini, DATABASE_URL="postgres://..." dibaca
// LITERAL berikut kutipnya → pgx gagal parse (database kosong). Kutip di tengah
// nilai tak tersentuh; hanya pasangan pembungkus persis yang dilepas.
func unquote(s string) string {
	if len(s) >= 2 {
		if c := s[0]; (c == '"' || c == '\'') && s[len(s)-1] == c {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// getEnvInt membaca env sebagai int positif. Kosong / tak valid / <=0 → fallback
// (jangan biarkan salah ketik jadi kuota 0 yang mengunci semua user).
func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// parseEmailList memecah string comma-separated jadi slice email lower-case,
// membuang entri kosong. Dipakai untuk SUPER_ADMIN_EMAILS.
func parseEmailList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		e := strings.ToLower(strings.TrimSpace(part))
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func mustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("config: env %q wajib di-set", key))
	}
	return v
}
