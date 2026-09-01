package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_quotes_scope_test.go — Quote Builder (Modul 4 Sales): F3 warisan,
// integritas nest, soft/hard-delete, & create. Dipecah dari sales_quotes_test.go
// (snapshot/pajak/status) untuk file health — helper seed (seedDeal/seedQuote/
// seedPlan/addQuoteItem/mustGetQuote/dealQuotes/quotesReq) tetap di sana, dipakai
// bersama (satu package).
//
//   - F3 warisan: non-owner (scope IsOwn) akses quote milik deal owner lain → 404.
//   - Integritas nest: quoteID dari deal lain (deal_id tak cocok) → 404.
//   - Soft-delete quote menyembunyikan dari GetQuote & daftar; item = HARD delete.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji
// terpisah di rls_test.go.

// --- F3 warisan & integritas nest ------------------------------------------

// TestQuotes_F3_OwnershipNotOwner404: quote mewarisi F3 dari deal induk. Aktor
// scope IsOwn (sales) yang BUKAN owner deal → 404 (menyangkal keberadaan). Admin
// ScopeAll atas quote yang sama → 200 (membuktikan quote memang ada).
func TestQuotes_F3_OwnershipNotOwner404(t *testing.T) {
	env, actor := setupAccounts(t)
	ownerB := env.seedMember(t, "ownerb@local", "member", 0).ID
	acc := env.seedAccount(t, "Desa B", &ownerB, nil, nil)
	deal := env.seedDeal(t, acc.ID, &ownerB) // deal milik B
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	req := quotesReq(http.MethodGet, quoteSub(deal.ID, q.ID), nil, itoa(deal.ID), itoa(q.ID), "")
	if rec := env.runAccount(actor, "member", "sales", req, env.h.QuoteDetail); rec.Code != http.StatusNotFound {
		t.Errorf("non-owner (IsOwn) harus 404, got %d", rec.Code)
	}
	if rec := env.runAccount(actor, "owner", "admin", req, env.h.QuoteDetail); rec.Code != http.StatusOK {
		t.Errorf("admin ScopeAll harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
}

// TestQuotes_DealMismatch404: quoteID sah tapi dealID URL bukan deal induknya →
// 404 (quote satu deal tak boleh terbuka lewat alamat deal lain).
func TestQuotes_DealMismatch404(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	dealA := env.seedDeal(t, acc.ID, &uid)
	dealB := env.seedDeal(t, acc.ID, &uid)
	q := env.seedQuote(t, dealA.ID, acc.ID, "0") // milik dealA

	wrong := quotesReq(http.MethodGet, quoteSub(dealB.ID, q.ID), nil, itoa(dealB.ID), itoa(q.ID), "")
	if rec := env.runAccount(uid, "owner", "admin", wrong, env.h.QuoteDetail); rec.Code != http.StatusNotFound {
		t.Errorf("deal_id tak cocok harus 404, got %d", rec.Code)
	}
	right := quotesReq(http.MethodGet, quoteSub(dealA.ID, q.ID), nil, itoa(dealA.ID), itoa(q.ID), "")
	if rec := env.runAccount(uid, "owner", "admin", right, env.h.QuoteDetail); rec.Code != http.StatusOK {
		t.Errorf("deal induk benar harus 200, got %d", rec.Code)
	}
}

// --- soft-delete & hard-delete item ----------------------------------------

// TestQuotes_SoftDeleteHidesQuote: soft-delete menyembunyikan quote dari GetQuote
// & daftar deal (ok=quote_deleted). Item TAK dihapus (hanya header disembunyikan).
func TestQuotes_SoftDeleteHidesQuote(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting
	q := env.seedQuote(t, deal.ID, acc.ID, "0")
	code := deref(q.EntityCode)

	req := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/delete", url.Values{}, itoa(deal.ID), itoa(q.ID), "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteDelete)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=quote_deleted") {
		t.Errorf("harus ok=quote_deleted, got %q (status %d)", loc, rec.Code)
	}
	if _, err := env.q.GetQuote(t.Context(), q.ID); err == nil {
		t.Error("quote ter-soft-delete tak boleh terbaca GetQuote")
	}
	env.assertAudited(t, "quote.delete")

	lReq := quotesReq(http.MethodGet, quoteListSub(deal.ID), nil, itoa(deal.ID), "", "")
	body := env.runAccount(uid, "owner", "admin", lReq, env.h.QuotesList).Body.String()
	if code != "" && strings.Contains(body, code) {
		t.Errorf("quote terhapus (%s) tak boleh muncul di daftar", code)
	}
}

// TestQuotes_ItemHardDelete: hapus item = HARD delete (baris hilang), total
// direkalkulasi (grand = tax saja bila item habis).
func TestQuotes_ItemHardDelete(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting
	plan := env.seedPlan(t, "Plan A", "PLN-A", "100000.00")
	q := env.seedQuote(t, deal.ID, acc.ID, "1000")

	env.addQuoteItem(t, uid, deal.ID, q.ID, plan, "1")
	items := env.quoteItems(t, q.ID)
	if len(items) != 1 {
		t.Fatalf("harus 1 item, ada %d", len(items))
	}
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "101000.00") {
		t.Errorf("grand dengan item = %s, want 101000.00", numericStr(got.GrandTotal))
	}

	dReq := quotesReq(http.MethodPost, quoteSub(deal.ID, q.ID)+"/items/"+itoa(items[0].ID)+"/delete",
		url.Values{}, itoa(deal.ID), itoa(q.ID), itoa(items[0].ID))
	if rec := env.runAccount(uid, "owner", "admin", dReq, env.h.QuoteItemDelete); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete item: status %d", rec.Code)
	}
	if n := len(env.quoteItems(t, q.ID)); n != 0 {
		t.Errorf("item harus HARD delete (0 baris), ada %d", n)
	}
	if got := env.mustGetQuote(t, q.ID); !numEq(got.GrandTotal, "1000.00") {
		t.Errorf("grand setelah item habis = %s, want 1000.00 (tax saja)", numericStr(got.GrandTotal))
	}
	env.assertAudited(t, "quote.item.delete")
}

// --- create ----------------------------------------------------------------

// TestQuotes_CreateInheritsDealAccount: create dari deal → account_id & deal_id
// DIWARISI dari deal induk; status awal Draft; prepared_by default = pembuat.
// BL-14: pajak TAK lagi di form create (pindah ke builder) → grand_total NULL saat
// belum ada item/pajak. Pajak negatif ditolak di endpoint /tax (err=tax, tak menyimpan).
func TestQuotes_CreateInheritsDealAccount(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting

	// Create tanpa pajak (pajak dikelola di builder, BL-14).
	form := url.Values{"quote_name": {"Penawaran Uji"}}
	req := quotesReq(http.MethodPost, quoteListSub(deal.ID), form, itoa(deal.ID), "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("create harus ok=created, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	rows := env.dealQuotes(t, deal.ID)
	if len(rows) != 1 {
		t.Fatalf("harus 1 quote, ada %d", len(rows))
	}
	q := rows[0]
	if q.DealID == nil || *q.DealID != deal.ID {
		t.Errorf("deal_id harus diwarisi (%d), got %v", deal.ID, q.DealID)
	}
	if q.AccountID != acc.ID {
		t.Errorf("account_id harus diwarisi dari deal (%d), got %d", acc.ID, q.AccountID)
	}
	if q.QuoteStatus != "Draft" {
		t.Errorf("status awal harus Draft, got %q", q.QuoteStatus)
	}
	if q.PreparedBy == nil || *q.PreparedBy != uid {
		t.Errorf("prepared_by default harus pembuat (%d), got %v", uid, q.PreparedBy)
	}
	if q.GrandTotal.Valid {
		t.Errorf("grand awal harus NULL (belum ada item/pajak), got %s", numericStr(q.GrandTotal))
	}
	env.assertAudited(t, "quote.create")

	// Pajak negatif di /tax → tolak sebelum DB (err=tax), grand tetap NULL.
	bad := url.Values{"tax_mode": {"amount"}, "tax_amount": {"-1"}}
	bRec := env.setTax(t, uid, deal.ID, q.ID, bad)
	if loc := bRec.Header().Get("Location"); !strings.Contains(loc, "err=tax") {
		t.Errorf("pajak negatif harus err=tax, got %q", loc)
	}
	if got := env.mustGetQuote(t, q.ID); got.GrandTotal.Valid {
		t.Errorf("pajak negatif tak boleh menyimpan, grand=%s", numericStr(got.GrandTotal))
	}
}

// TestQuotes_NewFormDefaultsPreparedByToActor: BL-15 — form BUAT quote (QuoteNew,
// GET) mempreselect "Disusun oleh" ke user aktif (pembuat hampir selalu penyusun),
// bukan "— Tidak ditugaskan —". Aktor = anggota tenant (ada di assignableMembers)
// → opsi dengan value id-nya ter-`selected`. Murni default view; tak menyentuh
// validasi backend (prepared_by tetap bisa diganti manual di POST).
func TestQuotes_NewFormDefaultsPreparedByToActor(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: masuk jendela quoting

	req := quotesReq(http.MethodGet, quoteListSub(deal.ID)+"/new", nil, itoa(deal.ID), "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteNew)
	if rec.Code != http.StatusOK {
		t.Fatalf("QuoteNew harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	want := `value="` + itoa(uid) + `" selected`
	if !strings.Contains(body, want) {
		t.Errorf("form buat harus preselect prepared_by = aktor (%q), tak ditemukan di body", want)
	}
}

// TestQuotes_FormUsesRefinedTermLabels: BL-16 — label field terms diperhalus
// (tetap 2 field terpisah, kolom/name tak berubah): "Termin Pembayaran" →
// "Catatan Pembayaran", "Catatan / Syarat" → "Catatan / Syarat Lainnya". Form
// buat harus merender label baru & tak lagi label lama. Atribut name tetap
// (payment_terms/notes_terms) → backend parse tak terpengaruh.
func TestQuotes_FormUsesRefinedTermLabels(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Q", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")

	req := quotesReq(http.MethodGet, quoteListSub(deal.ID)+"/new", nil, itoa(deal.ID), "", "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.QuoteNew)
	if rec.Code != http.StatusOK {
		t.Fatalf("QuoteNew harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Catatan Pembayaran", "Catatan / Syarat Lainnya"} {
		if !strings.Contains(body, want) {
			t.Errorf("form harus merender label baru %q", want)
		}
	}
	if strings.Contains(body, "Termin Pembayaran") {
		t.Errorf("label lama %q tak boleh muncul lagi", "Termin Pembayaran")
	}
	// Atribut name tetap → kontrak backend tak berubah.
	for _, name := range []string{`name="payment_terms"`, `name="notes_terms"`} {
		if !strings.Contains(body, name) {
			t.Errorf("atribut %s wajib tetap ada (name tak berubah)", name)
		}
	}
}
