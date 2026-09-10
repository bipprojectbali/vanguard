package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_quotes_status_items_test.go — BL-148: quote tanpa line item dikunci ke
// Draft. View hanya menawarkan opsi Draft; backend QuoteStatus = penjaga
// sesungguhnya (tolak transisi non-Draft dengan quote_no_items). Setelah ada ≥1
// item, semua transisi jalan normal.

// TestQuoteStatus_NoItemsRejectsNonDraft: POST status Sent atas quote tanpa item →
// 303 err=quote_no_items & status tetap Draft (backend penjaga).
func TestQuoteStatus_NoItemsRejectsNonDraft(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kunci", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: jendela quoting
	q := env.seedQuote(t, deal.ID, acc.ID, "0")   // tanpa item

	form := url.Values{"quote_status": {"Sent"}}
	req := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/status", form,
		itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=quote_no_items") {
		t.Fatalf("status non-Draft tanpa item harus err=quote_no_items, got %q (status %d)", loc, rec.Code)
	}
	if s := env.mustGetQuote(t, q.ID).QuoteStatus; s != "Draft" {
		t.Errorf("quote harus tetap Draft, got %q", s)
	}
}

// TestQuoteStatus_WithItemsAllowsTransition: quote ber-item ≥1 → transisi ke Sent
// sah (ok=status), status tersimpan.
func TestQuoteStatus_WithItemsAllowsTransition(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Maju", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	plan := env.seedPlan(t, "Paket Maju", "PLN-MAJU", "1000000")
	env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1")

	form := url.Values{"quote_status": {"Sent"}}
	req := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/status", form,
		itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=status") {
		t.Fatalf("status ber-item harus ok=status, got %q (status %d)\n%s",
			loc, rec.Code, rec.Body.String())
	}
	if s := env.mustGetQuote(t, q.ID).QuoteStatus; s != "Sent" {
		t.Errorf("quote harus jadi Sent, got %q", s)
	}
}

// TestQuoteStatus_DraftAlwaysAllowedWithoutItems: menyetel ulang ke Draft (status
// awal) atas quote tanpa item TIDAK terhalang guard (guard hanya untuk non-Draft).
func TestQuoteStatus_DraftAlwaysAllowedWithoutItems(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Draft", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	form := url.Values{"quote_status": {"Draft"}}
	req := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/status", form,
		itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteStatus)
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("Draft→Draft tak boleh ditolak, got %q", loc)
	}
	if s := env.mustGetQuote(t, q.ID).QuoteStatus; s != "Draft" {
		t.Errorf("quote harus tetap Draft, got %q", s)
	}
}

// TestQuoteDetail_NoItemsLimitsStatusOptions: halaman detail quote tanpa item hanya
// merender opsi Draft (tak menawarkan Sent dst.) + catatan pemandu. Dgn ≥1 item,
// opsi lanjutan muncul kembali.
func TestQuoteDetail_NoItemsLimitsStatusOptions(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Opsi", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	get := quotesReq(http.MethodGet, quoteSub(deal.ID, q.ID), nil, itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", get, env.h.QuoteDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `value="Sent"`) {
		t.Errorf("quote tanpa item tak boleh menawarkan opsi Sent, body:\n%s", body)
	}
	if !strings.Contains(body, "Tambahkan minimal satu item") {
		t.Errorf("quote tanpa item harus memuat catatan pemandu, body:\n%s", body)
	}

	// Tambah item → opsi lanjutan kembali muncul.
	plan := env.seedPlan(t, "Paket Opsi", "PLN-OPSI", "500000")
	env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1")
	rec2 := env.runAccount(uid, "owner", "admin",
		quotesReq(http.MethodGet, quoteSub(deal.ID, q.ID), nil, itoa(deal.ID), itoa(q.ID), ""),
		env.h.QuoteDetail)
	if !strings.Contains(rec2.Body.String(), `value="Sent"`) {
		t.Errorf("quote ber-item harus menawarkan opsi Sent lagi, body:\n%s", rec2.Body.String())
	}
}
