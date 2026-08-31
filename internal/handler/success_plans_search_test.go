package handler

import (
	"net/http"
	"strings"
	"testing"
)

// success_plans_search_test.go — regresi BL-6 slice 4: pencarian daftar Success
// Plans (?q=). MENYEMPITKAN pada nama plan & nama desa; TAK menembus F3.

func TestSuccessPlansList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _ = env.seedSuccessPlan(t, "Desa Sukamaju", "Adopsi Penuh", nil)
	_, _ = env.seedSuccessPlan(t, "Desa Mekarsari", "Retensi Tahunan", nil)

	req := accountsReq(http.MethodGet, "/w/test/success-plans?q=adopsi", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SuccessPlansList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Adopsi Penuh") {
		t.Errorf("q=adopsi harus memuat plan yang cocok (nama plan)")
	}
	if strings.Contains(body, "Retensi Tahunan") {
		t.Errorf("q=adopsi tak boleh memuat plan yang tak cocok")
	}
}

func TestSuccessPlansList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)

	// Plan desa binaan AKTOR (assigned_csm = uid).
	_, _ = env.seedSuccessPlan(t, "Desa Binaanku", "Rahasia Milikku", &uid)
	// Plan desa binaan orang lain (assigned_csm = lain.ID).
	_, _ = env.seedSuccessPlan(t, "Desa Binaan Lain", "Rahasia Orang Lain", &lain.ID)

	req := accountsReq(http.MethodGet, "/w/test/success-plans?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.SuccessPlansList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rahasia Milikku") {
		t.Errorf("csm harus melihat plan desa binaannya yang cocok")
	}
	if strings.Contains(body, "Rahasia Orang Lain") {
		t.Errorf("pencarian tak boleh menembus F3: csm melihat plan desa orang lain")
	}
}
