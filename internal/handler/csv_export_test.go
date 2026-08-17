package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// csv_export_test.go — writeCSV murni-fungsi (tanpa DB) → test langsung tanpa
// TestMain/pkgPool, meniru pola test util murni lain (dateStr_test.go dkk).

// TestWriteCSV_HeaderAndContentType: header CSV + Content-Type/Content-Disposition
// terset benar, baris muncul sesuai urutan input.
func TestWriteCSV_HeaderAndContentType(t *testing.T) {
	w := httptest.NewRecorder()
	err := writeCSV(w, "pipeline", []string{"Stage", "Jumlah"}, [][]string{
		{"Prospecting", "3"},
		{"Demo", "1"},
	})
	if err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/csv; charset=utf-8", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="pipeline.csv"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	body := w.Body.String()
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("baris CSV = %d, want 3 (header+2 data), got body=%q", len(lines), body)
	}
	if lines[0] != "Stage,Jumlah" {
		t.Errorf("header baris = %q", lines[0])
	}
	if lines[1] != "Prospecting,3" || lines[2] != "Demo,1" {
		t.Errorf("data baris tak sesuai, got %v", lines[1:])
	}
}

// TestWriteCSV_EscapesCommaAndQuote: nilai yang mengandung koma/quote wajib
// dibungkus quote sesuai RFC 4180 — regresi utk memastikan encoding/csv dipakai,
// bukan join(",") manual.
func TestWriteCSV_EscapesCommaAndQuote(t *testing.T) {
	w := httptest.NewRecorder()
	err := writeCSV(w, "export", []string{"Nama"}, [][]string{
		{`Desa "Maju", Blok A`},
	})
	if err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	body := w.Body.String()
	want := `"Desa ""Maju"", Blok A"`
	if !strings.Contains(body, want) {
		t.Errorf("body tak mengandung nilai ter-escape %q, got %q", want, body)
	}
}
