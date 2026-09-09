package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_quotes_accept_test.go — helper bersama transisi status quote untuk test.
// acceptQuote dipakai lintas test (BL-88 quote otoritatif, Won→langganan). Test
// backfill plan_requested_id di-retire bersama helper backfill (BL-88 PR2b: paket
// diturunkan langsung dari quote_items saat Closed Won, bukan lewat deal).

// acceptQuote menjalankan QuoteStatus dengan status tertentu sebagai admin owner dan
// mengembalikan status (assert redirect ok=status).
func (e *testEnv) acceptQuote(t *testing.T, uid, dealID, quoteID int64, status string) string {
	t.Helper()
	form := url.Values{"quote_status": {status}}
	req := quotesReq(http.MethodPost, quoteSub(dealID, quoteID)+"/status", form,
		itoa(dealID), itoa(quoteID), "")
	rec := e.runAccount(uid, "owner", "admin", req, e.h.QuoteStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=status") {
		t.Fatalf("status %q harus ok=status, got %q (code %d)", status, loc, rec.Code)
	}
	return status
}
