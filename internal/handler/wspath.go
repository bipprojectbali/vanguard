package handler

import (
	"context"
	"net/http"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// wspath.go — pembentukan URL workspace. SATU-SATUNYA tempat literal "/w/"
// muncul (Rule 15): setelah 0004 setiap tautan workspace bergantung slug, jadi
// path tak boleh lagi ditulis tangan yang tersebar.

// WorkspacePrefix = segmen akar SEMUA route ber-workspace, di kedua mode. Prefix
// eksplisit (bukan "/{slug}" telanjang) membuat tabrakan slug-vs-route MUSTAHIL
// secara struktural — tak perlu daftar kata terlarang yang dijaga selamanya.
const WorkspacePrefix = "/w"

// slugURLParam = nama parameter chi di pola route ("/w/{workspace}/...").
const slugURLParam = "workspace"

// wsPath membentuk URL halaman di dalam ruang kerja — SATU bentuk untuk kedua
// mode (keputusan 0007):
//
//	multi  → wsPath("acme", "/members") = "/w/acme/members"
//	single → wsPath("app",  "/members") = "/w/app/members"
//
// Mode single dulu punya bentuk sendiri (/app/...). Akibatnya menaikkan aplikasi
// ke multi mengubah SETIAP alamat yang sudah tersebar — bookmark, tautan di
// email, dokumentasi turunan. Dengan satu bentuk, kenaikan mode tak menyentuh
// satu tautan pun: /w/app hari ini adalah /w/app besok, cuma kini ada tetangganya.
//
// Yang dibayar: "/w/" muncul di URL aplikasi satu-workspace. Satu huruf, dan itu
// bukan kata "workspace" yang dulu ingin disembunyikan.
//
// Slug kosong (mode multi, belum punya workspace) → "/workspace/new":
// satu-satunya tujuan masuk akal, sekaligus mencegah URL rusak "/w//members".
func wsPath(slug, sub string) string {
	if slug == "" {
		return "/workspace/new"
	}
	base := WorkspacePrefix + "/" + slug
	if sub == "" || sub == "/" {
		return base
	}
	if !strings.HasPrefix(sub, "/") {
		sub = "/" + sub
	}
	return base + sub
}

// wsPathOf = wsPath memakai workspace aktif di session. Dipakai jalur yang tak
// punya slug di URL-nya sendiri (redirect setelah login, landing, /notifications).
func wsPathOf(ctx context.Context, sub string) string {
	return wsPath(session.TenantSlug(ctx), sub)
}

// slugFromRequest membaca slug workspace dari path route ("" bila route ini tak
// ber-workspace, mis. /dev atau /notifications).
//
// Tak ada cabang mode di sini, dan itu inti keputusan 0007: karena kedua mode
// memakai bentuk /w/{slug}, slug SELALU ada di path. Sebelumnya fungsi ini harus
// mengarang slug di mode single (route /app tak punya segmen untuk dibaca) —
// satu tempat yang harus tahu sedang di mode apa, dan satu asumsi yang bisa
// meleset saat mode berubah saat jalan.
func slugFromRequest(r *http.Request) string {
	return chi.URLParam(r, slugURLParam)
}

// wsRedirect mengalihkan (303) ke halaman di dalam workspace request ini,
// opsional dengan kode error PRG (?err=CODE). Slug diambil dari path — bukan
// session — agar redirect selalu kembali ke workspace yang SEDANG dibuka, bukan
// ke yang kebetulan aktif di cookie.
//
// Ada karena tiap handler workspace punya banyak cabang gagal; tanpa ini, literal
// path berulang di puluhan tempat (Rule 15) dan satu saja yang lupa diperbarui
// akan melempar user ke workspace lain secara senyap.
func wsRedirect(w http.ResponseWriter, r *http.Request, sub, errCode string) {
	url := wsPath(slugFromRequest(r), sub)
	if errCode != "" {
		url += "?err=" + errCode
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// wsRedirectOK mengalihkan (303) ke halaman workspace request ini dengan kode
// SUKSES PRG (?ok=CODE) — kembaran wsRedirect untuk jalur berhasil. Dipisah agar
// ?ok= (alert sukses) & ?err= (alert galat) tak pernah tertukar di pemanggil:
// keduanya string bebas, dan satu argumen yang keliru tempat akan menampilkan
// pesan sukses bervarian galat (atau sebaliknya).
func wsRedirectOK(w http.ResponseWriter, r *http.Request, sub, okCode string) {
	url := wsPath(slugFromRequest(r), sub)
	if okCode != "" {
		url += "?ok=" + okCode
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// homeFor mengembalikan tujuan "rumah" setelah login/dari landing. Role PLATFORM
// → /dev (lintas-workspace, tak punya slug); role tenant → akar workspace aktif.
//
// Menggantikan authz.HomePath untuk role tenant: sejak 0004 alamatnya bergantung
// WORKSPACE, bukan role. Orang yang sama owner di A & member di B — dulu itu
// berarti /admin vs /user, yang membuat alamat halaman yang sama berubah saat
// pindah workspace.
func homeFor(ctx context.Context) string {
	if role := session.Role(ctx); isPlatformRole(role) || session.IsRoot(ctx) {
		return authz.PlatformHomePath
	}
	return wsPathOf(ctx, "")
}
