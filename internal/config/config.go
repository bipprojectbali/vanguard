// Package config memuat konfigurasi dari environment ke struct typed.
// Semua env dibaca HANYA di sini — tidak tersebar ke package lain.
package config

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
