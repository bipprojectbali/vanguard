package handler

import (
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/desaplus"
)

// customer_success_sync_map_test.go — fungsi murni mapDesaPlusSummary/
// formatTopFeatures (customer_success_sync_map.go), dipisah dari
// customer_success_sync_test.go (gerbang + integrasi handler) agar tiap file
// tetap di bawah ambang Test (400 baris, CLAUDE.md §8) — dua concern berbeda:
// pemetaan data murni (tanpa DB) vs. alur HTTP+DB penuh.

func TestMapDesaPlusSummary(t *testing.T) {
	existingFeatures := "lama (1x)"
	existing := db.CustomerSuccess{
		KeyFeaturesUsed: &existingFeatures,
	}

	t.Run("lastActivity nil → last_login_date fallback ke existing (kosong)", func(t *testing.T) {
		sum := &desaplus.VillageSummary{LastActivity: nil, ActiveUsers: 9}
		lastLogin, activeUsers, keyFeatures := mapDesaPlusSummary(existing, sum)
		if lastLogin.Valid {
			t.Errorf("last_login_date harus fallback existing (invalid), got %v", lastLogin)
		}
		if activeUsers == nil || *activeUsers != 9 {
			t.Errorf("active_users harus 9 (tak pernah fallback), got %v", activeUsers)
		}
		if keyFeatures == nil || *keyFeatures != existingFeatures {
			t.Errorf("topFeatures kosong → key_features_used fallback existing, got %v", keyFeatures)
		}
	})

	t.Run("activeUsers 0 → nilai VALID, bukan data hilang", func(t *testing.T) {
		sum := &desaplus.VillageSummary{ActiveUsers: 0}
		_, activeUsers, _ := mapDesaPlusSummary(existing, sum)
		if activeUsers == nil || *activeUsers != 0 {
			t.Errorf("active_users=0 harus tetap 0 (bukan nil), got %v", activeUsers)
		}
	})

	t.Run("lastActivity format tak dikenal → fallback existing", func(t *testing.T) {
		sum := &desaplus.VillageSummary{LastActivity: &desaplus.LastActivity{Timestamp: "bukan format tanggal"}}
		lastLogin, _, _ := mapDesaPlusSummary(existing, sum)
		if lastLogin.Valid {
			t.Errorf("timestamp tak dikenal harus fallback existing (invalid), got %v", lastLogin)
		}
	})

	t.Run("topFeatures ada → menimpa existing", func(t *testing.T) {
		sum := &desaplus.VillageSummary{TopFeatures: []desaplus.TopFeature{{Feature: "baru", Count: 3}}}
		_, _, keyFeatures := mapDesaPlusSummary(existing, sum)
		if keyFeatures == nil || *keyFeatures != "baru (3x)" {
			t.Errorf("topFeatures ada harus menimpa existing, got %v", keyFeatures)
		}
	})
}

func TestFormatTopFeatures(t *testing.T) {
	cases := []struct {
		name string
		in   []desaplus.TopFeature
		want string
	}{
		{"kosong", nil, ""},
		{"satu", []desaplus.TopFeature{{Feature: "presensi", Count: 12}}, "presensi (12x)"},
		{"banyak", []desaplus.TopFeature{{Feature: "presensi", Count: 12}, {Feature: "surat", Count: 8}}, "presensi (12x), surat (8x)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatTopFeatures(c.in); got != c.want {
				t.Errorf("formatTopFeatures(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
