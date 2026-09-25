package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/session"
)

// jena_ai_tools_contacts_test.go — F2/F3/F4 tool search_contacts &
// get_contact_summary (BL-162 fase 4, jena_ai_tools_contacts.go). Pola sama
// jena_ai_tools_test.go (search_accounts/get_account_summary) — bedanya F3 di
// sini diuji lewat DESA INDUK kontak (warisan), bukan filter kontak sendiri
// (lihat contacts_fls_test.go untuk pola acuan warisan yang sama).

// --- search_contacts ---------------------------------------------------

func TestJenaSearchContacts_F2Denied(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	env.seedContact(t, acc.ID, "Budi", &uid, false)

	var out string
	env.runJena(uid, "", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchContacts(ctx, json.RawMessage(`{"query":"budi"}`))
		if err != nil {
			t.Fatalf("jenaSearchContacts: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data kontak") {
		t.Errorf("business_role kosong harus ditolak F2, got %s", out)
	}
}

func TestJenaSearchContacts_F3ScopesToOwnDesaInduk(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-sales@local", "member", 0).ID
	mine := env.seedAccount(t, "Desa Milik Saya", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Milik Orang Lain", &other, nil, nil)
	env.seedContact(t, mine.ID, "KontakSaya", &uid, false)
	env.seedContact(t, theirs.ID, "KontakOrangLain", &other, false)

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaSearchContacts(ctx, json.RawMessage(`{"query":""}`))
		if err != nil {
			t.Fatalf("jenaSearchContacts: %v", err)
		}
	})
	if !strings.Contains(out, "KontakSaya") {
		t.Errorf("hasil harus memuat kontak desa milik sendiri:\n%s", out)
	}
	if strings.Contains(out, "KontakOrangLain") {
		t.Errorf("hasil TAK BOLEH memuat kontak desa milik sales lain (F3 warisan):\n%s", out)
	}
}

// --- get_contact_summary ------------------------------------------------

// TestJenaGetContactSummary_F2Denied: jenaLoadContact menggabung F2+F3 jadi
// satu pesan "tidak ditemukan" (pola sama jenaLoadAccount/get_account_summary
// — tak ada pesan F2 terpisah untuk get_*_summary, hanya search_* yang
// membedakan; lihat jena_ai_tools_test.go, TAK ADA TestJenaGetAccountSummary_
// F2Denied). Kontak MILIK SENDIRI (F3 pasti lolos bila F2 diabaikan)
// membuktikan penolakan murni dari F2, bukan F3 yang kebetulan gagal.
func TestJenaGetContactSummary_F2Denied(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	c := env.seedContact(t, acc.ID, "Budi", &uid, false)

	var out string
	env.runJena(uid, "", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetContactSummary(ctx, json.RawMessage(itoaJSON("contact_id", c.ID)))
		if err != nil {
			t.Fatalf("jenaGetContactSummary: %v", err)
		}
	})
	if !strings.Contains(out, "kontak tidak ditemukan") {
		t.Errorf("business_role kosong harus ditolak F2 (walau kontak milik sendiri), got %s", out)
	}
}

func TestJenaGetContactSummary_F3NotFoundCrossOwner(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-sales@local", "member", 0).ID
	theirs := env.seedAccount(t, "Desa Bukan Milik Saya", &other, nil, nil)
	c := env.seedContact(t, theirs.ID, "KontakOrangLain", &other, false)

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetContactSummary(ctx, json.RawMessage(itoaJSON("contact_id", c.ID)))
		if err != nil {
			t.Fatalf("jenaGetContactSummary: %v", err)
		}
	})
	if !strings.Contains(out, "kontak tidak ditemukan") {
		t.Errorf("kontak di desa di luar cakupan Sales lain harus \"tidak ditemukan\" (F3), got %s", out)
	}
}

// TestJenaGetContactSummary_F4PhoneMaskedForSupport: Support tanpa konfigurasi
// tenant BL-107 (default) tak berhak lihat HP/WA penuh. data_scope diset
// eksplisit ke DataScopeAll agar F3 lolos & F4 murni diuji, pola sama
// TestJenaGetAccountSummary_F4BudgetMaskedForSupport.
func TestJenaGetContactSummary_F4PhoneMaskedForSupport(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kontak", &uid, nil, nil)
	c := env.seedContactPhone(t, acc.ID, "Budi", "081234567890", "081234567890", "0219876543")

	var out string
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", "owner", false,
			env.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, "support")
		session.SetBusinessDataScope(ctx, authz.DataScopeAll) // F3 lolos manual, isolasi F4
		var err error
		out, err = env.h.jenaGetContactSummary(withQueries(ctx, env.q), json.RawMessage(itoaJSON("contact_id", c.ID)))
		if err != nil {
			t.Fatalf("jenaGetContactSummary: %v", err)
		}
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if parsed["mobile_phone"] != flsHidden {
		t.Errorf("mobile_phone harus tersamar untuk Support (F4), got %v", parsed["mobile_phone"])
	}
	if parsed["whatsapp_number"] != flsHidden {
		t.Errorf("whatsapp_number harus tersamar untuk Support (F4), got %v", parsed["whatsapp_number"])
	}
	if strings.Contains(out, "081234567890") {
		t.Errorf("nomor HP/WA TAK BOLEH bocor ke Support:\n%s", out)
	}
	if parsed["office_phone"] != "0219876543" {
		t.Errorf("office_phone TAK disamarkan (bukan F4-gated di app ini), got %v", parsed["office_phone"])
	}
}

// TestJenaGetContactSummary_F4PhoneVisibleForSales: Sales default berhak
// lihat HP/WA penuh (canSeeFullPhone default pra-BL-107).
func TestJenaGetContactSummary_F4PhoneVisibleForSales(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kontak Sales", &uid, nil, nil)
	c := env.seedContactPhone(t, acc.ID, "Budi", "081234567890", "081234567890", "0219876543")

	var out string
	env.runJena(uid, "sales", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaGetContactSummary(ctx, json.RawMessage(itoaJSON("contact_id", c.ID)))
		if err != nil {
			t.Fatalf("jenaGetContactSummary: %v", err)
		}
	})
	if strings.Contains(out, flsHidden) {
		t.Errorf("HP/WA harus tampil penuh untuk Sales (F4 default), got %s", out)
	}
	if !strings.Contains(out, "081234567890") {
		t.Errorf("mobile_phone harus tampil untuk Sales, got %s", out)
	}
}
