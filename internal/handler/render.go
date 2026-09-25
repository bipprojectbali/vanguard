package handler

import "time"

// cssPath adalah path app.css dengan cache-bust hash, di-set saat startup.
var cssPath = "/static/app.css"

// SetCSSPath menetapkan path CSS ber-hash (dipanggil dari main saat startup).
func SetCSSPath(p string) { cssPath = p }

// dealboardJSPath = path dealboard.js dengan cache-bust hash, di-set saat
// startup (SAMA pola cssPath). SEBELUMNYA di-hardcode "/static/dealboard.js"
// di view (sales_deals.go) TANPA hash — browser (atau proxy) yang meng-cache
// respons lama bisa menyajikan JS basi tanpa jalan untuk memaksa refresh;
// ditambah setelah insiden dev-server sempat menyajikan dealboard.js basi
// (air.toml exclude_dir static/ + go:embed, lihat docs/crm/tasks.md BL-75)
// yang nyaris identik gejalanya dgn browser-cache basi — hash mencegah KEDUA
// kelas masalah ini sekaligus utk aset ini ke depannya.
var dealboardJSPath = "/static/dealboard.js"

// SetDealboardJSPath menetapkan path dealboard.js ber-hash (dipanggil dari
// main saat startup).
func SetDealboardJSPath(p string) { dealboardJSPath = p }

// appName = nama aplikasi untuk brand & judul halaman. Di-inject dari config
// (APP_NAME) via SetAppName, mengikuti pola setter global lainnya di file ini.
//
// Sebelumnya "go_starter" di-hardcode di tujuh tempat. Untuk sebuah TEMPLATE
// yang memang dimaksudkan di-clone, itu bentuk hardcode yang paling mahal:
// bukan cuma melanggar Rule 15, tapi membuat setiap project turunan memampangkan
// nama template-nya di sidebar sampai ada yang menyisirnya satu per satu.
//
// Default "App" menyamai config.getEnv("APP_NAME", "App") — dua tempat yang
// menyimpan default berbeda akan tampak sebagai nama yang berubah-ubah
// tergantung jalur mana yang menyetelnya.
var appName = "App"

// SetAppName menetapkan nama aplikasi (dipanggil dari main saat startup).
func SetAppName(n string) {
	if n != "" {
		appName = n
	}
}

// AppName mengembalikan nama aplikasi yang aktif.
func AppName() string { return appName }

// devBrand = label brand panel platform, mis. "Acme /dev".
func devBrand() string { return appName + " /dev" }

// devMode menandai environment non-production. Menentukan apakah form login
// password ditampilkan (password auth = dev-only; produksi hanya Google).
var devMode bool

// SetDevMode menetapkan flag dev (dipanggil dari main: !cfg.IsProduction()).
func SetDevMode(v bool) { devMode = v }

// isSuperAdminEmail memeriksa apakah email = super-admin env (root immutable).
// Di-inject dari main (config.IsSuperAdminEmail) agar handler tak import config.
var isSuperAdminEmail = func(string) bool { return false }

// SetSuperAdminChecker menyuntik fungsi cek super-admin env dari config.
func SetSuperAdminChecker(fn func(string) bool) { isSuperAdminEmail = fn }

// appTZ = zona waktu untuk agregasi tampilan panel logs (tampilan jam lokal).
// Default UTC bila belum di-set; di-inject dari main via SetAppTimezone.
var appTZ = time.UTC

// SetAppTimezone menetapkan zona waktu aplikasi (dipanggil dari main saat startup).
func SetAppTimezone(loc *time.Location) {
	if loc != nil {
		appTZ = loc
	}
}
