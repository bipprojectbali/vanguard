package preflight

import (
	"strings"
	"testing"
)

// preflight_test.go — yang diuji di sini adalah KEGUNAAN pesannya, bukan
// sekadar "fungsi mengembalikan sesuatu".
//
// Paket ini tak menghasilkan perilaku aplikasi apa pun; satu-satunya alasannya
// ada adalah supaya orang yang boot-nya gagal tahu harus berbuat apa. Jadi
// yang harus dijaga: saran yang muncul benar-benar menunjuk kekeliruan yang
// sebenarnya, dan perintah perbaikannya bisa langsung ditempel.

// TestSimilar_MenangkapSalahKetikNyata: pola yang BENAR-BENAR terjadi saat
// meng-clone template ini — pemisah kata tertukar, akhiran _test tertinggal,
// nama lama sebelum rename. Inilah yang membuat "database tak ada" berubah jadi
// "oh, saya salah ketik" tanpa perlu membuka psql.
func TestSimilar_MenangkapSalahKetikNyata(t *testing.T) {
	cases := []struct{ want, got string }{
		{"vanguard_matrix", "vanguard-matrix"},      // underscore vs hyphen — kasus nyata
		{"vanguard-matrix", "vanguard_matrix"},      // arah sebaliknya
		{"vanguard-matrix", "vanguard-matrix_test"}, // akhiran _test tertinggal
		{"myapp", "myapp_test"},
		{"myapp_dev", "myapp"}, // nama lama sebelum rename
	}
	for _, c := range cases {
		if !similar(c.want, c.got) {
			t.Errorf("similar(%q, %q) = false — salah ketik ini harus disarankan", c.want, c.got)
		}
	}
}

// TestSimilar_TakMenyarankanYangTakBerhubungan: saran yang keliru lebih buruk
// daripada tak ada saran — ia mengirim orang menyelidiki database yang tak ada
// hubungannya, dan mereka akan berhenti mempercayai pesan berikutnya.
func TestSimilar_TakMenyarankanYangTakBerhubungan(t *testing.T) {
	for _, c := range []struct{ want, got string }{
		{"vanguard", "postgres"},
		{"myapp", "template1"},
		{"toko", "gudang"},
		{"myapp", "myapp"}, // yang sama persis BUKAN "mirip" — ia yang dicari
	} {
		if similar(c.want, c.got) {
			t.Errorf("similar(%q, %q) = true — saran yang tak berhubungan menyesatkan", c.want, c.got)
		}
	}
}

// TestReport_MengumpulkanSemua: memperbaiki satu masalah lalu menemukan
// berikutnya berarti menjalankan perintah yang sama berkali-kali untuk
// pertanyaan yang sama — "apa saja yang kurang?".
func TestReport_MengumpulkanSemua(t *testing.T) {
	r := &Report{}
	if !r.OK() {
		t.Error("laporan kosong harus OK")
	}
	r.Add(EnvMissing())
	r.Add(NameMismatch("proyek-baru", "proyek_baru"))
	if r.OK() {
		t.Error("laporan berisi masalah tak boleh OK")
	}
	if len(r.Problems) != 2 {
		t.Errorf("semua masalah harus terkumpul, got %d", len(r.Problems))
	}
}

// TestReport_SetiapMasalahPunyaPerbaikan: "periksa konfigurasi Anda" bukan
// petunjuk. Tiap temuan wajib membawa perintah yang bisa langsung ditempel —
// itu seluruh alasan paket ini ada.
func TestReport_SetiapMasalahPunyaPerbaikan(t *testing.T) {
	target := DBTarget{DBName: "toko", Host: "localhost", Port: "5432"}
	for _, p := range []Problem{
		EnvMissing(),
		DatabaseMissing(target, nil),
		PostgresDown(target, errFake("connection refused")),
		RedisDown("localhost:6379", errFake("connection refused")),
		NameMismatch("toko-online", "toko_online"),
	} {
		if p.What == "" {
			t.Error("tiap masalah harus menyebut apa yang salah")
		}
		if len(p.Fix) == 0 {
			t.Errorf("masalah %q tanpa langkah perbaikan — itu keluhan, bukan petunjuk", p.What)
		}
	}
}

// TestDatabaseMissing_MenyebutYangMirip: saat ada database mirip, pesannya
// WAJIB menyebutkannya — di situlah jawabannya hampir selalu berada.
func TestDatabaseMissing_MenyebutYangMirip(t *testing.T) {
	target := DBTarget{DBName: "vanguard_matrix", Host: "localhost", Port: "5432"}

	p := DatabaseMissing(target, []string{"vanguard-matrix_test"})
	if !strings.Contains(p.Why, "vanguard-matrix_test") {
		t.Errorf("nama mirip harus disebut agar salah ketik terlihat: %q", p.Why)
	}
	// Perintahnya tetap memakai nama yang DIMINTA, bukan yang mirip: menebak
	// niat orang lalu menyodorkan createdb untuk nama lain akan membuat mereka
	// membuat database ketiga yang juga salah.
	if !strings.Contains(strings.Join(p.Fix, " "), "createdb vanguard_matrix") {
		t.Errorf("perintah harus memakai nama yang diminta: %v", p.Fix)
	}

	// Tanpa yang mirip, tak ada Why yang mengarang-ngarang.
	if q := DatabaseMissing(target, nil); strings.Contains(q.Why, "mirip") {
		t.Errorf("tanpa kandidat mirip, jangan menyinggungnya: %q", q.Why)
	}
}

// TestReport_TerbacaDiTerminal: laporannya dipakai manusia yang sedang
// terhambat, jadi bentuknya bagian dari fungsinya.
func TestReport_TerbacaDiTerminal(t *testing.T) {
	r := &Report{}
	r.Add(EnvMissing())
	out := r.String()

	if !strings.Contains(out, "cp .env.example .env") {
		t.Errorf("perintah perbaikan harus tampil:\n%s", out)
	}
	if !strings.Contains(out, "$ ") {
		t.Errorf("perintah harus ditandai agar bisa dibedakan dari penjelasan:\n%s", out)
	}
	if ok := (&Report{}).String(); !strings.Contains(ok, "siap") {
		t.Errorf("laporan bersih harus menyatakan lingkungannya siap, got %q", ok)
	}
}

// TestAdminDSN_MempertahankanServer: DSN admin dipakai membuat database &
// mencari yang mirip. Ia harus menunjuk SERVER yang sama — kalau host/port ikut
// berubah, database dibuat di tempat yang salah dan saran diambil dari server
// yang tak ada hubungannya.
func TestAdminDSN_MempertahankanServer(t *testing.T) {
	got := adminDSN("postgres://budi@db.internal:6432/toko?sslmode=disable")
	for _, want := range []string{"budi", "db.internal", "6432", "/postgres"} {
		if !strings.Contains(got, want) {
			t.Errorf("adminDSN kehilangan %q: %s", want, got)
		}
	}
	if strings.Contains(got, "/toko") {
		t.Errorf("adminDSN harus menunjuk database 'postgres', bukan target: %s", got)
	}
}

// TestParseDSN_RusakDitolakDenganJelas: DSN tak terurai harus jadi pesan yang
// menyebut DATABASE_URL, bukan istilah pustaka yang tak dikenali penerimanya.
func TestParseDSN_RusakDitolakDenganJelas(t *testing.T) {
	_, err := ParseDSN("ini bukan dsn sama sekali")
	if err == nil {
		t.Fatal("DSN rusak harus ditolak")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("pesan harus menyebut env yang salah agar orang tahu di mana memperbaikinya: %v", err)
	}
}

// TestRun_FromFileFalse_TakLaporEnvHilang: BUG yang ditemukan saat review MCP.
// Run dulu SELALU os.Stat(".env"); dari dalam container (env dari lingkungan,
// bukan file) itu selalu gagal dan melapor ".env belum ada" untuk keadaan yang
// justru normal — diagnosis palsu yang menyesatkan.
//
// Dengan FromFile:false, pemeriksaan file dilewati. DSN sengaja dibuat rusak
// supaya Run berhenti CEPAT (tak menyentuh Postgres nyata di unit test) — yang
// diuji cukup: laporannya BUKAN "EnvMissing".
func TestRun_FromFileFalse_TakLaporEnvHilang(t *testing.T) {
	rep := Run(t.Context(), Opts{
		DatabaseURL: "dsn-sengaja-rusak",
		FromFile:    false, // runtime container: .env memang tak ada
	})
	want := EnvMissing().What
	for _, p := range rep.Problems {
		if p.What == want {
			t.Error("FromFile:false tak boleh melapor .env hilang — env datang dari lingkungan, bukan file")
		}
	}
}

// TestRun_FromFileTrue_MasihLaporEnvHilang: jalur `make doctor` (setup dari
// repo) TETAP harus melapor .env hilang — di sana ketiadaannya memang masalah
// pertama. Dijalankan di t.TempDir() yang pasti tanpa .env.
func TestRun_FromFileTrue_MasihLaporEnvHilang(t *testing.T) {
	t.Chdir(t.TempDir()) // dir kosong → .env pasti tak ada
	rep := Run(t.Context(), Opts{DatabaseURL: "apa-saja", FromFile: true})
	want := EnvMissing().What
	found := false
	for _, p := range rep.Problems {
		if p.What == want {
			found = true
		}
	}
	if !found {
		t.Error("FromFile:true di dir tanpa .env harus melapor .env hilang (jalur make doctor)")
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }
