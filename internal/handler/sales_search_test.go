package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// sales_search_test.go — regresi BL-6 slice 2: pencarian daftar Sales (?q=) untuk
// leads, deals (tabel), kontak global, quotes. Kembaran accounts_search_test.go.
// Yang dijaga di sisi handler adalah dua sifat yang WAJIB benar bersama:
//   - MENYEMPITKAN: ?q= menyaring baris via ILIKE contains (case-insensitive) pada
//     nama/kode yang TAMPIL di daftar — bukan pada field tersamar (telepon/nilai).
//   - TAK MEMPERLUAS: pencarian berjalan DI ATAS filter kepemilikan F3 — tak pernah
//     memunculkan baris di luar cakupan aktor. Aktor 'own' (sales) yang mencari kata
//     yang cocok dengan baris milik orang lain tetap tak melihatnya. Ini pengaman
//     utama slice: q hanya menyaring di dalam cakupan, bukan menembusnya.
// (Kontrak URL kotak cari/pager diuji di view: panel/sales_search_test.go.)

// --- Leads -----------------------------------------------------------------

func TestLeadsList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedLeadWithPhone(t, "Lead Sukamaju", uid, "", "")
	env.seedLeadWithPhone(t, "Lead Mekarsari", uid, "", "")

	req := accountsReq(http.MethodGet, "/w/test/leads?q=suka", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Lead Sukamaju") {
		t.Errorf("q=suka harus memuat lead yang cocok (case-insensitive)")
	}
	if strings.Contains(body, "Lead Mekarsari") {
		t.Errorf("q=suka tak boleh memuat lead yang tak cocok")
	}
}

func TestLeadsList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)
	env.seedLeadWithPhone(t, "Lead Sales Rahasia", uid, "", "")     // milik aktor
	env.seedLeadWithPhone(t, "Lead Orang Rahasia", lain.ID, "", "") // milik orang lain

	req := accountsReq(http.MethodGet, "/w/test/leads?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.LeadsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Lead Sales Rahasia") {
		t.Errorf("sales harus melihat lead MILIKNYA yang cocok pencarian")
	}
	if strings.Contains(body, "Lead Orang Rahasia") {
		t.Errorf("pencarian tak boleh menembus F3: sales melihat lead milik orang lain")
	}
}

// --- Deals (tabel) ---------------------------------------------------------

func TestDealsTable_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa D", &uid, nil, nil)
	env.seedDealNamed(t, acc.ID, &uid, "Deal Sukamaju")
	env.seedDealNamed(t, acc.ID, &uid, "Deal Mekarsari")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&q=suka", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Deal Sukamaju") {
		t.Errorf("q=suka harus memuat deal yang cocok (case-insensitive)")
	}
	if strings.Contains(body, "Deal Mekarsari") {
		t.Errorf("q=suka tak boleh memuat deal yang tak cocok")
	}
}

func TestDealsTable_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)
	mine := env.seedAccount(t, "Desa Aku", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Lain", &lain.ID, nil, nil)
	env.seedDealNamed(t, mine.ID, &uid, "Deal Sales Rahasia")
	env.seedDealNamed(t, theirs.ID, &lain.ID, "Deal Orang Rahasia")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Deal Sales Rahasia") {
		t.Errorf("sales harus melihat deal MILIKNYA yang cocok pencarian")
	}
	if strings.Contains(body, "Deal Orang Rahasia") {
		t.Errorf("pencarian tak boleh menembus F3: sales melihat deal milik orang lain")
	}
}

// --- Kontak (global) -------------------------------------------------------

func TestContactsAll_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa K", &uid, nil, nil)
	env.seedContact(t, acc.ID, "Sukamaju", &uid, false)
	env.seedContact(t, acc.ID, "Mekarsari", &uid, false)

	req := contactsReq(http.MethodGet, "/w/test/contacts?q=suka", nil, "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Sukamaju") {
		t.Errorf("q=suka harus memuat kontak yang cocok (case-insensitive)")
	}
	if strings.Contains(body, "Mekarsari") {
		t.Errorf("q=suka tak boleh memuat kontak yang tak cocok")
	}
}

func TestContactsAll_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)
	mine := env.seedAccount(t, "Desa Aku", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Lain", &lain.ID, nil, nil)
	env.seedContact(t, mine.ID, "RahasiaKu", &uid, false)          // kontak desa milik aktor
	env.seedContact(t, theirs.ID, "RahasiaOrang", &lain.ID, false) // kontak desa orang lain

	req := contactsReq(http.MethodGet, "/w/test/contacts?q=Rahasia", nil, "", "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.ContactsAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "RahasiaKu") {
		t.Errorf("sales harus melihat kontak di desa MILIKNYA yang cocok")
	}
	if strings.Contains(body, "RahasiaOrang") {
		t.Errorf("pencarian tak boleh menembus F3: sales melihat kontak desa orang lain")
	}
}

// --- Quotes (global) -------------------------------------------------------

func TestQuotesIndex_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.seedQuoteNamed(t, deal.ID, acc.ID, "Quote Sukamaju")
	env.seedQuoteNamed(t, deal.ID, acc.ID, "Quote Mekarsari")

	req := quotesReq(http.MethodGet, "/quotes?q=suka", nil, "", "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Quote Sukamaju") {
		t.Errorf("q=suka harus memuat quote yang cocok (case-insensitive)")
	}
	if strings.Contains(body, "Quote Mekarsari") {
		t.Errorf("q=suka tak boleh memuat quote yang tak cocok")
	}
}

func TestQuotesIndex_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)
	mine := env.seedAccount(t, "Desa Aku", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Lain", &lain.ID, nil, nil)
	dealMine := env.seedDeal(t, mine.ID, &uid)
	dealTheirs := env.seedDeal(t, theirs.ID, &lain.ID)
	env.seedQuoteNamed(t, dealMine.ID, mine.ID, "Quote Sales Rahasia")
	env.seedQuoteNamed(t, dealTheirs.ID, theirs.ID, "Quote Orang Rahasia")

	req := quotesReq(http.MethodGet, "/quotes?q=Rahasia", nil, "", "", "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.QuotesIndex)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Quote Sales Rahasia") {
		t.Errorf("sales harus melihat quote deal MILIKNYA yang cocok")
	}
	if strings.Contains(body, "Quote Orang Rahasia") {
		t.Errorf("pencarian tak boleh menembus F3: sales melihat quote deal orang lain")
	}
}

// seedDealNamed = seedDeal dgn nama terkontrol (untuk regresi pencarian, yang
// perlu membedakan baris cocok vs tak-cocok). Varian seedDeal (nama tetap "Deal
// Uji"); stage awal & kode entitas sama.
func (e *testEnv) seedDealNamed(t *testing.T, accountID int64, owner *int64, name string) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:   e.tenantID,
		EntityCode: &code,
		DealName:   name,
		AccountID:  accountID,
		DealOwner:  owner,
		Stage:      dealInitialStage,
		CreatedBy:  owner,
	})
	if err != nil {
		t.Fatalf("seed deal %s: %v", name, err)
	}
	return d
}

// seedQuoteNamed = seedQuote dgn nama terkontrol (regresi pencarian). Varian
// seedQuote (nama tetap "Quote Uji").
func (e *testEnv) seedQuoteNamed(t *testing.T, dealID, accountID int64, name string) db.Quote {
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
		QuoteName:   ptr(name),
		QuoteStatus: quoteInitialStatus,
		GrandTotal:  numFrom(t, "0"),
		TaxAmount:   numFrom(t, "0"),
	})
	if err != nil {
		t.Fatalf("seed quote %s: %v", name, err)
	}
	return q
}
