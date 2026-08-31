package handler

import (
	"net/http"
	"strings"
	"testing"
)

// subscriptions_search_test.go — regresi BL-6 slice 3: pencarian daftar Active
// Subscriptions (?q=). Kembaran sales_search_test.go untuk daftar langganan.
// Dua sifat yang WAJIB benar bersama di sisi handler:
//   - MENYEMPITKAN: ?q= menyaring baris via ILIKE contains (case-insensitive) pada
//     kolom yang TAMPIL — desa, paket, kode entitas — BUKAN pada nilai tersamar F4
//     (MRR/ARR tak pernah jadi kunci cari).
//   - TAK MEMPERLUAS: pencarian berjalan DI ATAS filter kepemilikan F3 — aktor 'own'
//     (sales/csm) yang mencari kata cocok dengan langganan milik orang lain tetap
//     tak melihatnya. q hanya menyaring di dalam cakupan, bukan menembusnya.
// (Kontrak URL kotak cari/pager diuji di view: panel/subscriptions_search_test.go.)

func TestSubscriptionsList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	// Langganan berbeda diseed pada account+plan BERBEDA (idx_subs_one_active).
	accA := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket Alfa", "PKT-A", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "100000", "1200000")

	accB := env.seedAccount(t, "Desa Mekarsari", &uid, nil, nil)
	planB := env.seedPlanRow(t, "Paket Beta", "PKT-B", "Add-on")
	env.seedSubscription(t, accB.ID, planB.ID, &uid, "Active", "200000", "2400000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?q=suka", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Sukamaju") {
		t.Errorf("q=suka harus memuat langganan desa yang cocok (case-insensitive)")
	}
	if strings.Contains(body, "Mekarsari") {
		t.Errorf("q=suka tak boleh memuat langganan yang tak cocok")
	}
}

func TestSubscriptionsList_SearchByPlanName(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa Satu", &uid, nil, nil)
	planA := env.seedPlanRow(t, "Paket Hemat", "PKT-H", "Core")
	env.seedSubscription(t, accA.ID, planA.ID, &uid, "Active", "100000", "1200000")

	accB := env.seedAccount(t, "Desa Dua", &uid, nil, nil)
	planB := env.seedPlanRow(t, "Paket Premium", "PKT-P", "Add-on")
	env.seedSubscription(t, accB.ID, planB.ID, &uid, "Active", "200000", "2400000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?q=hemat", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Paket Hemat") {
		t.Errorf("q=hemat harus memuat langganan dengan paket yang cocok")
	}
	if strings.Contains(body, "Paket Premium") {
		t.Errorf("q=hemat tak boleh memuat langganan dengan paket lain")
	}
}

func TestSubscriptionsList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)

	// Langganan MILIK aktor (subscription_owner = uid).
	mine := env.seedAccount(t, "Desa RahasiaKu", &uid, nil, nil)
	planMine := env.seedPlanRow(t, "Paket Mine", "PKT-MINE", "Core")
	env.seedSubscription(t, mine.ID, planMine.ID, &uid, "Active", "100000", "1200000")

	// Langganan MILIK orang lain (subscription_owner = lain.ID).
	theirs := env.seedAccount(t, "Desa RahasiaOrang", &lain.ID, nil, nil)
	planTheirs := env.seedPlanRow(t, "Paket Theirs", "PKT-THEIRS", "Add-on")
	env.seedSubscription(t, theirs.ID, planTheirs.ID, &lain.ID, "Active", "300000", "3600000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "RahasiaKu") {
		t.Errorf("sales harus melihat langganan MILIKNYA yang cocok pencarian")
	}
	if strings.Contains(body, "RahasiaOrang") {
		t.Errorf("pencarian tak boleh menembus F3: sales melihat langganan milik orang lain")
	}
}
