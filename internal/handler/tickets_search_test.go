package handler

import (
	"net/http"
	"strings"
	"testing"
)

// tickets_search_test.go — regresi BL-6 slice 4 (Customer Success): pencarian
// daftar Tickets (?q=). Dua sifat yang WAJIB benar bersama di sisi handler:
//   - MENYEMPITKAN: ?q= menyaring baris via ILIKE contains (case-insensitive)
//     pada kolom yang TAMPIL — subjek tiket & nama desa.
//   - TAK MEMPERLUAS: pencarian berjalan DI ATAS filter kepemilikan F3 — CSM
//     ('own') yang mencari kata cocok dengan tiket desa binaan orang lain tetap
//     tak melihatnya. q menyaring di dalam cakupan, bukan menembusnya.
// (Kontrak URL kotak cari/pager diuji di view: panel/tickets_search_test.go.)

func TestTicketsList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	accA := env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	env.seedTicketRow(t, accA.ID, "Internet mati total")
	accB := env.seedAccount(t, "Desa Mekarsari", &uid, nil, nil)
	env.seedTicketRow(t, accB.ID, "Server bermasalah")

	req := accountsReq(http.MethodGet, "/w/test/tickets?q=internet", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Internet mati total") {
		t.Errorf("q=internet harus memuat tiket yang cocok (case-insensitive)")
	}
	if strings.Contains(body, "Server bermasalah") {
		t.Errorf("q=internet tak boleh memuat tiket yang tak cocok")
	}
}

func TestTicketsList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)

	// Tiket desa binaan AKTOR (assigned_csm = uid).
	mine := env.seedAccount(t, "Desa Binaanku", nil, &uid, nil)
	env.seedTicketRow(t, mine.ID, "Rahasia milikku")
	// Tiket desa binaan orang lain (assigned_csm = lain.ID).
	theirs := env.seedAccount(t, "Desa Binaan Lain", nil, &lain.ID, nil)
	env.seedTicketRow(t, theirs.ID, "Rahasia orang lain")

	req := accountsReq(http.MethodGet, "/w/test/tickets?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.TicketsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rahasia milikku") {
		t.Errorf("csm harus melihat tiket desa binaannya yang cocok pencarian")
	}
	if strings.Contains(body, "Rahasia orang lain") {
		t.Errorf("pencarian tak boleh menembus F3: csm melihat tiket desa orang lain")
	}
}
