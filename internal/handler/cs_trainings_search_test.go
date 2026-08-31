package handler

import (
	"net/http"
	"strings"
	"testing"
)

// cs_trainings_search_test.go — regresi BL-6 slice 4: pencarian daftar Training
// Schedule (?q=). MENYEMPITKAN pada topik & nama desa; TAK menembus F3.

func TestCSTrainingsList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	env.seedCSTrainingRow(t, accA.ID, "Pelatihan dasar aplikasi")
	accB := env.seedAccount(t, "Desa Mekarsari", &uid, nil, nil)
	env.seedCSTrainingRow(t, accB.ID, "Workshop lanjutan")

	req := accountsReq(http.MethodGet, "/w/test/trainings?q=dasar", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Pelatihan dasar aplikasi") {
		t.Errorf("q=dasar harus memuat training yang cocok")
	}
	if strings.Contains(body, "Workshop lanjutan") {
		t.Errorf("q=dasar tak boleh memuat training yang tak cocok")
	}
}

func TestCSTrainingsList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)

	mine := env.seedAccount(t, "Desa Binaanku", nil, &uid, nil)
	env.seedCSTrainingRow(t, mine.ID, "Rahasia milikku")
	theirs := env.seedAccount(t, "Desa Binaan Lain", nil, &lain.ID, nil)
	env.seedCSTrainingRow(t, theirs.ID, "Rahasia orang lain")

	req := accountsReq(http.MethodGet, "/w/test/trainings?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.CSTrainingsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rahasia milikku") {
		t.Errorf("csm harus melihat training desa binaannya yang cocok")
	}
	if strings.Contains(body, "Rahasia orang lain") {
		t.Errorf("pencarian tak boleh menembus F3: csm melihat training desa orang lain")
	}
}
