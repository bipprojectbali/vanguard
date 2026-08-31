package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// config_dotenv.go — parser .env minimal tanpa dependency (LoadDotEnv) &
// pelepas kutip pembungkus (unquote). Dipisah dari pembaca env di config_env.go
// agar file di bawah ambang tipe Config (100). Env dibaca HANYA di paket config.

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
