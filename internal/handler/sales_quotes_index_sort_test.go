package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_index_sort_test.go — BL-157f: daftar Quote (global, lintas-deal)
// terurut ke-5 kolomnya via ?sort=col&dir=asc|desc, klon arsitektur BL-157a..e
// (deals/leads/accounts/contacts/subscriptions _sort_test.go). Representatif per
// pola cursor: nullable teks (code/name), NOT NULL teks via JOIN (deal), NOT NULL
// teks (status), nullable numeric (total) + fallback + F3 warisan. Mekanisme
// cursor generik (sortcursor.go) sudah teruji tuntas di 157a — tak diulang di sini.

// seedQuoteFull = varian seed quote dgn kontrol penuh atas kolom sortable
// (entity_code/quote_name/quote_status/grand_total) — seedQuote (sales_quotes_test.go)
// tak cukup fleksibel utk uji posisi NULL & status non-Draft. withCode=false ≡
// entity_code NULL; name/total "" ≡ NULL.
func (e *testEnv) seedQuoteFull(
	t *testing.T, dealID, accountID int64, withCode bool, name, status, total string,
) db.Quote {
	t.Helper()
	var codePtr *string
	if withCode {
		code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityQuote)
		if err != nil {
			t.Fatalf("generate quote code: %v", err)
		}
		codePtr = &code
	}
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	if status == "" {
		status = quoteInitialStatus
	}
	var totalNum pgtype.Numeric
	if total != "" {
		totalNum = numFrom(t, total)
	}
	q, err := e.q.CreateQuote(t.Context(), db.CreateQuoteParams{
		TenantID:    e.tenantID,
		EntityCode:  codePtr,
		DealID:      &dealID,
		AccountID:   accountID,
		QuoteName:   namePtr,
		QuoteStatus: status,
		GrandTotal:  totalNum,
		TaxAmount:   totalNum,
	})
	if err != nil {
		t.Fatalf("seed quote full: %v", err)
	}
	return q
}

func TestQuotesIndex_SortNameAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote Zebra", "", "")
	env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote Awal", "", "")
	env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote Mekar", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=name&dir=asc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Quote Awal")
	iMekar := strings.Index(body, "Quote Mekar")
	iZebra := strings.Index(body, "Quote Zebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga quote harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestQuotesIndex_SortNameDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote Zebra", "", "")
	env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote Awal", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=name&dir=desc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Quote Awal")
	iZebra := strings.Index(body, "Quote Zebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua quote harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// Nama (quote_name nullable — beda dari Deal.deal_name yang NOT NULL). asc→NULL
// akhir, desc→NULL awal.
func TestQuotesIndex_SortNameAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote NameAwal", "", "")
	env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote NameZebra", "", "")
	noName := env.seedQuoteFull(t, deal.ID, acc.ID, true, "", "", "")
	noNameCode := deref(noName.EntityCode)

	req := quotesReq(http.MethodGet, "/quotes?sort=name&dir=asc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Quote NameAwal")
	iZ := strings.Index(body, "Quote NameZebra")
	iN := strings.Index(body, noNameCode)
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut Awal < Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestQuotesIndex_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	q := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote Fallback", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=bogus", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), deref(q.EntityCode)) {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// Kode (entity_code nullable). asc→NULL akhir, desc→NULL awal.
func TestQuotesIndex_SortCodeAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	qA := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote CodeAwal", "", "")
	qZ := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote CodeZebra", "", "")
	env.seedQuoteFull(t, deal.ID, acc.ID, false, "Quote CodeNull", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=code&dir=asc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, deref(qA.EntityCode))
	iZ := strings.Index(body, deref(qZ.EntityCode))
	iN := strings.Index(body, "Quote CodeNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	// entity_code dialokasikan berurutan (QUO-001, QUO-002, ...) → Awal < Zebra.
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut kode-Awal < kode-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

func TestQuotesIndex_SortCodeDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	qA := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote CodeAwal", "", "")
	qZ := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote CodeZebra", "", "")
	env.seedQuoteFull(t, deal.ID, acc.ID, false, "Quote CodeNull", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=code&dir=desc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, deref(qA.EntityCode))
	iZ := strings.Index(body, deref(qZ.EntityCode))
	iN := strings.Index(body, "Quote CodeNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iZ && iZ < iA) {
		t.Errorf("dir=desc harus urut NULL(awal) < kode-Zebra < kode-Awal, dapat posisi %d/%d/%d", iN, iZ, iA)
	}
}

// Deal (d.deal_name via JOIN, NOT NULL — deal selalu punya nama).
func TestQuotesIndex_SortDealAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa A", &uid, nil, nil)
	accZ := env.seedAccount(t, "Desa Z", &uid, nil, nil)
	dealA, err := env.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID: env.tenantID, DealName: "Deal QDealAwal", AccountID: accA.ID,
		DealOwner: &uid, Stage: dealInitialStage, CreatedBy: &uid,
	})
	if err != nil {
		t.Fatalf("seed deal awal: %v", err)
	}
	dealZ, err := env.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID: env.tenantID, DealName: "Deal QDealZebra", AccountID: accZ.ID,
		DealOwner: &uid, Stage: dealInitialStage, CreatedBy: &uid,
	})
	if err != nil {
		t.Fatalf("seed deal zebra: %v", err)
	}
	qA := env.seedQuoteFull(t, dealA.ID, accA.ID, true, "Quote DA", "", "")
	qZ := env.seedQuoteFull(t, dealZ.ID, accZ.ID, true, "Quote DZ", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=deal&dir=asc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, deref(qA.EntityCode))
	iZ := strings.Index(body, deref(qZ.EntityCode))
	if iA < 0 || iZ < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iA < iZ) {
		t.Errorf("dir=asc harus urut Deal Awal < Deal Zebra, dapat posisi %d/%d", iA, iZ)
	}
}

// Status (quote_status NOT NULL, raw enum alfabetis — bukan urutan lifecycle).
func TestQuotesIndex_SortStatusAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	qAccepted := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote SAccepted", "Accepted", "")
	qDraft := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote SDraft", "Draft", "")
	qSent := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote SSent", "Sent", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=status&dir=asc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAcc := strings.Index(body, deref(qAccepted.EntityCode))
	iDraft := strings.Index(body, deref(qDraft.EntityCode))
	iSent := strings.Index(body, deref(qSent.EntityCode))
	if iAcc < 0 || iDraft < 0 || iSent < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iAcc < iDraft && iDraft < iSent) {
		t.Errorf("dir=asc harus urut Accepted < Draft < Sent, dapat posisi %d/%d/%d", iAcc, iDraft, iSent)
	}
}

// Grand Total (nullable numeric) + pagination representatif utk cursor numeric.
func TestQuotesIndex_SortTotalAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	qLow := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote TotalRendah", "", "1000000")
	qHigh := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote TotalTinggi", "", "9000000")
	qNull := env.seedQuoteFull(t, deal.ID, acc.ID, true, "Quote TotalNull", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=total&dir=asc", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iL := strings.Index(body, deref(qLow.EntityCode))
	iH := strings.Index(body, deref(qHigh.EntityCode))
	iN := strings.Index(body, deref(qNull.EntityCode))
	if iL < 0 || iH < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iL < iH && iH < iN) {
		t.Errorf("dir=asc harus urut Rendah < Tinggi < NULL(akhir), dapat posisi %d/%d/%d", iL, iH, iN)
	}
}

func TestQuotesIndex_SortTotalPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort Quote", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	var lastCode string
	for i := 0; i <= pageSize; i++ {
		name := "Quote TotalV" + padded(i)
		val := itoa(int64(100000 + i*1000))
		q := env.seedQuoteFull(t, deal.ID, acc.ID, true, name, "", val)
		lastCode = deref(q.EntityCode)
	}

	first := quotesReq(http.MethodGet, "/quotes?sort=total&dir=asc", nil, "", "", "")
	rec1 := env.runAccount(uid, "owner", "admin", first, env.h.QuotesIndex)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastCode) {
		t.Fatalf("Total terbesar (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/quotes")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := quotesReq(http.MethodGet, "/quotes?sort=total&dir=asc&after="+after, nil, "", "", "")
	rec2 := env.runAccount(uid, "owner", "admin", second, env.h.QuotesIndex)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastCode) {
		t.Errorf("Total terbesar harus muncul di halaman kedua — daftar masih terpotong")
	}
}

// F3 warisan dari deal tetap dihormati di jalur sort (representatif, predikat
// ownership TAK berubah antar kolom — cukup 1 test lintas kolom yang sudah ada).
func TestQuotesIndex_SortNameRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-quote-sort@x", "member", env.tenantID)
	accOwn := env.seedAccount(t, "Desa Sendiri", &uid, nil, nil)
	accLain := env.seedAccount(t, "Desa Lain", &lain.ID, nil, nil)
	dealOwn := env.seedDeal(t, accOwn.ID, &uid)
	dealLain := env.seedDeal(t, accLain.ID, &lain.ID)

	env.seedQuoteFull(t, dealOwn.ID, accOwn.ID, true, "Quote MilikSaya", "", "")
	env.seedQuoteFull(t, dealLain.ID, accLain.ID, true, "Quote MilikOrang", "", "")

	req := quotesReq(http.MethodGet, "/quotes?sort=name&dir=asc", nil, "", "", "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MilikSaya") {
		t.Errorf("sales harus melihat quote deal MILIKNYA walau sort aktif")
	}
	if strings.Contains(body, "MilikOrang") {
		t.Errorf("sort tak boleh menembus F3: sales melihat quote deal milik orang lain")
	}
}
