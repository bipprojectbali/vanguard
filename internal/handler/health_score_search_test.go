package handler

import (
	"net/http"
	"strings"
	"testing"
)

// health_score_search_test.go — regresi BL-6 slice 4: pencarian daftar Health
// Score (?q=). Satu-satunya kolom teks yang TAMPIL adalah nama desa — itu kunci
// cari. MENYEMPITKAN pada nama desa; TAK menembus F3 (is_csm/is_sales).

func TestHealthScoreList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	suka := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	mekar := env.seedAccount(t, "Desa Mekarsari", &uid, nil, nil)
	env.makeCustomer(t, suka.ID, "Active")
	env.makeCustomer(t, mekar.ID, "Active")

	req := accountsReq(http.MethodGet, "/w/test/health-scores?q=suka", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Sukamaju") {
		t.Errorf("q=suka harus memuat desa yang cocok (case-insensitive)")
	}
	if strings.Contains(body, "Mekarsari") {
		t.Errorf("q=suka tak boleh memuat desa yang tak cocok")
	}
}

func TestHealthScoreList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "lain@x", "member", env.tenantID)

	// Desa binaan AKTOR (assigned_csm = uid) vs desa orang lain.
	ku := env.seedAccount(t, "Desa RahasiaKu", nil, &uid, nil)
	orang := env.seedAccount(t, "Desa RahasiaOrang", nil, &other.ID, nil)
	env.makeCustomer(t, ku.ID, "Active")
	env.makeCustomer(t, orang.ID, "Active")

	req := accountsReq(http.MethodGet, "/w/test/health-scores?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "RahasiaKu") {
		t.Errorf("csm harus melihat desa binaannya yang cocok pencarian")
	}
	if strings.Contains(body, "RahasiaOrang") {
		t.Errorf("pencarian tak boleh menembus F3: csm melihat desa orang lain")
	}
}
