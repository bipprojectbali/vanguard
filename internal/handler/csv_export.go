package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
)

// csv_export.go — helper generik ekspor CSV manual (tasks.md M8-1, "export-CSV
// manual"). Pola BARU di repo (belum ada presedennya) — disendirikan di sini
// agar reports_sales.go/reports_subscriptions.go pakai bareng, bukan duplikasi.
// encoding/csv menangani quoting/escaping (koma, quote, newline dalam nilai)
// sesuai RFC 4180 — jangan tulis join(",") manual.

// writeCSV menulis header+rows sebagai file CSV attachment. filename TANPA
// ekstensi/tanda kutip liar — dibungkus quote di sini. Dipanggil di akhir
// handler export (bukan di tengah rendering HTML) → header sudah final saat
// WriteHeader implisit oleh csv.Writer.Flush.
func writeCSV(w http.ResponseWriter, filename string, header []string, rows [][]string) error {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, filename))

	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("csv: write header: %w", err)
	}
	if err := cw.WriteAll(rows); err != nil {
		return fmt.Errorf("csv: write rows: %w", err)
	}
	cw.Flush()
	return cw.Error()
}
