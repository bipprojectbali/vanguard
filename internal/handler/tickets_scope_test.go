package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// tickets_scope_test.go — bagian F3 (ownership) & KPI dari tickets_test.go,
// dipisah agar tiap file di bawah ambang tipe Test (400). Setup, gerbang F2,
// create/SLA, dan helper seed tetap di tickets_test.go (paket sama).

// --- F3: ownership tickets ---------------------------------------------

// TestTickets_F3_CSMLihatBinaan: CSM melihat HANYA tiket desa yang ia bina
// (assigned_csm). Tiket desa binaan orang lain tidak tampil di daftar.
func TestTickets_F3_CSMLihatBinaan(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID

	// Desa A: uid sebagai assignedCSM → tiket harus tampil ke uid sebagai CSM.
	accA := env.seedAccount(t, "Desa Binaan CSM", nil, &uid, nil)
	_ = env.seedTicketRow(t, accA.ID, "Tiket Desa Binaan")

	// Desa B: other sebagai assignedCSM → tiket TIDAK tampil ke uid sebagai CSM.
	accB := env.seedAccount(t, "Desa Orang Lain", nil, &other, nil)
	_ = env.seedTicketRow(t, accB.ID, "Tiket Desa Lain")

	req := accountsReq(http.MethodGet, "/w/test/tickets", nil, "")
	body := env.runAccount(uid, "member", "csm", req, env.h.TicketsList).Body.String()

	if !strings.Contains(body, "Tiket Desa Binaan") {
		t.Error("CSM harus melihat tiket desa binaannya")
	}
	if strings.Contains(body, "Tiket Desa Lain") {
		t.Error("CSM TAK boleh melihat tiket desa binaan orang lain (F3 bocor)")
	}
}

// TestTickets_F3_SupportLihatSemua: Support (data_scope='none' + canWrite)
// mendapat override ScopeAll via TicketsListFilterFor — lihat semua tiket
// workspace meski bukan desa binaannya.
func TestTickets_F3_SupportLihatSemua(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "sup2@local", "member", 0).ID

	accA := env.seedAccount(t, "Desa Alpha", &other, nil, nil)
	_ = env.seedTicketRow(t, accA.ID, "Tiket Alpha")

	accB := env.seedAccount(t, "Desa Beta", nil, &other, nil)
	_ = env.seedTicketRow(t, accB.ID, "Tiket Beta")

	req := accountsReq(http.MethodGet, "/w/test/tickets", nil, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.TicketsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("support harus lolos gate (200), got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Tiket Alpha") {
		t.Error("support harus melihat Tiket Alpha (ScopeAll override)")
	}
	if !strings.Contains(body, "Tiket Beta") {
		t.Error("support harus melihat Tiket Beta (ScopeAll override)")
	}
}

// TestTickets_KPIsSanity: CountTicketKPIs mengembalikan nilai non-negatif dan
// menghitung tiket terbuka dengan benar. Setelah 2 tiket di-seed (keduanya
// status='baru', tidak di-assign), open_count harus 2, unassigned_count harus 2.
func TestTickets_KPIsSanity(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa KPI", &uid, nil, nil)
	_ = env.seedTicketRow(t, acc.ID, "Tiket KPI A")
	_ = env.seedTicketRow(t, acc.ID, "Tiket KPI B")

	kpis, err := env.q.CountTicketKPIs(t.Context(), db.CountTicketKPIsParams{
		ScopeAll: true,
		Uid:      &uid,
	})
	if err != nil {
		t.Fatalf("CountTicketKPIs: %v", err)
	}
	if kpis.OpenCount < 0 || kpis.UnassignedCount < 0 || kpis.BreachedCount < 0 {
		t.Errorf("KPIs harus non-negatif: open=%d unassigned=%d breached=%d",
			kpis.OpenCount, kpis.UnassignedCount, kpis.BreachedCount)
	}
	// Keduanya status='baru' belum di-assign → open=2, unassigned=2, breached=0.
	if kpis.OpenCount != 2 {
		t.Errorf("open_count harus 2, got %d", kpis.OpenCount)
	}
	if kpis.UnassignedCount != 2 {
		t.Errorf("unassigned_count harus 2, got %d", kpis.UnassignedCount)
	}
	if kpis.BreachedCount != 0 {
		t.Errorf("breached_count harus 0 (tanpa SLA deadline), got %d", kpis.BreachedCount)
	}
}
