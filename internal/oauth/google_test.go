package oauth

import (
	"net/url"
	"regexp"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func TestNormalizeAvatarURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *string // nil = expect nil
	}{
		{"kosong", "", nil},
		{"spasi saja", "   ", nil},
		{"strip =s96-c", "https://lh3.googleusercontent.com/a/abc=s96-c", ptr("https://lh3.googleusercontent.com/a/abc")},
		{"strip =s256 tanpa -c", "https://lh3.googleusercontent.com/a/x=s256", ptr("https://lh3.googleusercontent.com/a/x")},
		{"tanpa suffix ukuran", "https://lh3.googleusercontent.com/a/plain", ptr("https://lh3.googleusercontent.com/a/plain")},
		{"suffix di tengah tak di-strip (anchored)", "https://x/=s96-c/y", ptr("https://x/=s96-c/y")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeAvatarURL(c.in)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("want nil, got %q", *got)
			case c.want != nil && got == nil:
				t.Errorf("want %q, got nil", *c.want)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("got %q, want %q", *got, *c.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestNewState(t *testing.T) {
	s, err := NewState()
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	// 32 byte → 64 char hex.
	if len(s) != 64 {
		t.Errorf("state harus 64 char hex, got %d", len(s))
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s) {
		t.Errorf("state harus hex lowercase: %q", s)
	}
	// Entropi: dua panggilan berbeda.
	s2, _ := NewState()
	if s == s2 {
		t.Error("dua NewState harus berbeda (entropi)")
	}
}

func TestNewVerifier(t *testing.T) {
	v := NewVerifier()
	if v == "" {
		t.Fatal("verifier tak boleh kosong")
	}
	// PKCE spec: 43–128 char.
	if len(v) < 43 || len(v) > 128 {
		t.Errorf("verifier di luar rentang PKCE (43-128): %d", len(v))
	}
	if v == NewVerifier() {
		t.Error("dua verifier harus berbeda")
	}
}

// TestAuthURL_PromptSelectAccount mengunci perbaikan: URL consent SELALU
// menyertakan prompt=select_account supaya Google menampilkan pemilih akun
// alih-alih memilih diam-diam sesi Google tunggal yang masih aktif. Construct
// Provider langsung dgn oauth2.Config buatan tangan — AuthCodeURL murni, tak
// butuh discovery jaringan.
func TestAuthURL_PromptSelectAccount(t *testing.T) {
	p := &Provider{oauth: &oauth2.Config{
		ClientID:    "test-client",
		RedirectURL: "https://app.example/callback",
		Endpoint:    google.Endpoint,
		Scopes:      []string{oidc.ScopeOpenID, "email", "profile"},
	}}

	raw := p.AuthURL("state123", "nonce456", NewVerifier())
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("URL tak valid: %v", err)
	}
	if got := u.Query().Get("prompt"); got != "select_account" {
		t.Fatalf("prompt = %q, mau %q", got, "select_account")
	}
	// Proteksi lain harus tetap ada (regresi guard).
	if u.Query().Get("state") != "state123" {
		t.Errorf("state hilang: %q", u.Query().Get("state"))
	}
	if u.Query().Get("nonce") != "nonce456" {
		t.Errorf("nonce hilang: %q", u.Query().Get("nonce"))
	}
	if u.Query().Get("code_challenge") == "" || u.Query().Get("code_challenge_method") != "S256" {
		t.Errorf("PKCE S256 hilang: challenge=%q method=%q",
			u.Query().Get("code_challenge"), u.Query().Get("code_challenge_method"))
	}
}
