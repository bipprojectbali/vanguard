package health

import (
	"strings"
	"testing"
	"testing/fstest"
)

// lines membuat konten dengan n baris.
func lines(n int) []byte {
	return []byte(strings.Repeat("x\n", n))
}

func TestClassify(t *testing.T) {
	cases := []struct {
		path string
		kind string
		max  int
	}{
		{"internal/handler/auth.go", "Route/Handler", 150},
		{"routes.go", "Route/Handler", 150},
		{"internal/config/config.go", "Config", 100},
		{"internal/db/users.sql.go", "Repository/Query", 250},
		{"internal/ui/layout.go", "View/Component", 300},
		{"internal/auth/password.go", "Service/Lainnya", 300},
		{"internal/handler/auth_test.go", "Test", 400}, // _test menang atas /handler/
	}
	for _, c := range cases {
		k := classify(c.path)
		if k.Name != c.kind || k.MaxLines != c.max {
			t.Errorf("classify(%q)=%+v, want %s/%d", c.path, k, c.kind, c.max)
		}
	}
}

func TestIsExcluded(t *testing.T) {
	excluded := []string{
		"routes.go", // SSOT semua route, sengaja over-limit
		"internal/db/users.sql.go",
		"internal/x/foo.generated.go",
		"internal/x/foo_gen.go",
		"vendor/lib/x.go",
	}
	for _, p := range excluded {
		if !isExcluded(p) {
			t.Errorf("%q harusnya excluded", p)
		}
	}
	if isExcluded("internal/handler/auth.go") {
		t.Error("file normal tak boleh excluded")
	}
	// Hanya routes.go di ROOT yang SSOT; routes.go bersarang tetap discan.
	if isExcluded("internal/foo/routes.go") {
		t.Error("routes.go bersarang tak boleh ikut excluded")
	}
}

func TestEvaluate_Thresholds(t *testing.T) {
	// Handler 120 baris → sehat (di bawah 150).
	if r := evaluate("internal/handler/x.go", 120, 3000); !r.Healthy {
		t.Errorf("handler 120 baris harus sehat, got %+v", r)
	}
	// Handler 200 baris → tak sehat (>150 tipe).
	r := evaluate("internal/handler/x.go", 200, 4000)
	if r.Healthy {
		t.Error("handler 200 baris harus tak sehat (>150)")
	}
	// Service 600 baris → tak sehat (>500 hard limit + >300 tipe).
	r = evaluate("internal/svc/x.go", 600, 10000)
	if r.Healthy || len(r.Reasons) < 2 {
		t.Errorf("service 600 baris harus tak sehat >1 alasan, got %+v", r)
	}
	// Karakter berlebih walau baris sedikit.
	r = evaluate("internal/svc/x.go", 50, 25000)
	if r.Healthy {
		t.Error("25k karakter harus tak sehat")
	}
}

func TestScan_SortsUnhealthyFirst(t *testing.T) {
	fsys := fstest.MapFS{
		"internal/handler/big.go":   {Data: lines(300)},  // tak sehat (>150)
		"internal/handler/small.go": {Data: lines(50)},   // sehat
		"internal/svc/huge.go":      {Data: lines(600)},  // tak sehat (>500)
		"internal/db/gen.sql.go":    {Data: lines(9999)}, // EXCLUDED (generated)
		"internal/ui/mid.go":        {Data: lines(200)},  // sehat (view 300)
		"README.md":                 {Data: lines(999)},  // bukan .go, di-skip
	}
	res, err := Scan(fsys)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// 4 file .go non-excluded (big, small, huge, mid). gen.sql.go excluded.
	if res.Total != 4 {
		t.Errorf("Total=%d, want 4 (gen.sql.go excluded, README di-skip)", res.Total)
	}
	if res.Unhealthy != 2 {
		t.Errorf("Unhealthy=%d, want 2 (big, huge)", res.Unhealthy)
	}
	// Dua teratas harus tak sehat.
	if res.Reports[0].Healthy || res.Reports[1].Healthy {
		t.Errorf("dua teratas harus tak sehat: %+v", res.Reports[:2])
	}
	// Yang terbesar (huge 600) di paling atas.
	if res.Reports[0].Lines != 600 {
		t.Errorf("teratas harus file terbesar (600), got %d", res.Reports[0].Lines)
	}
	// Pastikan generated tak ikut.
	for _, r := range res.Reports {
		if strings.HasSuffix(r.Path, ".sql.go") {
			t.Errorf("file generated tak boleh masuk: %s", r.Path)
		}
	}
}
