package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/go-chi/chi/v5"
)

// sales_quotes_test.go — Quote Builder (Modul 4 Sales), inti: snapshot harga +
// pajak/rekalkulasi + status. F3/integritas-nest/soft-delete/create ada di
// sales_quotes_scope_test.go (helper seed & assert dipakai bersama, dipecah
// murni karena file health — lihat header di sana).
//
//   - SNAPSHOT harga (acceptance M4): unit_price BEKU walau plans.base_price berubah.
//   - Pajak MANUAL + rekalkulasi: grand_total == Σ subtotal + tax setelah add/update/
//     delete item & ubah tax header.
//   - Status: transisi sah tersimpan; status liar ditolak (cermin CHECK), tak menyimpan.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji
// terpisah di rls_test.go. Enforcer nyata dari setupAccounts (butuh sumbu bisnis
// crm:deals + enforcer tenant untuk renderWorkspaceShell).

// --- helper ----------------------------------------------------------------

// quotesReq membangun request dengan chi param slug + {id}(deal) + {quoteID} +
// {itemID} sesuai yang diberi (kosong = tak dipasang). Path relatif cukup: id
// diambil dari param chi, bukan diurai dari URL.
func quotesReq(method, target string, form url.Values, dealID, quoteID, itemID string) *http.Request {
	body := strings.NewReader("")
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(slugURLParam, "test") // slug dibaca handler (wsPath/wsRedirect)
	if dealID != "" {
		rctx.URLParams.Add("id", dealID)
	}
	if quoteID != "" {
		rctx.URLParams.Add("quoteID", quoteID)
	}
	if itemID != "" {
		rctx.URLParams.Add("itemID", itemID)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// seedDeal menaruh satu deal langsung (bypass handler) dengan owner tertentu —
// jangkar F3 quote (quote mewarisi kepemilikan dari deal). Stage awal Prospecting.
func (e *testEnv) seedDeal(t *testing.T, accountID int64, owner *int64) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:   e.tenantID,
		EntityCode: &code,
		DealName:   "Deal Uji",
		AccountID:  accountID,
		DealOwner:  owner,
		Stage:      dealInitialStage,
		CreatedBy:  owner,
	})
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return d
}

// seedPlan menaruh satu plan langsung lewat pool (tak ada CreatePlan sqlc — plan
// dikelola di luar modul ini). base_price = harga yang akan di-SNAPSHOT saat item
// ditambah; category 'Core' (cermin plans_category_chk).
func (e *testEnv) seedPlan(t *testing.T, name, planCode, price string) int64 {
	t.Helper()
	var id int64
	if err := e.h.Pool.QueryRow(t.Context(),
		`INSERT INTO plans (tenant_id, plan_name, plan_code, plan_category, base_price)
		 VALUES ($1,$2,$3,'Core',$4) RETURNING id`,
		e.tenantID, name, planCode, price).Scan(&id); err != nil {
		t.Fatalf("seed plan %s: %v", name, err)
	}
	return id
}

// setPlanPrice mengubah base_price plan SETELAH item dibuat — untuk membuktikan
// snapshot unit_price tak ikut berubah (harga beku).
func (e *testEnv) setPlanPrice(t *testing.T, planID int64, price string) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE plans SET base_price=$1 WHERE id=$2`, price, planID); err != nil {
		t.Fatalf("update plan price: %v", err)
	}
}

// seedQuote menaruh satu quote Draft langsung (grand = tax, belum ada item).
func (e *testEnv) seedQuote(t *testing.T, dealID, accountID int64, tax string) db.Quote {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityQuote)
	if err != nil {
		t.Fatalf("generate quote code: %v", err)
	}
	q, err := e.q.CreateQuote(t.Context(), db.CreateQuoteParams{
		TenantID:    e.tenantID,
		EntityCode:  &code,
		DealID:      &dealID,
		AccountID:   accountID,
		QuoteName:   ptr("Quote Uji"),
		QuoteStatus: quoteInitialStatus,
		GrandTotal:  numFrom(t, tax),
		TaxAmount:   numFrom(t, tax),
	})
	if err != nil {
		t.Fatalf("seed quote: %v", err)
	}
	return q
}

// quoteItems mendaftar item satu quote langsung dari pool (assert snapshot/subtotal).
func (e *testEnv) quoteItems(t *testing.T, quoteID int64) []db.QuoteItem {
	t.Helper()
	items, err := e.q.ListQuoteItems(t.Context(), quoteID)
	if err != nil {
		t.Fatalf("list quote items: %v", err)
	}
	return items
}

// addQuoteItem menjalankan QuoteItemAdd sebagai admin (ScopeAll) untuk deal/quote.
func (e *testEnv) addQuoteItem(t *testing.T, uid, dealID, quoteID, planID int64, qty string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"plan_id": {itoa(planID)}, "quantity": {qty}}
	req := quotesReq(http.MethodPost, quoteSub(dealID, quoteID)+"/items", form,
		itoa(dealID), itoa(quoteID), "")
	return e.runAccount(uid, "owner", "admin", req, e.h.QuoteItemAdd)
}

// mustGetQuote membaca quote (fatal bila hilang) — assert total/status sesudah aksi.
func (e *testEnv) mustGetQuote(t *testing.T, id int64) db.Quote {
	t.Helper()
	q, err := e.q.GetQuote(t.Context(), id)
	if err != nil {
		t.Fatalf("get quote %d: %v", id, err)
	}
	return q
}

// dealQuotes mendaftar quote satu deal langsung dari pool (assert create/soft-delete).
func (e *testEnv) dealQuotes(t *testing.T, dealID int64) []db.Quote {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListQuotesForDeal(t.Context(), db.ListQuotesForDealParams{
		DealID: &dealID, CursorCreatedAt: at, CursorID: id, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("list quotes: %v", err)
	}
	return rows
}

// --- snapshot harga --------------------------------------------------------

// TestQuotes_ItemPriceSnapshotFreeze: unit_price disalin dari base_price saat item
// dibuat & BEKU — mengubah plans.base_price sesudahnya TAK menyentuh item lama
// (acceptance M4). Subtotal & grand_total ikut snapshot.
func TestQuotes_ItemPriceSnapshotFreeze(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	plan := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	if rec := env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "2"); rec.Code != http.StatusSeeOther {
		t.Fatalf("add item: status %d\n%s", rec.Code, rec.Body.String())
	}
	items := env.quoteItems(t, q.ID)
	if len(items) != 1 {
		t.Fatalf("harus 1 item, ada %d", len(items))
	}
	if !numEq(items[0].UnitPrice, "100000.00") {
		t.Errorf("unit_price snapshot = %s, want 100000.00", numericStr(items[0].UnitPrice))
	}
	if !numEq(items[0].Subtotal, "200000.00") {
		t.Errorf("subtotal = %s, want 200000.00", numericStr(items[0].Subtotal))
	}

	// Harga plan naik 5×; item lama HARUS tetap.
	env.setPlanPrice(t, plan, "500000.00")
	items = env.quoteItems(t, q.ID)
	if !numEq(items[0].UnitPrice, "100000.00") {
		t.Errorf("SNAPSHOT BOCOR: unit_price ikut berubah jadi %s", numericStr(items[0].UnitPrice))
	}
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "200000.00") {
		t.Errorf("grand_total = %s, want 200000.00", numericStr(got.GrandTotal))
	}
	env.assertAudited(t, "quote.item.add")
}

// --- pajak manual + rekalkulasi --------------------------------------------

// TestQuotes_TaxRecompute: grand_total = Σ subtotal + tax, dihitung ulang tiap item
// berubah & tiap tax header diubah. Pajak MANUAL (bukan konstanta PPN).
func TestQuotes_TaxRecompute(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	p1 := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")
	p2 := env.seedPlan(t, "Plan B", "PLN-B", "50000.00")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	env.addQuoteItem(t, uid, deal.ID, q.ID, p1, "1") // subtotal 100000
	env.addQuoteItem(t, uid, deal.ID, q.ID, p2, "2") // subtotal 100000
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "200000.00") {
		t.Fatalf("grand setelah 2 item = %s, want 200000.00", numericStr(got.GrandTotal))
	}

	// Ubah tax header → grand = Σsubtotal + tax baru.
	form := url.Values{"quote_name": {"Quote Uji"}, "tax_amount": {"5000"}}
	req := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID), form, itoa(deal.ID), itoa(q.ID), "")
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update header: status %d\n%s", rec.Code, rec.Body.String())
	}
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "205000.00") {
		t.Errorf("grand setelah tax 5000 = %s, want 205000.00", numericStr(got.GrandTotal))
	}

	// Hapus item Plan B (100000) → grand = 100000 + 5000; tax bertahan.
	var delID int64
	for _, it := range env.quoteItems(t, q.ID) {
		if it.PlanID != nil && *it.PlanID == p2 {
			delID = it.ID
		}
	}
	dReq := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/items/"+itoa(delID)+"/delete",
		url.Values{}, itoa(deal.ID), itoa(q.ID), itoa(delID))
	if rec := env.runAccount(uid, "owner", "admin", dReq, env.h.QuoteItemDelete); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete item: status %d\n%s", rec.Code, rec.Body.String())
	}
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "105000.00") {
		t.Errorf("grand setelah hapus item = %s, want 105000.00", numericStr(got.GrandTotal))
	}
}

// --- status ----------------------------------------------------------------

// TestQuotes_StatusValidAndRejected: transisi sah tersimpan (ok=status); status
// liar ditolak di handler (err=status) & tak menimpa nilai tersimpan.
func TestQuotes_StatusValidAndRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	// Valid: Draft → Sent.
	form := url.Values{"quote_status": {"Sent"}}
	req := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/status", form, itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=status") {
		t.Errorf("status sah harus ok=status, got %q (status %d)", loc, rec.Code)
	}
	if got := env.mustGetQuote(t, q.ID); got.QuoteStatus != "Sent" {
		t.Errorf("status tak tersimpan, got %q", got.QuoteStatus)
	}
	env.assertAudited(t, "quote.status")

	// Liar: ditolak, status tetap Sent.
	bad := url.Values{"quote_status": {"Ngawur"}}
	bReq := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/status", bad, itoa(deal.ID), itoa(q.ID), "")
	bRec := env.runAccount(uid, "owner", "admin", bReq, env.h.QuoteStatus)
	if loc := bRec.Header().Get("Location"); !strings.Contains(loc, "err=status") {
		t.Errorf("status liar harus err=status, got %q", loc)
	}
	if got := env.mustGetQuote(t, q.ID); got.QuoteStatus != "Sent" {
		t.Errorf("status liar tak boleh menimpa, got %q", got.QuoteStatus)
	}
}
