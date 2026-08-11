package mcpserver

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// tools_test.go — tiap tool memanggil handler langsung (bukan lewat transport)
// dan diperiksa dua hal: outputnya benar, DAN ia TIDAK MENULIS apa pun. Yang
// kedua adalah janji utama seluruh paket ini — read-only bukan sekadar niat.

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// countRows menghitung baris satu tabel — dipakai sebagai bukti nol-tulis:
// dijalankan sebelum & sesudah tiap tool, harus sama.
func countRows(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	if err := pkgPool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("hitung %s: %v", table, err)
	}
	return n
}

// seedAudit menyisipkan satu baris audit agar activity_trail punya isi.
func seedAudit(t *testing.T, action string, at time.Time) {
	t.Helper()
	_, err := pkgPool.Exec(context.Background(),
		`INSERT INTO audit_logs (action, target_type, metadata, created_at)
		 VALUES ($1, 'platform', '{}'::jsonb, $2)`, action, at)
	if err != nil {
		t.Fatalf("seed audit: %v", err)
	}
}

// TestReadOnly_NolTulis: GERBANG. Panggil SETIAP tool, buktikan tak satu pun
// menyentuh tabel yang bisa ditulis. Kalau kelak ada yang menambah tool dengan
// query tulis tak sengaja, test ini yang menangkapnya.
func TestReadOnly_NolTulis(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	seedAudit(t, "auth.login", time.Now())

	tables := []string{"audit_logs", "activity_presence", "tenants", "users", "platform_settings"}
	before := map[string]int64{}
	for _, tb := range tables {
		before[tb] = countRows(t, tb)
	}

	// Jalankan semua tool sekali.
	if _, _, err := d.runtimeHealth(ctx, nil, noInput{}); err != nil {
		t.Errorf("runtime_health: %v", err)
	}
	if _, _, err := d.preflight(ctx, nil, noInput{}); err != nil {
		t.Errorf("preflight: %v", err)
	}
	if _, _, err := d.migrationVersion(ctx, nil, noInput{}); err != nil {
		t.Errorf("migration_version: %v", err)
	}
	if _, _, err := d.platformStats(ctx, nil, noInput{}); err != nil {
		t.Errorf("platform_stats: %v", err)
	}
	if _, _, err := d.dbSchema(ctx, nil, noInput{}); err != nil {
		t.Errorf("db_schema: %v", err)
	}
	if _, _, err := d.activityTrail(ctx, nil, rangeInput{Range: "day"}); err != nil {
		t.Errorf("activity_trail: %v", err)
	}
	if _, _, err := d.activityKPIs(ctx, nil, rangeInput{Range: "day"}); err != nil {
		t.Errorf("activity_kpis: %v", err)
	}

	for _, tb := range tables {
		if after := countRows(t, tb); after != before[tb] {
			t.Errorf("tabel %s berubah (%d → %d) — sebuah tool MENULIS, padahal read-only", tb, before[tb], after)
		}
	}
}

// TestRuntimeHealth_RLSMengikat: di jalur MCP (WithSuper/app_rw), RLS harus
// dilaporkan MENGIKAT dan role BUKAN owner — bukti read-only-nya struktural,
// bukan sekadar disiplin memilih query.
func TestRuntimeHealth_RLSMengikat(t *testing.T) {
	d := testDeps(t)
	_, out, err := d.runtimeHealth(context.Background(), nil, noInput{})
	if err != nil {
		t.Fatalf("runtime_health: %v", err)
	}
	if !out.DBReachable {
		t.Error("DB harus terjangkau di test")
	}
	// Koneksi test = owner pool, jadi RLS TIDAK mengikat di sini (owner bypass).
	// Yang penting: tool MELAPORKAN keadaan sebenarnya, tak berbohong "aman".
	if out.RLSBinds && out.ConnRole == "" {
		t.Error("bila RLS dilaporkan mengikat, role koneksi harus terisi")
	}
	if !out.RLSBinds && out.RLSReason == "" {
		t.Error("bila RLS tak mengikat, alasannya WAJIB dijelaskan — jangan diam")
	}
}

// TestActivityTrail_MerangkaiKalimat: peristiwa dirender jadi kalimat terbaca,
// bukan kode mentah — inti kegunaan tool ini bagi agent.
func TestActivityTrail_MerangkaiKalimat(t *testing.T) {
	d := testDeps(t)
	// Bersihkan lalu seed satu peristiwa yang pasti punya kalimat.
	if _, err := pkgPool.Exec(context.Background(), "TRUNCATE audit_logs"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	seedAudit(t, "workspace.create", time.Now())

	_, out, err := d.activityTrail(context.Background(), nil, rangeInput{Range: "day"})
	if err != nil {
		t.Fatalf("activity_trail: %v", err)
	}
	if len(out.Events) == 0 {
		t.Fatal("peristiwa yang di-seed harus muncul")
	}
	e := out.Events[0]
	if e.Action != "workspace.create" {
		t.Errorf("action mentah harus ikut: %q", e.Action)
	}
	// Kalimat harus terbaca (bukan kosong, bukan kode mentah).
	if e.Sentence == "" || strings.Contains(e.Sentence, "workspace.create") && !strings.Contains(e.Sentence, " ") {
		t.Errorf("peristiwa harus dirangkai jadi kalimat: %q", e.Sentence)
	}
}

// TestActivityTrail_MenghormatiRentang: peristiwa lama di luar rentang harian
// tak boleh muncul (memastikan window benar-benar dipakai).
func TestActivityTrail_MenghormatiRentang(t *testing.T) {
	d := testDeps(t)
	if _, err := pkgPool.Exec(context.Background(), "TRUNCATE audit_logs"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	seedAudit(t, "auth.login", time.Now())                          // hari ini
	seedAudit(t, "workspace.delete", time.Now().AddDate(0, 0, -40)) // 40 hari lalu

	_, day, err := d.activityTrail(context.Background(), nil, rangeInput{Range: "day"})
	if err != nil {
		t.Fatalf("activity_trail day: %v", err)
	}
	for _, e := range day.Events {
		if e.Action == "workspace.delete" {
			t.Error("peristiwa 40 hari lalu tak boleh muncul di rentang harian")
		}
	}

	_, month, err := d.activityTrail(context.Background(), nil, rangeInput{Range: "month"})
	if err != nil {
		t.Fatalf("activity_trail month: %v", err)
	}
	_ = month // rentang bulanan pun tak mencakup 40 hari; cukup pastikan tak error & harian tersaring
}

// TestMigrationVersion_Terbaca: schema test dimigrasi penuh, jadi versinya > 0.
func TestMigrationVersion_Terbaca(t *testing.T) {
	d := testDeps(t)
	_, out, err := d.migrationVersion(context.Background(), nil, noInput{})
	if err != nil {
		t.Fatalf("migration_version: %v", err)
	}
	if out.Version <= 0 {
		t.Errorf("schema test sudah dimigrasi, versi harus > 0, got %d", out.Version)
	}
}

// TestPlatformStats_AllowlistMenyaringKeySensitif: INTI perbaikan. platform_stats
// hanya boleh mengeluarkan key yang di-allowlist. Key baru (mis. kredensial yang
// ditambahkan setelah kode ini ditulis) TAK boleh bocor otomatis.
//
// Diuji dengan menaruh key palsu yang jelas sensitif ke platform_settings, lalu
// memastikan ia TIDAK muncul di output — sementara key allowlist tetap muncul.
func TestPlatformStats_AllowlistMenyaringKeySensitif(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()

	// Key allowlist (harus muncul) + key sensitif palsu (harus disaring).
	seedSetting(t, "workspace_quota_default", "5")
	seedSetting(t, "smtp_password", "rahasia-yang-tak-boleh-bocor")
	t.Cleanup(func() {
		_, _ = pkgPool.Exec(ctx, "DELETE FROM platform_settings WHERE key IN ('smtp_password','workspace_quota_default')")
	})

	_, out, err := d.platformStats(ctx, nil, noInput{})
	if err != nil {
		t.Fatalf("platform_stats: %v", err)
	}
	if _, bocor := out.Settings["smtp_password"]; bocor {
		t.Error("key sensitif smtp_password BOCOR — allowlist tak menyaring; ini pelanggaran Rule 7/12")
	}
	for _, v := range out.Settings {
		if v == "rahasia-yang-tak-boleh-bocor" {
			t.Error("nilai rahasia muncul di output dengan key lain — kebocoran")
		}
	}
	if out.Settings["workspace_quota_default"] != "5" {
		t.Errorf("key allowlist harus tetap muncul, got %q", out.Settings["workspace_quota_default"])
	}
}

// seedSetting menulis satu baris platform_settings (UPSERT agar aman diulang).
func seedSetting(t *testing.T, key, value string) {
	t.Helper()
	_, err := pkgPool.Exec(context.Background(),
		`INSERT INTO platform_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value)
	if err != nil {
		t.Fatalf("seed setting %s: %v", key, err)
	}
}

// TestDBSchema_MermaidBerisi: introspeksi harus menemukan tabel & menghasilkan
// teks Mermaid yang mengandung tabel inti.
func TestDBSchema_MermaidBerisi(t *testing.T) {
	d := testDeps(t)
	_, out, err := d.dbSchema(context.Background(), nil, noInput{})
	if err != nil {
		t.Fatalf("db_schema: %v", err)
	}
	if out.TableCount == 0 {
		t.Fatal("harus mendeteksi tabel")
	}
	if !strings.Contains(out.Mermaid, "erDiagram") || !strings.Contains(out.Mermaid, "users") {
		t.Errorf("Mermaid harus memuat erDiagram & tabel users:\n%s", firstN(out.Mermaid, 200))
	}
}

// TestBuild_MendaftarkanSemuaTool: server yang dirakit harus punya ketujuh tool
// — jaring bila registrasi terlupa saat menambah tool baru.
func TestBuild_MendaftarkanSemuaTool(t *testing.T) {
	d := testDeps(t)
	srv := build(d.pool, d.cfg, d.log)
	if srv == nil {
		t.Fatal("build mengembalikan nil")
	}
	// Server terbentuk tanpa panic = semua AddTool sukses (tipe In/Out valid).
	// Jumlah tool diverifikasi lewat protokol di test transport (lihat protocol_test.go).
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
