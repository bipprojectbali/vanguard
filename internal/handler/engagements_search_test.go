package handler

import (
	"net/http"
	"strings"
	"testing"
)

// engagements_search_test.go — regresi BL-6 slice 4: pencarian daftar
// Engagements (?q=). MENYEMPITKAN pada subjek & nama desa; TAK menembus F3.

func TestEngagementsList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	env.seedEngagementRow(t, accA.ID, "Kunjungan onboarding")
	accB := env.seedAccount(t, "Desa Mekarsari", &uid, nil, nil)
	env.seedEngagementRow(t, accB.ID, "Review kuartalan")

	req := accountsReq(http.MethodGet, "/w/test/engagements?q=onboarding", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Kunjungan onboarding") {
		t.Errorf("q=onboarding harus memuat engagement yang cocok")
	}
	if strings.Contains(body, "Review kuartalan") {
		t.Errorf("q=onboarding tak boleh memuat engagement yang tak cocok")
	}
}

func TestEngagementsList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)

	mine := env.seedAccount(t, "Desa Binaanku", nil, &uid, nil)
	env.seedEngagementRow(t, mine.ID, "Rahasia milikku")
	theirs := env.seedAccount(t, "Desa Binaan Lain", nil, &lain.ID, nil)
	env.seedEngagementRow(t, theirs.ID, "Rahasia orang lain")

	req := accountsReq(http.MethodGet, "/w/test/engagements?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.EngagementsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rahasia milikku") {
		t.Errorf("csm harus melihat engagement desa binaannya yang cocok")
	}
	if strings.Contains(body, "Rahasia orang lain") {
		t.Errorf("pencarian tak boleh menembus F3: csm melihat engagement desa orang lain")
	}
}
