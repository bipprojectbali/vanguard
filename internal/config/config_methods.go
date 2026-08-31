package config

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// config_methods.go — accessor turunan atas Config (nama cookie sesi, lokasi
// timezone, cek super-admin/Google/production). Dipisah dari struct di config.go
// agar tiap file di bawah ambang tipe Config (100). Semua env tetap dibaca HANYA
// di paket config.

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
