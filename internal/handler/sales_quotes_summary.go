package handler

import (
	"context"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_quotes_summary.go — BL-18: ringkasan agregat quote per deal ("N quote ·
// M kedaluwarsa"). Opsi A: quote di bawah satu deal SENGAJA tetap independen (log
// revisi) — TAK ada penegakan "satu utama/Accepted per deal", TAK menyentuh alur
// Closed Won→subscription. Hanya PERJELAS TAMPILAN mana yang masih berlaku vs
// kedaluwarsa. Hitung dipisah dari baris termuat (kartu detail hanya 5, daftar
// ter-keyset sepotong) → butuh query sendiri (ListQuoteBucketsForDeal).

// quotesSummaryForDeal memuat SEMUA quote hidup satu deal dan membucketnya jadi
// ringkasan (BL-18). Best-effort seperti dealQuotesPreview: gagal query →
// ringkasan nol (halaman tetap terbaca, bukan 500). Deal biasanya sedikit quote
// → murah; hindari keyset (butuh total penuh, bukan sepotong).
func (h *Handler) quotesSummaryForDeal(ctx context.Context, dealID int64) panel.QuotesSummary {
	rows, err := h.q(ctx).ListQuoteBucketsForDeal(ctx, &dealID)
	if err != nil {
		h.Log.Error("quotes: summary", "err", err)
		return panel.QuotesSummary{}
	}
	return quotesSummaryOf(rows, todayInAppTZ())
}

// quotesSummaryOf membucket baris (status, expiration_date) → ringkasan. Dipisah
// dari query agar teruji tanpa DB. Aturan "kedaluwarsa" IDENTIK BL-17 (quoteExpired:
// Draft dikecualikan, banding date-only di appTZ) — konsistensi disengaja. Active =
// belum kedaluwarsa & status bukan terminal-tak-berlaku (Rejected/Expired). today =
// "hari ini" menurut appTZ.
func quotesSummaryOf(rows []db.ListQuoteBucketsForDealRow, today time.Time) panel.QuotesSummary {
	s := panel.QuotesSummary{Total: len(rows)}
	for _, r := range rows {
		if quoteExpired(r.QuoteStatus, r.ExpirationDate, today) {
			s.Expired++
			continue
		}
		if r.QuoteStatus != "Rejected" && r.QuoteStatus != "Expired" {
			s.Active++
		}
	}
	return s
}
