package handler

import (
	"net/http"
	"strings"
	"testing"
)

// sales_quotes_detail_flash_test.go — BL-149: halaman DETAIL quote merender flash
// PRG (?err=/?ok=). Regresi bug: QuoteDetail tak pernah membaca query err/ok & view
// tak punya slot → tolakan (mis. Accept kedua quote_already_accepted) & konfirmasi
// ditelan senyap. Kini banner tampil, pola sama halaman DAFTAR quote.

// TestQuoteDetail_RendersErrFlash: GET detail dengan ?err=quote_already_accepted
// memuat teks pesan galat (bukan halaman polos yang sekadar termuat ulang).
func TestQuoteDetail_RendersErrFlash(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Flash", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	target := quoteSub(deal.ID, q.ID) + "?err=quote_already_accepted"
	req := quotesReq(http.MethodGet, target, nil, itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if want := wsErrMsg("quote_already_accepted"); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("detail quote harus memuat pesan galat %q, body:\n%s", want, rec.Body.String())
	}
}

// TestQuoteDetail_RendersOKFlash: GET detail dengan ?ok=tax_saved memuat pesan
// sukses (quotesMsg dipakai bersama halaman daftar & detail).
func TestQuoteDetail_RendersOKFlash(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Flash OK", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	target := quoteSub(deal.ID, q.ID) + "?ok=tax_saved"
	req := quotesReq(http.MethodGet, target, nil, itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if want := quotesMsg("tax_saved"); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("detail quote harus memuat pesan sukses %q, body:\n%s", want, rec.Body.String())
	}
}
