package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sales_quotes_index_test.go — daftar quote LINTAS-deal (GET /quotes, QuotesIndex).
// Yang dijaga khusus daftar global (di luar cakupan sales_quotes_test.go yang
// menguji builder per-deal):
//
//   - F3 warisan pada LIST: aktor scope IsOwn hanya melihat quote milik deal-nya;
//     ScopeAll melihat lintas-owner. (ListQuotes JOIN deals → deal_owner.)
//   - Baris hidup saja: quote ter-soft-delete tak muncul di daftar global.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA filter handler; isolasi RLS
// diuji terpisah. Kode entity (QUO-00x) dipakai sebagai penanda baris di HTML.

// indexBody menjalankan QuotesIndex untuk (uid, role) dan mengembalikan HTML shell.
func (e *testEnv) indexBody(t *testing.T, uid int64, wsRole, bizRole string) string {
	t.Helper()
	req := quotesReq(http.MethodGet, "/quotes", nil, "", "", "")
	rec := e.runAccount(uid, wsRole, bizRole, req, e.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("QuotesIndex status %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestQuotesIndex_F3_ScopeFiltersByDealOwner: quote mewarisi ownership F3 dari deal
// induk. Aktor scope IsOwn (sales) hanya melihat quote deal MILIKNYA; ScopeAll
// (admin) melihat quote lintas-owner. Membuktikan filter deal_owner ikut di list.
func TestQuotesIndex_F3_ScopeFiltersByDealOwner(t *testing.T) {
	env, actor := setupAccounts(t)
	ownerB := env.seedMember(t, "ownerb@local", "member", 0).ID

	// Quote milik owner lain (B).
	accB := env.seedAccount(t, "Desa B", &ownerB, nil, nil)
	dealB := env.seedDeal(t, accB.ID, &ownerB)
	qB := env.seedQuote(t, dealB.ID, accB.ID, "0")
	codeB := deref(qB.EntityCode)

	// Quote milik aktor sendiri.
	accOwn := env.seedAccount(t, "Desa Sendiri", &actor, nil, nil)
	dealOwn := env.seedDeal(t, accOwn.ID, &actor)
	qOwn := env.seedQuote(t, dealOwn.ID, accOwn.ID, "0")
	codeOwn := deref(qOwn.EntityCode)

	// IsOwn (sales): lihat punya sendiri, TIDAK punya B.
	own := env.indexBody(t, actor, "member", "sales")
	if !strings.Contains(own, codeOwn) {
		t.Errorf("IsOwn harus melihat quote sendiri (%s)", codeOwn)
	}
	if strings.Contains(own, codeB) {
		t.Errorf("IsOwn tak boleh melihat quote owner lain (%s)", codeB)
	}

	// ScopeAll (admin): lihat keduanya.
	all := env.indexBody(t, actor, "owner", "admin")
	if !strings.Contains(all, codeOwn) || !strings.Contains(all, codeB) {
		t.Errorf("ScopeAll harus melihat kedua quote (%s, %s)", codeOwn, codeB)
	}
}

// TestQuotesIndex_SoftDeletedHidden: quote ter-soft-delete disaring dari daftar
// global (ListQuotes q.deleted_at IS NULL) — baris hidup tetap tampil.
func TestQuotesIndex_SoftDeletedHidden(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	live := env.seedQuote(t, deal.ID, acc.ID, "0")
	gone := env.seedQuote(t, deal.ID, acc.ID, "0")

	if err := env.q.SoftDeleteQuote(t.Context(), db.SoftDeleteQuoteParams{
		ID: gone.ID, UpdatedBy: &uid,
	}); err != nil {
		t.Fatalf("soft delete quote: %v", err)
	}

	body := env.indexBody(t, uid, "owner", "admin")
	if !strings.Contains(body, deref(live.EntityCode)) {
		t.Errorf("quote hidup harus tampil (%s)", deref(live.EntityCode))
	}
	if strings.Contains(body, deref(gone.EntityCode)) {
		t.Errorf("quote terhapus tak boleh tampil (%s)", deref(gone.EntityCode))
	}
}
