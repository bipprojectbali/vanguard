package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_summary_test.go — BL-18: bucket ringkasan agregat quote per deal.
// Menguji quotesSummaryOf (fungsi murni, tanpa DB): Total/Expired/Active dihitung
// benar dari campuran status+tanggal, aturan "kedaluwarsa" IDENTIK BL-17 (Draft
// dikecualikan walau tanggalnya lampau), dan deal tanpa quote → ringkasan nol.
// appTZ default UTC di test → tanggal relatif dihitung UTC agar cocok handler.

func bucketRow(status string, days int, hasDate bool) db.ListQuoteBucketsForDealRow {
	exp := pgtype.Date{Valid: false}
	if hasDate {
		exp = pgtype.Date{Time: time.Now().UTC().AddDate(0, 0, days), Valid: true}
	}
	return db.ListQuoteBucketsForDealRow{QuoteStatus: status, ExpirationDate: exp}
}

// TestQuotesSummaryOf_Mixed: campuran status & tanggal → Total/Expired/Active.
// - Sent lampau            → kedaluwarsa (Expired), bukan Active
// - Accepted depan         → Active (belum kedaluwarsa, bukan terminal-tak-berlaku)
// - Draft lampau           → TIDAK kedaluwarsa (BL-17), Active (Draft masih hidup)
// - Rejected depan         → bukan Expired, bukan Active (terminal-tak-berlaku)
// - Sent tanpa tanggal     → Active (tak bisa kedaluwarsa tanpa tanggal)
func TestQuotesSummaryOf_Mixed(t *testing.T) {
	today := time.Now().UTC()
	rows := []db.ListQuoteBucketsForDealRow{
		bucketRow("Sent", -1, true),
		bucketRow("Accepted", 5, true),
		bucketRow("Draft", -3, true),
		bucketRow("Rejected", 5, true),
		bucketRow("Sent", 0, false),
	}
	got := quotesSummaryOf(rows, today)
	if got.Total != 5 {
		t.Errorf("Total = %d, want 5", got.Total)
	}
	if got.Expired != 1 {
		t.Errorf("Expired = %d, want 1 (hanya Sent lampau)", got.Expired)
	}
	if got.Active != 3 {
		t.Errorf("Active = %d, want 3 (Accepted depan, Draft lampau, Sent tanpa tanggal)", got.Active)
	}
}

// TestQuotesSummaryOf_DraftPastNotExpired: penegas eksplisit konsistensi BL-17 —
// Draft dengan tanggal lampau TIDAK dihitung kedaluwarsa (masih WIP, belum
// ditawarkan), tetap masuk Active.
func TestQuotesSummaryOf_DraftPastNotExpired(t *testing.T) {
	got := quotesSummaryOf([]db.ListQuoteBucketsForDealRow{
		bucketRow("Draft", -10, true),
	}, time.Now().UTC())
	if got.Expired != 0 {
		t.Errorf("Draft lampau: Expired = %d, want 0", got.Expired)
	}
	if got.Active != 1 || got.Total != 1 {
		t.Errorf("Draft lampau: Total/Active = %d/%d, want 1/1", got.Total, got.Active)
	}
}

// TestQuotesSummaryOf_Empty: deal tanpa quote → ringkasan nol (kartu/header sudah
// menangani state kosong; badge disembunyikan via Total>0).
func TestQuotesSummaryOf_Empty(t *testing.T) {
	got := quotesSummaryOf(nil, time.Now().UTC())
	if got != (panel.QuotesSummary{}) {
		t.Errorf("kosong → %+v, want semua nol", got)
	}
}
