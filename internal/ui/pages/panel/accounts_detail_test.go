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

	// Tipe Akun di header kini badge TERISI (badge-neutral, mewarisi gaya kode
	// sistem), bukan ghost. Cek span eksplisitnya (badge-ghost masih sah dipakai
	// kartu lain seperti status langganan, jadi jangan cek ghost global).
	if !strings.Contains(out, `<span class="badge badge-neutral">Pelanggan</span>`) {
		t.Errorf("Tipe Akun harus badge terisi (badge-neutral), bukan ghost:\n%s", out)
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

// TestAccountDetail_PairedSectionsEqualHeight (BL-62): kartu detail & ringkasan
// dirender dalam SATU grid 2-kolom ber-items-stretch (bukan dua stack tertumpuk
// independen) dan berurutan selang-seling, sehingga tiap pasangan berbagi baris
// grid → grid-item meregang setinggi pasangan tertinggi (tepi bawah rata).
// Regresi: kembali ke dua stack membuat urutan mengelompok kiri-lalu-kanan &
// menghapus items-stretch.
func TestAccountDetail_PairedSectionsEqualHeight(t *testing.T) {
	v := AccountDetailView{
		Base:        "/w/desa",
		ID:          3,
		VillageName: "Desa Sejajar",
		VillageCode: "10.01.01.2001",
		AccountType: "Pelanggan",
	}
	out := renderAccountDetail(t, v)

	// items-stretch menegaskan pasangan sebaris meregang setinggi yang tertinggi.
	if !strings.Contains(out, "items-stretch") {
		t.Errorf("grid detail harus items-stretch agar pasangan setinggi:\n%s", out)
	}

	// Urutan selang-seling: tiap ringkasan (kanan) jatuh SETELAH kartu detail
	// pasangannya (kiri) dan SEBELUM kartu kiri berikutnya — bukti keduanya
	// sebaris. Di layout lama semua kartu kiri mendahului seluruh ringkasan.
	order := []string{
		"Identitas", "Ringkasan Langganan",
		"Wilayah", "Ringkasan Customer Success",
		"Profil Desa", "Sistem",
	}
	prev := -1
	for _, title := range order {
		i := strings.Index(out, title)
		if i < 0 {
			t.Fatalf("judul kartu %q tak ditemukan:\n%s", title, out)
		}
		if i <= prev {
			t.Errorf("urutan kartu salah: %q (idx %d) harus setelah judul sebelumnya (idx %d) — pasangan tak sebaris:\n%s", title, i, prev, out)
		}
		prev = i
	}
}

// TestAccountDetail_ContactAndCSButtonsAlwaysVisible (BL-176): Kontak &
// Customer Success disejajarkan ke baris header bersama Sunting/Hapus, tapi
// SENGAJA tanpa gate CanWrite (tautan baca) — beda dgn Sunting/Hapus yang
// gated tulis. Regresi: keduanya ikut hilang saat CanWrite=false.
func TestAccountDetail_ContactAndCSButtonsAlwaysVisible(t *testing.T) {
	base := AccountDetailView{
		Base:        "/w/desa",
		ID:          5,
		VillageName: "Desa Tombol",
		AccountType: "Pelanggan",
	}

	readOnly := base
	readOnly.CanWrite = false
	out := renderAccountDetail(t, readOnly)
	for _, want := range []string{">Kontak<", ">Customer Success<"} {
		if !strings.Contains(out, want) {
			t.Errorf("tombol %q harus tetap tampil walau CanWrite=false:\n%s", want, out)
		}
	}
	for _, notWant := range []string{">Sunting<", ">Hapus<"} {
		if strings.Contains(out, notWant) {
			t.Errorf("tombol %q tak boleh tampil saat CanWrite=false:\n%s", notWant, out)
		}
	}

	writable := base
	writable.CanWrite = true
	out = renderAccountDetail(t, writable)
	for _, want := range []string{">Kontak<", ">Customer Success<", ">Sunting<", ">Hapus<"} {
		if !strings.Contains(out, want) {
			t.Errorf("tombol %q harus tampil saat CanWrite=true:\n%s", want, out)
		}
	}
}

// TestAccountDetail_ActionButtonsBordered (BL-176): Sunting, Kontak, Customer
// Success kini berbatas (btn-outline) — sebelumnya Kontak/CS btn-ghost
// (tanpa batas) & Sunting tanpa modifier. Hapus sudah btn-outline sejak
// semula (destruktif, btn-error btn-outline) — dihitung terpisah dari 3
// lainnya krn beda warna semantik.
func TestAccountDetail_ActionButtonsBordered(t *testing.T) {
	v := AccountDetailView{
		Base:        "/w/desa",
		ID:          6,
		VillageName: "Desa Border",
		AccountType: "Pelanggan",
		CanWrite:    true,
	}
	out := renderAccountDetail(t, v)

	if strings.Contains(out, "btn-ghost") {
		t.Errorf("Kontak/Customer Success tak boleh lagi btn-ghost (kini btn-outline):\n%s", out)
	}
	// 4 tombol aksi berbatas: Kontak, Customer Success, Sunting (btn-outline
	// polos) + Hapus (btn-error btn-outline, ikut mengandung substring "btn-outline").
	if got := strings.Count(out, "btn-outline"); got != 4 {
		t.Errorf("harus ada 4 tombol btn-outline (Kontak/CS/Sunting/Hapus), got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "btn-error btn-outline") {
		t.Errorf("tombol Hapus harus tetap btn-error btn-outline:\n%s", out)
	}
}
