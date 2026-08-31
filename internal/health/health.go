// Package health menyaring "kesehatan" file sumber Go: menghitung baris &
// karakter tiap file, membandingkan dengan ambang per-tipe, dan melaporkan file
// yang melewati batas. Berbasis aturan file-health (route 150, service 300,
// dst; hard limit 500 baris / 20.000 karakter).
//
// Dev-only: butuh source .go di disk, yang tak ada di build single-binary.
package health

import (
	"bufio"
	"bytes"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// Batas global (hard limit) — berlaku untuk semua tipe.
const (
	MaxLines = 500
	MaxChars = 20000
)

// Kind = klasifikasi file berdasar path/nama, menentukan ambang baris.
type Kind struct {
	Name     string // "Route/Handler", "Service", ...
	MaxLines int    // ambang baris spesifik tipe
}

// Report = hasil scan satu file.
type Report struct {
	Path    string
	Kind    string
	Lines   int
	Chars   int
	Limit   int      // ambang baris efektif (min tipe & global)
	Healthy bool     // true bila di bawah semua ambang
	Reasons []string // alasan tak sehat (baris/karakter)
}

// classify menentukan Kind dari path relatif. Urutan cek: paling spesifik dulu.
func classify(path string) Kind {
	base := path[strings.LastIndex(path, "/")+1:]
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return Kind{"Test", 400}
	case strings.Contains(path, "/handler/") || strings.HasSuffix(base, "routes.go"):
		return Kind{"Route/Handler", 150}
	case strings.Contains(path, "/config/"):
		return Kind{"Config", 100}
	case strings.Contains(path, "/db/") || strings.Contains(path, "/database/"):
		return Kind{"Repository/Query", 250}
	case strings.Contains(path, "/ui/"):
		return Kind{"View/Component", 300}
	default:
		return Kind{"Service/Lainnya", 300}
	}
}

// genMarker mengenali penanda standar Go untuk kode hasil-generate
// (`^// Code generated .* DO NOT EDIT\.$`, lihat `go help generate`). sqlc
// memancarkan penanda ini di baris pertama querier.go/models.go/db.go yang
// TIDAK berakhiran `.sql.go`, sehingga tak tertangkap isExcluded berbasis-path.
var genMarker = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// isExcluded melaporkan apakah path dikecualikan dari scan (generated/vendor).
func isExcluded(path string) bool {
	// File generated sqlc & sejenisnya.
	if strings.Contains(path, "/db/") && strings.HasSuffix(path, ".sql.go") {
		return true
	}
	if strings.HasSuffix(path, ".generated.go") || strings.HasSuffix(path, "_gen.go") {
		return true
	}
	for _, seg := range []string{"vendor/", ".git/", "tmp/", "__mocks__/", "__fixtures__/"} {
		// Cocok baik sebagai prefix (vendor/x) maupun segmen (a/vendor/x).
		if strings.HasPrefix(path, seg) || strings.Contains(path, "/"+seg) {
			return true
		}
	}
	return false
}

// countFile menghitung baris & karakter satu file dari fsys, sekaligus
// mendeteksi penanda kode hasil-generate di bagian kepala (sebelum klausa
// `package`) — konvensi resmi Go. Penanda setelah `package` diabaikan.
func countFile(fsys fs.FS, path string) (lines, chars int, generated bool, err error) {
	f, err := fsys.Open(path)
	if err != nil {
		return 0, 0, false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024) // toleransi baris panjang
	inHead := true                                // penanda generate hanya sah sebelum `package`
	for sc.Scan() {
		lines++
		line := sc.Bytes()
		chars += len(line) + 1 // +1 newline
		if inHead {
			trimmed := bytes.TrimSpace(line)
			switch {
			case bytes.HasPrefix(trimmed, []byte("package ")):
				inHead = false
			case genMarker.Match(trimmed):
				generated = true
				inHead = false
			}
		}
	}
	return lines, chars, generated, sc.Err()
}

// evaluate membangun Report untuk satu file yang sudah dihitung.
func evaluate(path string, lines, chars int) Report {
	k := classify(path)
	limit := k.MaxLines
	if MaxLines < limit {
		limit = MaxLines
	}
	r := Report{Path: path, Kind: k.Name, Lines: lines, Chars: chars, Limit: limit, Healthy: true}
	if lines > k.MaxLines {
		r.Healthy = false
		r.Reasons = append(r.Reasons, "baris melebihi ambang tipe")
	}
	if lines > MaxLines {
		r.Healthy = false
		r.Reasons = append(r.Reasons, "baris melebihi hard limit 500")
	}
	if chars > MaxChars {
		r.Healthy = false
		r.Reasons = append(r.Reasons, "karakter melebihi 20.000")
	}
	return r
}

// Result = hasil scan lengkap.
type Result struct {
	Reports   []Report // terurut: tak sehat dulu, lalu terbesar
	Total     int
	Unhealthy int
}

// Scan menelusuri fsys, memindai semua .go non-excluded, dan mengembalikan
// laporan terurut (tak sehat di atas, lalu baris terbanyak).
func Scan(fsys fs.FS) (Result, error) {
	var reports []Report
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || isExcluded(path) {
			return nil
		}
		lines, chars, generated, err := countFile(fsys, path)
		if err != nil {
			return err
		}
		if generated {
			return nil // kode hasil-generate (header DO NOT EDIT): dikecualikan §8
		}
		reports = append(reports, evaluate(path, lines, chars))
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Healthy != reports[j].Healthy {
			return !reports[i].Healthy // tak sehat dulu
		}
		return reports[i].Lines > reports[j].Lines // lalu terbesar
	})

	res := Result{Reports: reports, Total: len(reports)}
	for _, r := range reports {
		if !r.Healthy {
			res.Unhealthy++
		}
	}
	return res, nil
}
