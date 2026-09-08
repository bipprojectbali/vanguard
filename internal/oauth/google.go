// Package oauth membungkus alur Google OAuth 2.0 / OIDC: authorization-code
// flow dengan PKCE, verifikasi id_token, dan ekstraksi claim.
//
// Hanya *flow* — session & penyimpanan user ditangani lapisan lain (scs, sqlc).
// Ini padanan idiomatik Go untuk "Login with Google" (bukan framework auth):
// x/oauth2 (flow + PKCE) + go-oidc (verifikasi id_token).
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// googleIssuer adalah issuer OIDC Google (dipakai discovery + verifikasi iss).
const googleIssuer = "https://accounts.google.com"

// Claims adalah subset claim id_token yang kita butuhkan untuk login.
type Claims struct {
	Sub           string // OIDC subject — id user Google yang immutable (kunci tautan)
	Email         string
	EmailVerified bool
	Picture       string // URL foto profil Google (claim "picture", scope profile)
	Name          string // nama tampilan (claim "name")
}

// Verifier memverifikasi raw id_token dan mengembalikan claim tepercaya.
// Diabstraksi jadi interface agar handler bisa di-test tanpa memanggil Google.
type Verifier interface {
	// VerifyIDToken memverifikasi signature + iss/aud, mencocokkan nonce, lalu
	// mengembalikan claim. Menolak bila email belum terverifikasi provider.
	VerifyIDToken(ctx context.Context, rawIDToken, wantNonce string) (*Claims, error)
}

// Provider menyatukan konfigurasi oauth2 + verifier OIDC untuk Google.
// Mengimplementasikan Verifier.
type Provider struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// New membangun Provider: discovery OIDC Google + konfigurasi oauth2.
// clientID/secret/redirectURL dari config (env). scope openid+email+profile.
func New(ctx context.Context, clientID, clientSecret, redirectURL string) (*Provider, error) {
	oidcProvider, err := oidc.NewProvider(ctx, googleIssuer)
	if err != nil {
		return nil, fmt.Errorf("oauth: discovery Google OIDC: %w", err)
	}
	return &Provider{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: oidcProvider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// AuthURL membangun URL consent Google dengan proteksi lengkap:
//   - state: anti-CSRF (dicek constant-time di callback)
//   - nonce: anti-replay id_token
//   - PKCE S256: anti authorization-code injection
//   - prompt=select_account: SELALU tampilkan pemilih akun Google. Tanpa ini,
//     bila browser masih punya SATU sesi Google aktif, Google memilih akun itu
//     diam-diam dan melewati layar pilih akun — sehingga setelah logout dari
//     aplikasi (yang hanya menghapus sesi app, bukan sesi Google), user tak
//     bisa memilih/berganti akun. Sesi Google terpisah dari sesi aplikasi.
func (p *Provider) AuthURL(state, nonce, verifier string) string {
	return p.oauth.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

// Exchange menukar authorization code jadi token, membuktikan kepemilikan PKCE
// verifier. Mengembalikan raw id_token dari field extra.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (string, error) {
	tok, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("oauth: exchange code: %w", err)
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", fmt.Errorf("oauth: id_token tidak ada di respons token")
	}
	return rawIDToken, nil
}

// VerifyIDToken mengimplementasikan Verifier: verifikasi kriptografis id_token,
// cek nonce MANUAL (verifier.Verify TIDAK cek nonce), lalu tolak bila email
// belum terverifikasi provider (email_verified=false → kelas nOAuth).
func (p *Provider) VerifyIDToken(ctx context.Context, rawIDToken, wantNonce string) (*Claims, error) {
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oauth: verifikasi id_token: %w", err)
	}
	// verifier.Verify TIDAK memeriksa nonce — wajib manual (jebakan paling umum).
	if idToken.Nonce != wantNonce {
		return nil, fmt.Errorf("oauth: nonce tidak cocok")
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Picture       string `json:"picture"` // boleh kosong walau scope diminta
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oauth: baca claim: %w", err)
	}
	if claims.Email == "" {
		return nil, fmt.Errorf("oauth: id_token tanpa email (scope email hilang?)")
	}
	if !claims.EmailVerified {
		return nil, fmt.Errorf("oauth: email belum terverifikasi provider")
	}
	return &Claims{
		Sub:           idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Picture:       claims.Picture,
		Name:          claims.Name,
	}, nil
}

// NewState menghasilkan token acak (state / nonce) 32 byte hex dari crypto/rand.
func NewState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: generate random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewVerifier menghasilkan PKCE code verifier (native x/oauth2).
func NewVerifier() string { return oauth2.GenerateVerifier() }

// avatarSizeRe mencocokkan suffix ukuran Google (mis. "=s96-c") agar bisa
// dinormalisasi ke base — ukuran final ditentukan saat render (sizedAvatar).
var avatarSizeRe = regexp.MustCompile(`=s\d+(-c)?$`)

// NormalizeAvatarURL membersihkan suffix ukuran dari URL avatar Google dan
// mengembalikan pointer (nil bila kosong) agar cocok kolom nullable sqlc.
func NormalizeAvatarURL(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	base := avatarSizeRe.ReplaceAllString(raw, "")
	return &base
}

// maxDisplayName membatasi panjang nama tampilan yang kita simpan. Google tak
// menjanjikan batas apa pun untuk claim `name`, dan nilainya diketik user —
// tanpa batas, satu orang bisa mengirim nama sepanjang megabyte yang lalu kita
// simpan dan render di daftar anggota setiap orang lain. Ambang ini longgar
// untuk nama sungguhan (termasuk nama panjang non-Latin) sekaligus menutup
// penyalahgunaan.
const maxDisplayName = 100

// nameCtlRe mencocokkan karakter kontrol & pemisah baris. Nama satu baris yang
// menyelipkan newline merusak tata letak tabel dan, lebih buruk, bisa dipakai
// menyamarkan teks tambahan seolah baris terpisah.
var nameCtlRe = regexp.MustCompile(`[\x00-\x1f\x7f\p{Cf}]+`)

// NormalizeDisplayName membersihkan claim `name` dari Google dan mengembalikan
// pointer (nil bila tak tersisa apa pun) agar cocok kolom nullable sqlc.
//
// Nilai ini USER-CONTROLLED: siapa pun bebas menyetel namanya di akun Google
// menjadi apa saja. Tiga konsekuensi yang ditutup di sini:
//
//   - karakter kontrol/format tak-terlihat dibuang — ia tak menambah makna dan
//     bisa dipakai memalsukan tampilan (mis. override arah teks);
//   - runtun spasi diciutkan jadi satu, agar nama tak bisa "menggeser" kolom;
//   - panjang dipotong pada batas rune, bukan byte — memotong di tengah rune
//     UTF-8 menghasilkan byte rusak yang berakhir sebagai "?" di layar.
//
// Yang TIDAK dilakukan di sini: menolak nama yang menyerupai alamat email atau
// nama orang lain. Itu mustahil dinilai, dan pertahanannya ada di tempat lain —
// nama tak pernah dipakai untuk identitas maupun otorisasi (id-lah yang dipakai),
// dan seluruh keluarannya di-escape oleh view.
func NormalizeDisplayName(raw string) *string {
	clean := strings.TrimSpace(nameCtlRe.ReplaceAllString(raw, " "))
	clean = strings.Join(strings.Fields(clean), " ")
	if clean == "" {
		return nil
	}
	if r := []rune(clean); len(r) > maxDisplayName {
		clean = strings.TrimSpace(string(r[:maxDisplayName]))
	}
	return &clean
}
