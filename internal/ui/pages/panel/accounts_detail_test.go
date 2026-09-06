package panel

import (
	"strings"
	"testing"
)

// accounts_detail_test.go — regresi BL-61: badge Kode Sistem (entity_code)
// dilepas dari detail Desa. Yang tampil = "Kode Desa (Kemendagri)"
// (village_code); entity_code tetap dibuat & tersimpan di backend, hanya tak
// lagi dirender di UI.

func renderAccountDetail(t *testing.T, v AccountDetailView) string {
	t.Helper()
	var sb strings.Builder
	if err := AccountDetail(v).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// TestAccountDetail_DropsSystemCodeBadge: detail TAK boleh merender nilai
// entity_code (badge kode sistem) — meski struct field-nya sudah tiada, kunci
// via nilai unik agar regresi ketahuan bila badge dikembalikan.
func TestAccountDetail_DropsSystemCodeBadge(t *testing.T) {
	v := AccountDetailView{
		Base:        "/w/desa",
		ID:          7,
		VillageName: "Desa Uji",
		VillageCode: "32.01.01.2001", // Kode Kemendagri — HARUS tampil
		AccountType: "Pelanggan",
	}
	out := renderAccountDetail(t, v)

	// Kode Kemendagri tetap ada (label + nilai).
	for _, want := range []string{"Kode Desa (Kemendagri)", "32.01.01.2001"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail harus tetap menampilkan %q:\n%s", want, out)
		}
	}
	// Badge kode sistem font-mono tak boleh muncul lagi.
	if strings.Contains(out, "badge badge-neutral font-mono") {
		t.Errorf("badge Kode Sistem (entity_code) harus dihapus dari detail (BL-61):\n%s", out)
	}
}

// TestAccountDetail_SystemCodeValueNotLeaked: kalaupun ada nilai kode sistem
// yang tak sengaja masuk (mis. lewat field lain), ia tak boleh bocor ke output.
// Di sini VillageCode diisi kode Kemendagri; nilai bergaya "DESA-XXX" (pola
// entity_code) tak boleh ada di mana pun.
func TestAccountDetail_SystemCodeValueNotLeaked(t *testing.T) {
	v := AccountDetailView{
		Base:        "/w/desa",
		ID:          9,
		VillageName: "Desa Bocor",
		VillageCode: "11.22.33.4444",
		AccountType: "Prospek",
	}
	out := renderAccountDetail(t, v)
	if strings.Contains(out, "DESA-") {
		t.Errorf("nilai kode sistem (pola DESA-XXX) tak boleh terrender di detail:\n%s", out)
	}
}
