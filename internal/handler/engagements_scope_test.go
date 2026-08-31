package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// engagements_scope_test.go — dipisah dari engagements_test.go agar tiap file di
// bawah ambang file-health. Berisi F3 (ownership: CSM lihat binaan, Admin lihat
// semua), KPI sanity, dan filter status. Gerbang F2 + create/status ada di
// engagements_test.go.

func TestEngagements_F3_CSMLihatBinaan(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-csm@local", "member", 0).ID

	// Desa A: uid sebagai assigned_csm → engagement harus tampil ke uid (CSM).
	accA := env.seedAccount(t, "Desa Binaan CSM", nil, &uid, nil)
	_ = env.seedEngagementRow(t, accA.ID, "Engagement Sendiri")

	// Desa B: other sebagai assigned_csm → engagement TIDAK tampil ke uid (CSM).
	accB := env.seedAccount(t, "Desa Orang Lain", nil, &other, nil)
	_ = env.seedEngagementRow(t, accB.ID, "Engagement Orang Lain")

	req := accountsReq(http.MethodGet, "/w/test/engagements", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.EngagementsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("CSM harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Engagement Sendiri") {
		t.Errorf("CSM harus melihat engagement dari desa binaannya")
	}
	if strings.Contains(body, "Engagement Orang Lain") {
		t.Errorf("CSM TAK boleh melihat engagement desa binaan orang lain (F3 bocor)")
	}
}

// TestEngagements_F3_AdminLihatSemua: Admin (data_scope='all') melihat SEMUA
// engagement dalam workspace, termasuk dari desa yang assigned_csm-nya user lain.
func TestEngagements_F3_AdminLihatSemua(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID

	accA := env.seedAccount(t, "Desa Alpha", &uid, nil, nil)
	_ = env.seedEngagementRow(t, accA.ID, "Engagement Alpha")

	accB := env.seedAccount(t, "Desa Beta", nil, &other, nil)
	_ = env.seedEngagementRow(t, accB.ID, "Engagement Beta")

	req := accountsReq(http.MethodGet, "/w/test/engagements", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("Admin harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Engagement Alpha") {
		t.Errorf("Admin harus melihat Engagement Alpha")
	}
	if !strings.Contains(body, "Engagement Beta") {
		t.Errorf("Admin harus melihat Engagement Beta")
	}
}

// --- KPI sanity --------------------------------------------------------

// TestEngagements_KPISanity: CountEngagementKPIs mengembalikan nilai non-negatif
// yang konsisten dengan data seed. Planned=1, Done=1 setelah status update.
func TestEngagements_KPISanity(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa KPI Eng", &uid, nil, nil)

	// Seed dua engagement dengan status berbeda melalui helper seed (bypass handler).
	engA := env.seedEngagementRow(t, acc.ID, "Engagement Planned")

	// Update satu ke 'done' langsung via query.
	engB := env.seedEngagementRow(t, acc.ID, "Engagement Done")
	_, err := env.q.UpdateEngagementStatus(t.Context(), db.UpdateEngagementStatusParams{
		ID:     engB.ID,
		Status: "done",
	})
	if err != nil {
		t.Fatalf("UpdateEngagementStatus ke done: %v", err)
	}
	_ = engA // masih 'planned'

	kpis, err := env.q.CountEngagementKPIs(t.Context(), db.CountEngagementKPIsParams{
		ScopeAll: true,
		Uid:      &uid,
	})
	if err != nil {
		t.Fatalf("CountEngagementKPIs: %v", err)
	}

	if kpis.TotalCount < 2 {
		t.Errorf("total harus ≥ 2, got %d", kpis.TotalCount)
	}
	if kpis.PlannedCount < 1 {
		t.Errorf("planned harus ≥ 1, got %d", kpis.PlannedCount)
	}
	if kpis.DoneCount < 1 {
		t.Errorf("done harus ≥ 1, got %d", kpis.DoneCount)
	}
	// Sanity: total >= planned + done (bisa ada missed/due_soon).
	if kpis.TotalCount < kpis.PlannedCount+kpis.DoneCount {
		t.Errorf("total (%d) < planned+done (%d+%d) — inconsistent KPIs",
			kpis.TotalCount, kpis.PlannedCount, kpis.DoneCount)
	}
}

// --- tab filter --------------------------------------------------------

// TestEngagements_StatusFilter: ?tab=planned → hanya baris planned tampil;
// baris done tidak tampil.
func TestEngagements_StatusFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Filter Eng", &uid, nil, nil)

	_ = env.seedEngagementRow(t, acc.ID, "Engagement Planned Filter")

	eng2 := env.seedEngagementRow(t, acc.ID, "Engagement Done Filter")
	_, err := env.q.UpdateEngagementStatus(t.Context(), db.UpdateEngagementStatusParams{
		ID:     eng2.ID,
		Status: "done",
	})
	if err != nil {
		t.Fatalf("UpdateEngagementStatus: %v", err)
	}

	req := accountsReq(http.MethodGet, "/w/test/engagements?tab=planned", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("filter planned harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Engagement Planned Filter") {
		t.Errorf("tab=planned harus menampilkan engagement planned")
	}
	if strings.Contains(body, "Engagement Done Filter") {
		t.Errorf("tab=planned tidak boleh menampilkan engagement done")
	}
}
