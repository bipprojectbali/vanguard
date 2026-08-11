package session

import (
	"context"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/redis/rueidis"
)

// Key session — satu-satunya tempat string ini ada, typo jadi mustahil (§4.9).
const (
	keyUserID        = "userID"
	keyEmail         = "email"         // cache email (hindari GetUser tiap render)
	keyRole          = "role"          // otoritas tenant/platform (subject Casbin)
	keyBusinessRole  = "businessRole"  // peran CRM di workspace aktif (subject Casbin sumbu bisnis; "" = belum diberi)
	keyBusinessScope = "businessScope" // cakupan data (F3) role aktif: 'all'/'own'/'none'; "" = tak lihat desa (fail-closed)
	keyIsRoot        = "isRoot"        // super-admin env (immutable root)
	keyTenantID      = "tenantID"      // tenant user (multi-tenancy; 0 = platform/anonim)
	keyTenantName    = "tenantName"    // nama workspace (cache utk brand sidebar; bukan PII)
	keyTenantSlug    = "tenantSlug"    // slug workspace aktif — dipakai membentuk URL /w/{slug}
	keyAvatarURL     = "avatarURL"     // foto Google untuk nav
	keyPendingInvite = "pendingInvite" // token undangan yg menunggu login/register
	keyOAuthState    = "oauthState"    // anti-CSRF token flow OAuth
	keyOAuthNonce    = "oauthNonce"    // anti-replay id_token
	keyOAuthVerifier = "oauthVerifier" // PKCE code verifier
)

// mgr di-inject saat startup via Init. Accessor di bawah membungkusnya agar
// handler/middleware tidak pernah menyentuh string key langsung.
var mgr *scs.SessionManager

// Options = pengaturan session yang bergantung environment. Struct (bukan
// deretan bool) supaya penambahan berikutnya tak mengubah signature — dan
// supaya di titik pemanggilan terbaca apa artinya, bukan `NewManager(rc, true)`.
type Options struct {
	// Secure menandai cookie sesi hanya boleh dikirim lewat HTTPS. WAJIB true di
	// production: tanpa ini cookie ikut terkirim pada request HTTP polos, dan
	// siapa pun di jaringan yang sama bisa mencurinya.
	Secure bool

	// CookieName memisahkan sesi antar-deployment di host yang sama (mis. dua
	// aplikasi di sub-path domain yang sama akan saling menimpa sesi bila nama
	// cookie-nya sama). Kosong → default scs.
	CookieName string
}

// NewManager membuat SessionManager dengan store rueidis.
//
// Opsi keamanan DITURUNKAN dari environment oleh pemanggil (main), bukan dibaca
// sendiri di sini: paket ini tak boleh bergantung config, dan yang lebih penting
// — Secure tak boleh jadi env tersendiri yang bisa lupa diisi. Ia mengikuti
// ENV=production yang sudah pasti ada.
func NewManager(client rueidis.Client, opt Options) *scs.SessionManager {
	sm := scs.New()
	sm.Store = NewRueidisStore(client)
	sm.Lifetime = 24 * time.Hour
	sm.Cookie.HttpOnly = true
	// HTTPS-only di production. Konsekuensi yang DISENGAJA: bila production
	// ter-deploy tanpa HTTPS, login tak berfungsi sama sekali — gagal keras &
	// langsung terlihat, jauh lebih baik daripada sesi bocor diam-diam.
	sm.Cookie.Secure = opt.Secure
	if opt.CookieName != "" {
		sm.Cookie.Name = opt.CookieName
	}
	// Lax (BUKAN Strict): callback OAuth adalah navigasi top-level dari
	// accounts.google.com kembali ke sini = cross-site. Strict menahan cookie
	// pada request itu → state flow OAuth hilang → "state tidak valid". Lax
	// mengirim cookie pada top-level GET (kasus callback) tapi tetap anti-CSRF
	// untuk POST cross-site.
	sm.Cookie.SameSite = http.SameSiteLaxMode
	return sm
}

// Init menyimpan manager global agar accessor typed bisa dipakai.
func Init(sm *scs.SessionManager) { mgr = sm }

// UserID mengembalikan id user login, atau 0 bila belum login.
func UserID(ctx context.Context) int64 { return mgr.GetInt64(ctx, keyUserID) }

// SetUserID menandai user login (dipanggil setelah login sukses).
func SetUserID(ctx context.Context, id int64) { mgr.Put(ctx, keyUserID, id) }

// SetIdentity menyimpan identitas login lengkap ke session sekaligus: id, email,
// role EFEKTIF di workspace aktif (subject Casbin), isRoot (super-admin env),
// tenantID = WORKSPACE AKTIF, tenantName, dan avatar. Disimpan saat login agar
// handler/middleware/render tak perlu hit DB per-request.
//
// PENTING (model membership): tenantID bukan "tenant milik user" melainkan
// workspace yang sedang DIPILIH — satu user bisa anggota banyak workspace. Nilai
// ini TIDAK boleh dipercaya begitu saja: middleware Scope memvalidasinya terhadap
// tabel memberships sebelum membuka tx ber-tenant (anti tenant-forcing).
func SetIdentity(ctx context.Context, id int64, email, role string, isRoot bool, tenantID int64, tenantName, tenantSlug, avatarURL string) {
	mgr.Put(ctx, keyUserID, id)
	mgr.Put(ctx, keyEmail, email)
	mgr.Put(ctx, keyRole, role)
	mgr.Put(ctx, keyIsRoot, isRoot)
	mgr.Put(ctx, keyTenantID, tenantID)
	mgr.Put(ctx, keyTenantName, tenantName)
	mgr.Put(ctx, keyTenantSlug, tenantSlug)
	mgr.Put(ctx, keyAvatarURL, avatarURL)
}

// TenantID mengembalikan WORKSPACE AKTIF user login (0 bila belum login / belum
// punya workspace). Divalidasi terhadap memberships oleh middleware Scope.
func TenantID(ctx context.Context) int64 { return mgr.GetInt64(ctx, keyTenantID) }

// SetActiveTenant memindahkan workspace aktif (switcher sidebar / fallback Scope).
// Pemanggil WAJIB sudah memverifikasi keanggotaan user di tenant tsb.
//
// Sejak 0004 (workspace di path), session bukan lagi SUMBER KEBENARAN workspace
// aktif melainkan PETUNJUK: dipakai saat URL tak menyebut workspace (mis. "/",
// setelah login, /notifications) untuk memilih ke mana user diantar. Begitu ada
// slug di path, path yang menang.
func SetActiveTenant(ctx context.Context, tenantID int64, name, slug string) {
	mgr.Put(ctx, keyTenantID, tenantID)
	mgr.Put(ctx, keyTenantName, name)
	mgr.Put(ctx, keyTenantSlug, slug)
}

// TenantSlug mengembalikan slug workspace aktif ("" bila belum login/belum punya).
// Dipakai membentuk URL /w/{slug}/… lewat handler.wsPath.
func TenantSlug(ctx context.Context) string { return mgr.GetString(ctx, keyTenantSlug) }

// TenantName mengembalikan nama workspace user login (kosong bila belum login).
func TenantName(ctx context.Context) string { return mgr.GetString(ctx, keyTenantName) }

// SetTenantName memperbarui nama workspace di session (dipakai saat ganti nama
// agar brand sidebar langsung segar tanpa re-login).
func SetTenantName(ctx context.Context, name string) { mgr.Put(ctx, keyTenantName, name) }

// PendingInvite mengembalikan token undangan yang menunggu (user membuka link
// invite sebelum punya akun/login). "" bila tak ada.
func PendingInvite(ctx context.Context) string { return mgr.GetString(ctx, keyPendingInvite) }

// PutPendingInvite menyimpan token undangan agar bisa diterima otomatis setelah
// register/login selesai.
func PutPendingInvite(ctx context.Context, token string) { mgr.Put(ctx, keyPendingInvite, token) }

// ClearPendingInvite menghapus token undangan (setelah diterima / dibatalkan).
func ClearPendingInvite(ctx context.Context) { mgr.Remove(ctx, keyPendingInvite) }

// Email mengembalikan email user login (kosong bila belum login).
func Email(ctx context.Context) string { return mgr.GetString(ctx, keyEmail) }

// Role mengembalikan role user login (kosong bila belum login) — subject Casbin.
func Role(ctx context.Context) string { return mgr.GetString(ctx, keyRole) }

// BusinessRole mengembalikan peran CRM (admin/manager/sales/csm/support) user di
// WORKSPACE AKTIF — subject enforcer sumbu bisnis (authz.CanBusiness). "" bila
// belum diberi peran CRM di workspace ini; sengaja TEGAK LURUS terhadap Role:
// satu orang bisa owner tenant tapi bukan siapa-siapa di CRM, dan sebaliknya.
func BusinessRole(ctx context.Context) string { return mgr.GetString(ctx, keyBusinessRole) }

// SetBusinessRole menyimpan peran CRM workspace aktif ke session. Dipanggil
// RefreshIdentity tiap request (real-time, per-workspace: pindah workspace =
// peran CRM bisa berbeda). Kosongkan ("") saat user tak punya peran CRM di sana.
func SetBusinessRole(ctx context.Context, role string) { mgr.Put(ctx, keyBusinessRole, role) }

// BusinessDataScope mengembalikan cakupan data (F3) role CRM aktif: 'all'/'own'/
// 'none'. Menentukan DESA SIAPA yang terlihat (db.AccountsListFilterFor), tegak
// lurus terhadap BusinessRole (yang menentukan MENU apa yang boleh diakses).
// Dibaca dari kolom business_roles.data_scope, jadi role custom yang disunting
// operator berefek tanpa perubahan kode. "" (belum diberi peran / tak dikenal) →
// fail-closed: db.AccountsScopeFor memetakannya ke ScopeNone (nol baris).
func BusinessDataScope(ctx context.Context) string { return mgr.GetString(ctx, keyBusinessScope) }

// SetBusinessDataScope menyimpan cakupan data role aktif. Dipanggil RefreshIdentity
// tiap request bersama SetBusinessRole (real-time: menyunting data_scope role di
// panel berefek pada request berikutnya tanpa re-login). Kosongkan ("") saat user
// tak punya peran CRM di workspace ini.
func SetBusinessDataScope(ctx context.Context, scope string) { mgr.Put(ctx, keyBusinessScope, scope) }

// IsRoot melaporkan apakah user login adalah super-admin env (root immutable).
func IsRoot(ctx context.Context) bool { return mgr.GetBool(ctx, keyIsRoot) }

// AvatarURL mengembalikan URL avatar (base, tanpa suffix ukuran); "" bila tak ada.
func AvatarURL(ctx context.Context) string { return mgr.GetString(ctx, keyAvatarURL) }

// Clear menghancurkan session (logout). RenewToken juga dipakai saat login
// untuk mencegah session fixation.
func Clear(ctx context.Context) error { return mgr.Destroy(ctx) }

// Renew memutar token session (panggil setelah login sukses — anti fixation).
func Renew(ctx context.Context) error { return mgr.RenewToken(ctx) }

// PutOAuthFlow menyimpan state/nonce/verifier transient flow OAuth ke session
// (server-side via store scs), dipanggil sebelum redirect ke Google.
func PutOAuthFlow(ctx context.Context, state, nonce, verifier string) {
	mgr.Put(ctx, keyOAuthState, state)
	mgr.Put(ctx, keyOAuthNonce, nonce)
	mgr.Put(ctx, keyOAuthVerifier, verifier)
}

// OAuthFlow mengambil state/nonce/verifier yang tersimpan (mis. saat callback).
// Nilai kosong bila tak ada.
func OAuthFlow(ctx context.Context) (state, nonce, verifier string) {
	return mgr.GetString(ctx, keyOAuthState),
		mgr.GetString(ctx, keyOAuthNonce),
		mgr.GetString(ctx, keyOAuthVerifier)
}

// ClearOAuthFlow menghapus token flow OAuth (one-time use — panggil setelah
// callback membacanya, sukses maupun gagal).
func ClearOAuthFlow(ctx context.Context) {
	mgr.Remove(ctx, keyOAuthState)
	mgr.Remove(ctx, keyOAuthNonce)
	mgr.Remove(ctx, keyOAuthVerifier)
}

// WriteCookie meng-commit session ke store dan menulis Set-Cookie ke response
// SECARA MANUAL. WAJIB dipanggil SEBELUM membuka stream Datastar (NewSSE),
// karena NewSSE langsung flush header via http.ResponseController yang meng-Unwrap
// pembungkus scs — sehingga cookie dari LoadAndSave tidak akan pernah terkirim.
// Lihat catatan bug di STARTER §4.6.
func WriteCookie(ctx context.Context, w http.ResponseWriter) error {
	token, expiry, err := mgr.Commit(ctx)
	if err != nil {
		return err
	}
	mgr.WriteSessionCookie(ctx, w, token, expiry)
	return nil
}
