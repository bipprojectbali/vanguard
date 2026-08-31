package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// config_env.go — pembaca env primitif (getEnv/getEnvInt/mustEnv) & pemecah
// daftar email (parseEmailList). Dipisah agar file di bawah ambang tipe Config
// (100). Semua env dibaca HANYA di paket config.

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
