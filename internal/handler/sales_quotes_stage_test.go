package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_quotes_stage_test.go — regresi BL-13: quote hanya boleh DIBUAT/DIUBAH saat
// deal di jendela quoting (Qualification–Negotiation). Prospecting = terlalu dini
// (diblokir); Closed Won/Lost = arsip read-only (terbaca, tak bisa dimutasi).
// Gerbang backend = penjaga sesungguhnya (helper quotableStage), ditolak lewat PRG
// ?err=quote_stage. Koneksi test = superuser (bypass RLS) → uji LOGIKA gate; peran
// admin (ScopeAll) memisahkan gate stage dari gate F3/izin.

// nonQuotableStages = di luar jendela quoting: dini (Prospecting) + terminal.
var nonQuotableStages = []string{"Prospecting", "Closed Won", "Closed Lost"}

// quotingWindow = jendela quoting penuh (4 tahap) tempat mutasi diizinkan.
var quotingWindow = []string{"Qualification", "Demo", "Proposal", "Negotiation"}

// TestQuotes_StageGate_CreateBlocked: QuoteCreate di stage non-quotable ditolak
// (?err=quote_stage) dan TAK menyimpan quote. Menjaga gate create nyata, bukan
// hanya UI (payload sah, hanya stage yang salah).
func TestQuotes_StageGate_CreateBlocked(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)

	for _, stage := range nonQuotableStages {
		deal := env.seedDeal(t, acc.ID, &uid)
		env.setDealStage(t, deal.ID, stage)

		form := url.Values{"quote_name": {"Penawaran"}}
		req := quotesReq(http.MethodPost, quoteListSub(deal.ID), form, itoa(deal.ID), "", "")
		rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=quote_stage") {
			t.Errorf("stage %q: create harus ?err=quote_stage, got %q (status %d)", stage, loc, rec.Code)
		}
		if n := len(env.dealQuotes(t, deal.ID)); n != 0 {
			t.Errorf("stage %q: quote tak boleh tersimpan saat diblokir, ada %d", stage, n)
		}
	}
}

// TestQuotes_StageGate_CreateAllowed: QuoteCreate SUKSES (?ok=created) di keempat
// tahap jendela quoting — gate tak boleh terlalu ketat.
func TestQuotes_StageGate_CreateAllowed(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)

	for _, stage := range quotingWindow {
		deal := env.seedDeal(t, acc.ID, &uid)
		env.setDealStage(t, deal.ID, stage)

		form := url.Values{"quote_name": {"Penawaran"}}
		req := quotesReq(http.MethodPost, quoteListSub(deal.ID), form, itoa(deal.ID), "", "")
		rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
			t.Errorf("stage %q: create harus ?ok=created, got %q (status %d)\n%s",
				stage, loc, rec.Code, rec.Body.String())
		}
		if n := len(env.dealQuotes(t, deal.ID)); n != 1 {
			t.Errorf("stage %q: quote harus tersimpan (1), ada %d", stage, n)
		}
	}
}

// TestQuotes_StageGate_ItemAddBlocked: QuoteItemAdd di stage non-quotable ditolak
// (?err=quote_stage) & TAK menambah item — arsip (Closed) & deal dini (Prospecting)
// tak boleh disunting isinya. Quote & plan di-seed langsung (bypass gate seed).
func TestQuotes_StageGate_ItemAddBlocked(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	plan := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")

	for _, stage := range nonQuotableStages {
		deal := env.seedDeal(t, acc.ID, &uid)
		env.setDealStage(t, deal.ID, stage)
		q := env.seedQuote(t, deal.ID, acc.ID, "0")

		rec := env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1")
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=quote_stage") {
			t.Errorf("stage %q: add item harus ?err=quote_stage, got %q (status %d)", stage, loc, rec.Code)
		}
		if n := len(env.quoteItems(t, q.ID)); n != 0 {
			t.Errorf("stage %q: item tak boleh bertambah saat diblokir, ada %d", stage, n)
		}
	}
}

// TestQuotes_StageGate_ItemAddAllowed: QuoteItemAdd SUKSES di setiap tahap jendela
// quoting — pasangan positif ItemAddBlocked.
func TestQuotes_StageGate_ItemAddAllowed(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	plan := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")

	for _, stage := range quotingWindow {
		deal := env.seedDeal(t, acc.ID, &uid)
		env.setDealStage(t, deal.ID, stage)
		q := env.seedQuote(t, deal.ID, acc.ID, "0")

		if rec := env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1"); rec.Code != http.StatusSeeOther {
			t.Fatalf("stage %q: add item gagal: %d\n%s", stage, rec.Code, rec.Body.String())
		}
		if n := len(env.quoteItems(t, q.ID)); n != 1 {
			t.Errorf("stage %q: item harus bertambah (1), ada %d", stage, n)
		}
	}
}

// TestQuotableStage_Helper: predikat quotableStage benar untuk 7 tahap pipeline &
// stageLockMsg memberi alasan berbeda untuk dini vs terminal (bukan pesan kosong).
func TestQuotableStage_Helper(t *testing.T) {
	for _, s := range quotingWindow {
		if !quotableStage(s) {
			t.Errorf("quotableStage(%q) harus true", s)
		}
		if msg := stageLockMsg(s); msg != "" {
			t.Errorf("stageLockMsg(%q) harus kosong (tak terkunci), got %q", s, msg)
		}
	}
	for _, s := range nonQuotableStages {
		if quotableStage(s) {
			t.Errorf("quotableStage(%q) harus false", s)
		}
		if stageLockMsg(s) == "" {
			t.Errorf("stageLockMsg(%q) harus memberi alasan, got kosong", s)
		}
	}
	// Dini & terminal memberi alasan BERBEDA (bukan pesan generik yang sama).
	if stageLockMsg("Prospecting") == stageLockMsg("Closed Won") {
		t.Error("alasan Prospecting vs Closed Won harus berbeda")
	}
}
