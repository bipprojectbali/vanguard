package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoogleEnabled(t *testing.T) {
	full := Config{GoogleClientID: "a", GoogleClientSecret: "b", AppBaseURL: "https://c"}
	if !full.GoogleEnabled() {
		t.Error("ketiga kredensial terisi → enabled")
	}
	// Tiap satu kosong → disabled.
	for _, c := range []Config{
		{GoogleClientSecret: "b", AppBaseURL: "https://c"},
		{GoogleClientID: "a", AppBaseURL: "https://c"},
		{GoogleClientID: "a", GoogleClientSecret: "b"},
		{},
	} {
		if c.GoogleEnabled() {
			t.Errorf("kredensial tak lengkap harus disabled: %+v", c)
		}
	}
}

func TestClaudeProxyEnabled(t *testing.T) {
	full := Config{ClaudeProxyURL: "https://proxy", ClaudeProxyToken: "sk-cp-x"}
	if !full.ClaudeProxyEnabled() {
		t.Error("URL+token terisi → enabled")
	}
	for _, c := range []Config{
		{ClaudeProxyURL: "https://proxy"},
		{ClaudeProxyToken: "sk-cp-x"},
		{},
	} {
		if c.ClaudeProxyEnabled() {
			t.Errorf("kredensial tak lengkap harus disabled: %+v", c)
		}
	}
}

func TestIsProduction(t *testing.T) {
	if !(&Config{Env: "production"}).IsProduction() {
		t.Error("'production' → true")
	}
	// Exact-match (case-sensitive).
	for _, env := range []string{"dev", "", "Production", "PROD"} {
		if (&Config{Env: env}).IsProduction() {
			t.Errorf("%q tak boleh production", env)
		}
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("GS_TEST_FOO", "bar")
	if getEnv("GS_TEST_FOO", "fallback") != "bar" {
		t.Error("env terisi harus dipakai")
	}
	if getEnv("GS_TEST_UNSET_XYZ", "fallback") != "fallback" {
		t.Error("env tak ada harus fallback")
	}
	// Set-tapi-kosong → "" (bukan fallback) — beda dgn mustEnv.
	t.Setenv("GS_TEST_EMPTY", "")
	if getEnv("GS_TEST_EMPTY", "fallback") != "" {
		t.Error("env set-kosong harus \"\", bukan fallback")
	}
}

// setMinimalEnv menyiapkan env dev minimal (isolasi via t.Setenv).
func setMinimalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	// bersihkan yang bisa mengganggu
	for _, k := range []string{"PORT", "ENV", "AUTO_MIGRATE", "SESSION_KEY",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "APP_BASE_URL", "SUPER_ADMIN_EMAILS"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestMustLoad_DevDefaults(t *testing.T) {
	setMinimalEnv(t)
	c := MustLoad()
	if c.Port != "8080" || c.Env != "dev" || !c.AutoMigrate {
		t.Errorf("default dev salah: port=%q env=%q autoMigrate=%v", c.Port, c.Env, c.AutoMigrate)
	}
	if c.GoogleEnabled() {
		t.Error("dev tanpa kredensial Google → disabled")
	}
}

func TestMustLoad_MissingRequiredPanics(t *testing.T) {
	for _, missing := range []string{"DATABASE_URL", "REDIS_ADDR"} {
		t.Run(missing, func(t *testing.T) {
			setMinimalEnv(t)
			os.Unsetenv(missing)
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("%s hilang harus panic", missing)
				}
			}()
			MustLoad()
		})
	}
}

func TestMustLoad_ProductionRequiresSecrets(t *testing.T) {
	// Production tanpa SESSION_KEY → panic.
	setMinimalEnv(t)
	t.Setenv("ENV", "production")
	func() {
		defer func() {
			if recover() == nil {
				t.Error("production tanpa SESSION_KEY harus panic")
			}
		}()
		MustLoad()
	}()

	// Production dgn semua secret → tak panic. Termasuk SESSION_KEY yang cukup
	// panjang: panjangnya jadi syarat boot (lihat
	// TestMustLoad_ProdTolakSessionKeyLemah).
	setProdEnv(t)
	c := MustLoad()
	if !c.IsProduction() || !c.GoogleEnabled() {
		t.Error("production lengkap harus load bersih")
	}
}

func TestMustLoad_AutoMigrateFlag(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("AUTO_MIGRATE", "false")
	if MustLoad().AutoMigrate {
		t.Error("AUTO_MIGRATE=false → false")
	}
	setMinimalEnv(t)
	t.Setenv("AUTO_MIGRATE", "yes") // hanya "true" literal yang true
	if MustLoad().AutoMigrate {
		t.Error("AUTO_MIGRATE=yes bukan true literal → false")
	}
}

func TestLoadDotEnv_MissingFileIsNil(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Errorf("file tak ada harus nil (opsional): %v", err)
	}
}

func TestLoadDotEnv_Parsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# komentar\n\nKEY_A = val_a \nKEY_B=val_b\nTANPA_SAMA_DENGAN\nKEY_C=c\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// Pastikan bersih dulu.
	for _, k := range []string{"KEY_A", "KEY_B", "KEY_C"} {
		os.Unsetenv(k)
	}
	// KEY_C sudah di-set → TIDAK boleh ditimpa (precedence).
	t.Setenv("KEY_C", "sudah-ada")

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if os.Getenv("KEY_A") != "val_a" { // di-trim
		t.Errorf("KEY_A=%q, want val_a (ter-trim)", os.Getenv("KEY_A"))
	}
	if os.Getenv("KEY_B") != "val_b" {
		t.Errorf("KEY_B=%q", os.Getenv("KEY_B"))
	}
	if os.Getenv("KEY_C") != "sudah-ada" {
		t.Errorf("KEY_C tak boleh ditimpa, got %q", os.Getenv("KEY_C"))
	}
	os.Unsetenv("KEY_A")
	os.Unsetenv("KEY_B")
}

// setProdEnv menyiapkan environment production yang LENGKAP & sah. Test di
// bawah lalu merusak satu hal saja — sehingga yang diuji benar-benar hal itu,
// bukan kombinasi kekurangan.
func setProdEnv(t *testing.T) {
	t.Helper()
	setMinimalEnv(t)
	t.Setenv("ENV", "production")
	t.Setenv("SESSION_KEY", strings.Repeat("k", MinSessionKeyLen))
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("APP_BASE_URL", "https://x.example.com")
}

// mustPanic menjalankan fn dan gagal bila TIDAK panic. Dipakai menguji fail-fast
// config: kegagalan boot adalah perilaku yang disengaja, jadi ia harus diuji
// seperti perilaku lain.
func mustPanic(t *testing.T, nama string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: HARUS panic saat boot, tapi lolos", nama)
		}
	}()
	fn()
}

// TestMustLoad_TakAdaEnvIsolasiTenant: isolasi tenant TIDAK lagi punya env sama
// sekali. Sebelumnya ada APP_DATABASE_URL — satu env yang bisa lupa diisi, dan
// yang lebih buruk: bisa diisi dengan DSN yang SAMA seperti DATABASE_URL ("biar
// aman, samakan saja") sehingga lolos setiap pemeriksaan sambil tetap
// menjalankan pool sebagai owner. Terukur di database nyata: 82 baris dari 15
// tenant tetap terbaca oleh query yang lupa WHERE tenant_id.
//
// Sekarang hak diturunkan per-transaksi (db.dropPrivileges) dan DIBUKTIKAN pada
// koneksi yang sudah terbuka (db.CheckRLSTx). Test ini menjaga agar env berbasis
// janji itu tak dihidupkan kembali — termasuk dengan nama lain.
func TestMustLoad_TakAdaEnvIsolasiTenant(t *testing.T) {
	setProdEnv(t)
	t.Setenv("APP_DATABASE_URL", "postgres://sesat@nowhere/db")
	c := MustLoad() // tak boleh panic, dan tak boleh membacanya

	if c.DatabaseURL == "" {
		t.Fatal("DATABASE_URL harus tetap terbaca")
	}
	// Satu-satunya DSN. Kalau kelak ada field kedua yang menampung env di atas,
	// baris ini tak akan menangkapnya — yang menangkap adalah ketiadaan field itu
	// di struct, dan test ini menandai niatnya.
	if c.DatabaseURL == "postgres://sesat@nowhere/db" {
		t.Error("APP_DATABASE_URL tak boleh lagi memengaruhi konfigurasi apa pun")
	}
}

// TestMustLoad_ProdTolakSessionKeyLemah: mustEnv hanya memastikan env TERISI.
// "SESSION_KEY=rahasia" lolos pemeriksaan itu tapi tak melindungi apa pun —
// justru menciptakan rasa aman yang keliru, yang lebih berbahaya daripada
// kosong (kosong setidaknya menggagalkan boot).
func TestMustLoad_ProdTolakSessionKeyLemah(t *testing.T) {
	setProdEnv(t)
	t.Setenv("SESSION_KEY", "rahasia")
	mustPanic(t, "SESSION_KEY pendek", func() { MustLoad() })

	setProdEnv(t)
	t.Setenv("SESSION_KEY", "")
	mustPanic(t, "SESSION_KEY kosong", func() { MustLoad() })
}

// TestMustLoad_ProdLengkapLolos: pagar-pagar di atas tak boleh menolak
// konfigurasi production yang benar.
func TestMustLoad_ProdLengkapLolos(t *testing.T) {
	setProdEnv(t)
	c := MustLoad()
	if !c.IsProduction() {
		t.Fatal("harus terbaca sebagai production")
	}
}

// TestSessionCookieName_TakMembocorkanKunci: nama cookie TERLIHAT di browser.
// Menaruh SESSION_KEY di sana berarti membocorkannya ke siapa pun yang membuka
// devtools — nama diturunkan dari HASH, bukan dari kuncinya.
func TestSessionCookieName_TakMembocorkanKunci(t *testing.T) {
	key := strings.Repeat("s", MinSessionKeyLen)
	c := &Config{SessionKey: key}
	name := c.SessionCookieName()

	if strings.Contains(name, key) || strings.Contains(name, key[:8]) {
		t.Fatalf("nama cookie %q memuat potongan SESSION_KEY", name)
	}
	if name == "" {
		t.Error("dengan SESSION_KEY terisi, nama cookie harus ditetapkan")
	}
	// Deterministik: nama cookie harus sama tiap boot, kalau tidak semua sesi
	// hangus setiap kali aplikasi restart.
	if again := c.SessionCookieName(); again != name {
		t.Errorf("nama cookie harus stabil: %q vs %q", name, again)
	}
	// Kunci berbeda → nama berbeda (itulah gunanya: memisahkan deployment).
	other := &Config{SessionKey: strings.Repeat("z", MinSessionKeyLen)}
	if other.SessionCookieName() == name {
		t.Error("deployment dengan kunci berbeda harus punya nama cookie berbeda")
	}
	// Dev tanpa SESSION_KEY → biarkan default scs.
	if (&Config{}).SessionCookieName() != "" {
		t.Error("tanpa SESSION_KEY harus memakai default scs")
	}
}
