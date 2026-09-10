package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_quotes_authoritative_test.go — BL-88 PR1: quote = sumber kebenaran komersial.
// Membuktikan: (1) Accept menyalin grand_total quote → deal.Amount (nilai DIAKUI,
// menggantikan perkiraan manual) + audit; (2) TEPAT 1 quote Accepted per deal — Accept
// kedua ditolak (quote_already_accepted); (3) termin langganan tersimpan di quote
// (subscription_term + contract_term_months diturunkan) via form; (4) termin tak sah
// ditolak. Alur Won yang membaca termin dari quote diuji di sales_deals_stage_test.go.

// TestQuoteAccept_RecognizesGrandTotalToDealAmount: saat quote di-Accept, grand_total
// quote disalin ke deal.Amount (nilai diakui). deal lahir tanpa amount → sesudah Accept
// = grand_total item.
func TestQuoteAccept_RecognizesGrandTotalToDealAmount(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Recog", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: jendela quoting
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	plan := env.seedPlan(t, "Paket Inti", "PLN-RCG", "1000000")
	env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "3") // grand_total = 3.000.000

	env.acceptQuote(t, uid, deal.ID, q.ID, "Accepted")

	gt := env.mustGetQuote(t, q.ID).GrandTotal
	got := env.freshDeal(t, deal.ID)
	if ratFromNumeric(got.Amount).Cmp(ratFromNumeric(gt)) != 0 {
		t.Errorf("deal.Amount = %s, want grand_total %s (nilai diakui dari quote)",
			formatRupiah(got.Amount), formatRupiah(gt))
	}
	env.assertAudited(t, "deal.value.recognized")
}

// TestQuoteAccept_SecondAcceptRejected: TEPAT 1 quote Accepted per deal. Setelah q1
// di-Accept, meng-Accept q2 pada deal yang SAMA ditolak (quote_already_accepted) &
// ATOMIK — q2 tetap Draft, q1 tetap satu-satunya Accepted.
func TestQuoteAccept_SecondAcceptRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Satu", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q1 := env.seedQuote(t, deal.ID, acc.ID, "0")
	q2 := env.seedQuote(t, deal.ID, acc.ID, "0")
	// BL-148: quote wajib punya ≥1 item sebelum maju dari Draft (guard mendahului
	// guard BL-88); beri item ke keduanya agar keduanya lolos ke jalur Accept.
	plan := env.seedPlan(t, "Paket Satu", "PLN-SATU", "1000000")
	env.addQuoteItem(t, uid, deal.ID, q1.ID, plan, "1")
	env.addQuoteItem(t, uid, deal.ID, q2.ID, plan, "1")

	env.acceptQuote(t, uid, deal.ID, q1.ID, "Accepted")

	// Accept kedua pada deal sama → tolak (tak fatal: kita periksa Location sendiri).
	form := url.Values{"quote_status": {"Accepted"}}
	req := quotesReq(http.MethodPost, quoteSub(deal.ID, q2.ID)+"/status", form,
		itoa(deal.ID), itoa(q2.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=quote_already_accepted") {
		t.Fatalf("Accept kedua harus err=quote_already_accepted, got %q (status %d)", loc, rec.Code)
	}
	if s := env.mustGetQuote(t, q2.ID).QuoteStatus; s != "Draft" {
		t.Errorf("q2 harus tetap Draft (ATOMIK), got %q", s)
	}
	if s := env.mustGetQuote(t, q1.ID).QuoteStatus; s != "Accepted" {
		t.Errorf("q1 harus tetap Accepted, got %q", s)
	}
}

// TestQuoteCreate_StoresSubscriptionTerm: BL-88 — termin langganan pindah ke quote.
// Form quote dgn subscription_term=Annual menyimpan term + menurunkan contract_term_months
// (peta termContractMonths → 12).
func TestQuoteCreate_StoresSubscriptionTerm(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Termin", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")

	form := url.Values{"quote_name": {"Quote Termin"}, "subscription_term": {"Annual"}}
	req := quotesReq(http.MethodPost, quoteListSub(deal.ID), form, itoa(deal.ID), "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("create quote bertermin harus ok=created, got %q\n%s", loc, rec.Body.String())
	}

	rows := env.dealQuotes(t, deal.ID)
	if len(rows) != 1 {
		t.Fatalf("harus 1 quote, ada %d", len(rows))
	}
	q := rows[0]
	if deref(q.SubscriptionTerm) != "Annual" {
		t.Errorf("subscription_term = %q, want Annual", deref(q.SubscriptionTerm))
	}
	if q.ContractTermMonths == nil || *q.ContractTermMonths != 12 {
		t.Errorf("contract_term_months = %v, want 12 (diturunkan dari Annual)", q.ContractTermMonths)
	}
}

// TestDealDetail_TermFromAcceptedQuote: BL-88 — termin milik quote, bukan deal.
// Detail deal merender "Termin Langganan" dari quote Accepted (deal.SubscriptionTerm
// tak lagi diisi form). Regresi: dulu baris ini "—" walau quote sudah Accepted
// bertermin, karena view membaca deal.SubscriptionTerm yang kosong.
func TestDealDetail_TermFromAcceptedQuote(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Termin Detail", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")

	// Buat quote bertermin Annual lewat form, lalu Accept.
	form := url.Values{"quote_name": {"Quote Termin"}, "subscription_term": {"Annual"}}
	req := quotesReq(http.MethodPost, quoteListSub(deal.ID), form, itoa(deal.ID), "", "")
	if loc := env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate).
		Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("create quote bertermin harus ok=created, got %q", loc)
	}
	rows := env.dealQuotes(t, deal.ID)
	if len(rows) != 1 {
		t.Fatalf("harus 1 quote, ada %d", len(rows))
	}
	// BL-148: quote butuh ≥1 item sebelum bisa maju dari Draft ke Accepted.
	plan := env.seedPlan(t, "Paket Termin", "PLN-TERMIN", "1000000")
	env.addQuoteItem(t, uid, deal.ID, rows[0].ID, plan, "1")
	env.acceptQuote(t, uid, deal.ID, rows[0].ID, "Accepted")

	// Detail deal harus menampilkan termin dari quote Accepted.
	get := accountsReq(http.MethodGet, "/w/test/deals/"+itoa(deal.ID), nil, itoa(deal.ID))
	rec := env.runAccount(uid, "owner", "admin", get, env.h.DealDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Termin Langganan") || !strings.Contains(body, "Annual") {
		t.Errorf("detail deal harus merender Termin Langganan = Annual (dari quote), body:\n%s", body)
	}
}

// TestQuoteCreate_InvalidTermRejected: termin di luar enum (mirror validSubscriptionTerms)
// ditolak sebelum DB (err=deal_term), tak ada quote tersimpan.
func TestQuoteCreate_InvalidTermRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Salah", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")

	form := url.Values{"subscription_term": {"Weekly"}} // bukan Monthly/Annual/Multi-year
	req := quotesReq(http.MethodPost, quoteListSub(deal.ID), form, itoa(deal.ID), "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=deal_term") {
		t.Fatalf("termin tak sah harus err=deal_term, got %q", loc)
	}
	if rows := env.dealQuotes(t, deal.ID); len(rows) != 0 {
		t.Fatalf("termin tak sah tak boleh menyimpan, ada %d quote", len(rows))
	}
}
