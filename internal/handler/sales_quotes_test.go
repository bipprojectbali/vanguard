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
//   - Pajak builder (BL-14): mode Nominal (rupiah tetap) & Persentase (PPN ikut
//     subtotal); grand_total == Σ subtotal + tax setelah add/delete item & set /tax.
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

// setDealStage memindah deal ke stage tertentu (BL-13). seedDeal mulai di
// Prospecting yang DILUAR jendela quoting (mutasi quote diblokir gate), jadi test
// yang membuat/mengubah quote via handler memanggil ini dulu ke stage quotable
// (Qualification–Negotiation). Lewat UpdateDealStage: satu-satunya jalur pindah
// pipeline (bukan CreateDeal, yang mengunci stage awal).
func (e *testEnv) setDealStage(t *testing.T, dealID int64, stage string) {
	t.Helper()
	if err := e.q.UpdateDealStage(t.Context(), db.UpdateDealStageParams{
		Stage: stage, ID: dealID,
	}); err != nil {
		t.Fatalf("set deal stage %s: %v", stage, err)
	}
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

// setTax menjalankan QuoteTax (POST .../tax) sebagai admin — set mode+nilai pajak
// di builder (BL-14). form berisi tax_mode + tax_rate/tax_amount sesuai mode.
func (e *testEnv) setTax(t *testing.T, uid, dealID, quoteID int64, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := quotesReq(http.MethodPost, quoteSub(dealID, quoteID)+"/tax", form,
		itoa(dealID), itoa(quoteID), "")
	return e.runAccount(uid, "owner", "admin", req, e.h.QuoteTax)
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
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting
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

// --- pajak builder (BL-14): mode Nominal & Persentase + rekalkulasi ---------

// TestQuotes_TaxRecompute (mode NOMINAL): grand = Σsubtotal + tax tetap, dihitung
// ulang tiap item berubah. Pajak di-set via builder /tax (BL-14: pindah dari header);
// nilai rupiah tetap (materai) BERTAHAN saat item dihapus.
func TestQuotes_TaxRecompute(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting
	p1 := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")
	p2 := env.seedPlan(t, "Plan B", "PLN-B", "50000.00")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	env.addQuoteItem(t, uid, deal.ID, q.ID, p1, "1") // subtotal 100000
	env.addQuoteItem(t, uid, deal.ID, q.ID, p2, "2") // subtotal 100000
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "200000.00") {
		t.Fatalf("grand setelah 2 item = %s, want 200000.00", numericStr(got.GrandTotal))
	}

	// Set pajak Nominal 5000 via builder → grand = Σsubtotal + 5000.
	tax := url.Values{"tax_mode": {taxModeAmount}, "tax_amount": {"5000"}}
	if rec := env.setTax(t, uid, deal.ID, q.ID, tax); rec.Code != http.StatusSeeOther {
		t.Fatalf("set pajak: status %d\n%s", rec.Code, rec.Body.String())
	}
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "205000.00") || got.TaxMode != taxModeAmount {
		t.Errorf("grand setelah tax 5000 = %s (mode %q), want 205000.00 amount",
			numericStr(got.GrandTotal), got.TaxMode)
	}

	// Hapus item Plan B (100000) → grand = 100000 + 5000; tax nominal BERTAHAN.
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
		t.Errorf("grand setelah hapus item = %s, want 105000.00 (tax nominal tetap)",
			numericStr(got.GrandTotal))
	}
}

// TestQuotes_TaxPercentFollowsSubtotal (mode PERSENTASE): tax = subtotal × rate/100,
// MENGIKUTI subtotal otomatis saat item ditambah/dihapus (inti BL-14 — PPN tak perlu
// dihitung ulang manual). Rate & mode tersimpan; subtotal 0 → tax 0.
func TestQuotes_TaxPercentFollowsSubtotal(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	p1 := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")
	p2 := env.seedPlan(t, "Plan B", "PLN-B", "50000.00")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	env.addQuoteItem(t, uid, deal.ID, q.ID, p1, "1") // subtotal 100000

	// Set PPN 11% → tax 11000, grand 111000; mode & rate tersimpan.
	tax := url.Values{"tax_mode": {taxModePercent}, "tax_rate": {"11"}}
	if rec := env.setTax(t, uid, deal.ID, q.ID, tax); rec.Code != http.StatusSeeOther {
		t.Fatalf("set PPN: status %d\n%s", rec.Code, rec.Body.String())
	}
	got := env.mustGetQuote(t, q.ID)
	if got.TaxMode != taxModePercent || !numEq(got.TaxRate, "11.00") {
		t.Errorf("mode/rate tak tersimpan: mode=%q rate=%s", got.TaxMode, numericStr(got.TaxRate))
	}
	if !numEq(got.TaxAmount, "11000.00") || !numEq(got.GrandTotal, "111000.00") {
		t.Errorf("PPN 11%% dari 100000: tax=%s grand=%s, want 11000/111000",
			numericStr(got.TaxAmount), numericStr(got.GrandTotal))
	}

	// Tambah item (subtotal → 200000) → pajak IKUT naik ke 22000 tanpa set ulang.
	env.addQuoteItem(t, uid, deal.ID, q.ID, p2, "2") // +100000
	if got := env.mustGetQuote(t, q.ID); !numEq(got.TaxAmount, "22000.00") || !numEq(got.GrandTotal, "222000.00") {
		t.Errorf("tax harus ikut subtotal: tax=%s grand=%s, want 22000/222000",
			numericStr(got.TaxAmount), numericStr(got.GrandTotal))
	}

	// Hapus semua item (subtotal → 0) → tax 0, grand 0; mode persen tetap.
	for _, it := range env.quoteItems(t, q.ID) {
		dReq := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/items/"+itoa(it.ID)+"/delete",
			url.Values{}, itoa(deal.ID), itoa(q.ID), itoa(it.ID))
		env.runAccount(uid, "owner", "admin", dReq, env.h.QuoteItemDelete)
	}
	if got := env.mustGetQuote(t, q.ID); !numEq(got.TaxAmount, "0.00") || !numEq(got.GrandTotal, "0.00") {
		t.Errorf("subtotal 0 → tax/grand harus 0: tax=%s grand=%s",
			numericStr(got.TaxAmount), numericStr(got.GrandTotal))
	}
}

// TestQuotes_TaxMigratedDefaultAmount: baris lama (pra-BL-14) = mode 'amount' default
// DB dengan tax_amount rupiah tersimpan. Migrasi 00030 TAK menyentuh nilainya; item
// berikutnya direkalkulasi sebagai Nominal (rupiah utuh), bukan tiba-tiba persen.
func TestQuotes_TaxMigratedDefaultAmount(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	plan := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")
	q := env.seedQuote(t, deal.ID, acc.ID, "2000") // cermin baris lama: nominal 2000

	if q.TaxMode != taxModeAmount || !numEq(q.TaxAmount, "2000.00") {
		t.Fatalf("baris warisan harus mode amount 2000: mode=%q tax=%s",
			q.TaxMode, numericStr(q.TaxAmount))
	}
	// quoteTaxView menawarkan Nominal (bukan default persen) selama nilai tersimpan.
	if mode, _, amt, _ := quoteTaxView(q); mode != taxModeAmount || amt != "2000" {
		t.Errorf("view baris warisan: mode=%q amount=%q, want amount/2000", mode, amt)
	}

	// Tambah item → mode & nilai warisan utuh; grand = subtotal + 2000.
	env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1") // subtotal 100000
	got := env.mustGetQuote(t, q.ID)
	if got.TaxMode != taxModeAmount || !numEq(got.TaxAmount, "2000.00") {
		t.Errorf("mode/nilai warisan berubah: mode=%q tax=%s", got.TaxMode, numericStr(got.TaxAmount))
	}
	if !numEq(got.GrandTotal, "102000.00") {
		t.Errorf("grand = %s, want 102000.00", numericStr(got.GrandTotal))
	}
}

// --- status ----------------------------------------------------------------

// TestQuotes_StatusValidAndRejected: transisi sah tersimpan (ok=status); status
// liar ditolak di handler (err=status) & tak menimpa nilai tersimpan.
func TestQuotes_StatusValidAndRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting
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
