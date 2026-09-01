package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_expiration_test.go — BL-17: penegakan `expiration_date` quote
// (Opsi 1, computed-on-read). Dua sisi: (a) validasi submit menolak tanggal
// lampau di create & edit (penjaga backend, SATU aturan lewat parseQuoteForm),
// (b) penanda kedaluwarsa `quoteExpired` (Draft dikecualikan). Tanpa scheduler/
// tulis-DB: enforcement ringan. appTZ default UTC di test → tanggal relatif
// dihitung UTC agar cocok dengan todayInAppTZ() handler.

// dateOffset = tanggal N hari dari hari ini (UTC), format ISO input date.
func dateOffset(days int) string {
	return time.Now().UTC().AddDate(0, 0, days).Format(dateLayout)
}

// TestQuotes_ExpirationPast_Rejected: submit tanggal kedaluwarsa < hari ini
// ditolak (err=expiration_past) baik saat create maupun edit; == hari ini &
// kosong (NULL) tetap sukses. Menegakkan aturan (a) BL-17.
func TestQuotes_ExpirationPast_Rejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Exp", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting

	// --- CREATE ---
	// Tanggal lampau → ditolak sebelum DB, redirect ke form dengan err.
	past := url.Values{"expiration_date": {dateOffset(-1)}}
	req := quotesReq(http.MethodPost, quoteListSub(deal.ID), past, itoa(deal.ID), "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=expiration_past") {
		t.Fatalf("create tanggal lampau harus err=expiration_past, got %q (status %d)", loc, rec.Code)
	}
	if rows := env.dealQuotes(t, deal.ID); len(rows) != 0 {
		t.Fatalf("tanggal lampau tak boleh menyimpan, ada %d quote", len(rows))
	}

	// Tanggal == hari ini → sukses (berlaku s/d akhir hari).
	today := url.Values{"expiration_date": {dateOffset(0)}}
	req = quotesReq(http.MethodPost, quoteListSub(deal.ID), today, itoa(deal.ID), "", "")
	rec = env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("create tanggal hari ini harus ok=created, got %q\n%s", loc, rec.Body.String())
	}

	// Tanpa tanggal (NULL) → sukses.
	none := url.Values{"quote_name": {"Tanpa Kedaluwarsa"}}
	req = quotesReq(http.MethodPost, quoteListSub(deal.ID), none, itoa(deal.ID), "", "")
	rec = env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("create tanpa tanggal harus ok=created, got %q", loc)
	}

	// --- EDIT ---
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	// Sunting dengan tanggal lampau → ditolak (aturan sama dgn create).
	editPast := url.Values{"expiration_date": {dateOffset(-3)}}
	req = quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID), editPast, itoa(deal.ID), itoa(q.ID), "")
	rec = env.runAccount(uid, "owner", "admin", req, env.h.QuoteUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=expiration_past") {
		t.Fatalf("edit tanggal lampau harus err=expiration_past, got %q", loc)
	}
	if got := env.mustGetQuote(t, q.ID); got.ExpirationDate.Valid {
		t.Fatalf("edit tanggal lampau tak boleh menyimpan, expiration=%v", got.ExpirationDate.Time)
	}

	// Sunting dengan tanggal depan → sukses tersimpan.
	editOK := url.Values{"expiration_date": {dateOffset(7)}}
	req = quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID), editOK, itoa(deal.ID), itoa(q.ID), "")
	rec = env.runAccount(uid, "owner", "admin", req, env.h.QuoteUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("edit tanggal depan harus ok=saved, got %q", loc)
	}
	if got := env.mustGetQuote(t, q.ID); !got.ExpirationDate.Valid {
		t.Fatal("edit tanggal depan harus tersimpan, expiration masih NULL")
	}
}

// TestQuoteExpired_Flag: penanda kedaluwarsa (b) BL-17. Draft SELALU dikecualikan;
// non-Draft dgn tanggal lampau → true; hari ini / depan / NULL → false.
func TestQuoteExpired_Flag(t *testing.T) {
	today := time.Now().UTC()
	dateOf := func(days int) pgtype.Date {
		return pgtype.Date{Time: today.AddDate(0, 0, days), Valid: true}
	}
	null := pgtype.Date{Valid: false}

	cases := []struct {
		name   string
		status string
		exp    pgtype.Date
		want   bool
	}{
		{"non-draft tanggal lampau → kedaluwarsa", "Sent", dateOf(-1), true},
		{"draft tanggal lampau → dikecualikan", "Draft", dateOf(-1), false},
		{"non-draft hari ini → belum kedaluwarsa", "Sent", dateOf(0), false},
		{"non-draft tanggal depan → belum kedaluwarsa", "Accepted", dateOf(5), false},
		{"non-draft tanpa tanggal → tak kedaluwarsa", "Sent", null, false},
		{"draft tanpa tanggal → tak kedaluwarsa", "Draft", null, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := quoteExpired(c.status, c.exp, today); got != c.want {
				t.Errorf("quoteExpired(%q, %v) = %v, want %v", c.status, c.exp.Time, got, c.want)
			}
		})
	}
}
