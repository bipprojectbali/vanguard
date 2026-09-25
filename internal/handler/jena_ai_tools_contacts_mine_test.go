package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// jena_ai_tools_contacts_mine_test.go — F2/F3 tool list_my_contacts (BL-162
// fase 5, jena_ai_tools_contacts_mine.go). Pola sama TestJenaListMyDeals_*
// (jena_ai_tools_more_test.go), tapi F2Denied di sini HARUS pakai
// business_role KOSONG (""), bukan peran bernama seperti "support"/"csm" —
// beda dari Deals: kelima peran default (authz/business_defaults.go) SEMUA
// punya crm:contacts, jadi tak ada peran bernama yang bisa dipakai menguji
// penolakan F2 modul Kontak (lihat TestJenaSearchContacts_F2Denied, pola sama).

func TestJenaListMyContacts_F2Denied(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kontak Saya", &uid, nil, nil)
	env.seedContact(t, acc.ID, "Budi", &uid, false)

	var out string
	env.runJena(uid, "", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyContacts(ctx)
		if err != nil {
			t.Fatalf("jenaListMyContacts: %v", err)
		}
	})
	if !strings.Contains(out, "tidak berhak melihat data kontak") {
		t.Errorf("business_role kosong harus ditolak F2, got %s", out)
	}
}

func TestJenaListMyContacts_ForcesOwnOnlyEvenForManager(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-manager-contacts@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Kontak Milik Saya", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Kontak Milik Orang Lain", &other, nil, nil)
	env.seedContact(t, accMine.ID, "KontakSaya", &uid, false)
	env.seedContact(t, accOther.ID, "KontakOrangLain", &other, false)

	// Manager punya data_scope=all (ScopeAll) — tanpa pemaksaan IsSales/IsCsm,
	// ia akan melihat kontak di SEMUA desa. list_my_contacts harus tetap hanya
	// kontak di desa milik SENDIRI (account_owner/assigned_csm/backup_csm = uid).
	var out string
	env.runJena(uid, "manager", func(ctx context.Context) {
		var err error
		out, err = env.h.jenaListMyContacts(ctx)
		if err != nil {
			t.Fatalf("jenaListMyContacts: %v", err)
		}
	})

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("hasil bukan JSON valid: %v\n%s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("Manager (ScopeAll) harus tetap hanya melihat kontak desa MILIK SENDIRI lewat Jena AI, got %d baris: %s", len(results), out)
	}
	if results[0]["name"] != "KontakSaya" {
		t.Errorf("kontak yang tampil harus KontakSaya, got %v", results[0]["name"])
	}
}
