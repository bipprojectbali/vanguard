package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// cs_trainings_ownership_test.go — sisipan cs_trainings_test.go (dipecah:
// file induk sudah mendekati batas 400 baris/tipe test — rule 8 CLAUDE.md).
// Menutup F3 ownership + KPI sanity + filter tab; helper (seedCSTrainingRow,
// allCSTrainings, itoa, dst.) tetap di cs_trainings_test.go. Meniru pola
// pemisahan cs_impl_tasks_test.go bila kelak melewati batas serupa.

// --- F3: ownership -------------------------------------------------------

// TestCSTrainings_F3_CSMLihatBinaan: CSM melihat HANYA training dari desa di
// mana ia terdaftar sebagai assigned_csm/backup_csm/account_owner. Training
// desa milik user lain tidak tampil di daftar, dan status-update ditolak 404.
func TestCSTrainings_F3_CSMLihatBinaan(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-csm-training@local", "member", 0).ID

	// Desa A: uid sebagai assigned_csm → training harus tampil ke uid (CSM).
	accA := env.seedAccount(t, "Desa Binaan CSM Training", nil, &uid, nil)
	_ = env.seedCSTrainingRow(t, accA.ID, "Training Sendiri")

	// Desa B: other sebagai assigned_csm → training TIDAK tampil ke uid (CSM).
	accB := env.seedAccount(t, "Desa Orang Lain Training", nil, &other, nil)
	trB := env.seedCSTrainingRow(t, accB.ID, "Training Orang Lain")

	req := accountsReq(http.MethodGet, "/w/test/trainings", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.CSTrainingsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("CSM harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Training Sendiri") {
		t.Errorf("CSM harus melihat training dari desa binaannya")
	}
	if strings.Contains(body, "Training Orang Lain") {
		t.Errorf("CSM TAK boleh melihat training desa binaan orang lain (F3 bocor)")
	}

	// F3 juga menutup jalur tulis: CSM tak bisa update status training orang lain.
	form := url.Values{"status": {"completed"}}
	upReq := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(trB.ID)+"/status", form, itoa(trB.ID))
	upRec := env.runAccount(uid, "member", "csm", upReq, env.h.CSTrainingUpdateStatus)
	if upRec.Code != http.StatusNotFound {
		t.Errorf("CSM update status training orang lain harus 404, got %d", upRec.Code)
	}
}

// TestCSTrainings_F3_AdminLihatSemua: Admin (data_scope='all') melihat SEMUA
// training dalam workspace, termasuk dari desa yang assigned_csm-nya user lain.
func TestCSTrainings_F3_AdminLihatSemua(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-training@local", "member", 0).ID

	accA := env.seedAccount(t, "Desa Alpha Training", &uid, nil, nil)
	_ = env.seedCSTrainingRow(t, accA.ID, "Training Alpha")

	accB := env.seedAccount(t, "Desa Beta Training", nil, &other, nil)
	_ = env.seedCSTrainingRow(t, accB.ID, "Training Beta")

	req := accountsReq(http.MethodGet, "/w/test/trainings", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("Admin harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Training Alpha") {
		t.Errorf("Admin harus melihat Training Alpha")
	}
	if !strings.Contains(body, "Training Beta") {
		t.Errorf("Admin harus melihat Training Beta")
	}
}

// --- KPI sanity ------------------------------------------------------------

// TestCSTrainings_KPISanity: CountCSTrainingKPIs mengembalikan nilai
// non-negatif yang konsisten dengan data seed.
func TestCSTrainings_KPISanity(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa KPI Training", &uid, nil, nil)

	trA := env.seedCSTrainingRow(t, acc.ID, "Training Scheduled")
	trB := env.seedCSTrainingRow(t, acc.ID, "Training Completed")
	_, err := env.q.UpdateCSTrainingStatus(t.Context(), db.UpdateCSTrainingStatusParams{
		ID:             trB.ID,
		TrainingStatus: "completed",
	})
	if err != nil {
		t.Fatalf("UpdateCSTrainingStatus ke completed: %v", err)
	}
	_ = trA // masih 'scheduled'

	kpis, err := env.q.CountCSTrainingKPIs(t.Context(), db.CountCSTrainingKPIsParams{
		ScopeAll: true,
		Uid:      &uid,
	})
	if err != nil {
		t.Fatalf("CountCSTrainingKPIs: %v", err)
	}

	if kpis.TotalCount < 2 {
		t.Errorf("total harus ≥ 2, got %d", kpis.TotalCount)
	}
	if kpis.ScheduledCount < 1 {
		t.Errorf("scheduled harus ≥ 1, got %d", kpis.ScheduledCount)
	}
	if kpis.CompletedCount < 1 {
		t.Errorf("completed harus ≥ 1, got %d", kpis.CompletedCount)
	}
	if kpis.TotalCount < kpis.ScheduledCount+kpis.CompletedCount {
		t.Errorf("total (%d) < scheduled+completed (%d+%d) — inconsistent KPIs",
			kpis.TotalCount, kpis.ScheduledCount, kpis.CompletedCount)
	}
}

// --- tab filter --------------------------------------------------------

// TestCSTrainings_StatusFilter: ?tab=scheduled → hanya baris scheduled
// tampil; baris completed tidak tampil.
func TestCSTrainings_StatusFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Filter Training", &uid, nil, nil)

	_ = env.seedCSTrainingRow(t, acc.ID, "Training Scheduled Filter")

	trDone := env.seedCSTrainingRow(t, acc.ID, "Training Completed Filter")
	_, err := env.q.UpdateCSTrainingStatus(t.Context(), db.UpdateCSTrainingStatusParams{
		ID:             trDone.ID,
		TrainingStatus: "completed",
	})
	if err != nil {
		t.Fatalf("UpdateCSTrainingStatus: %v", err)
	}

	req := accountsReq(http.MethodGet, "/w/test/trainings?tab=scheduled", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingsList)

	if rec.Code != http.StatusOK {
		t.Fatalf("filter scheduled harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Training Scheduled Filter") {
		t.Errorf("tab=scheduled harus menampilkan training scheduled")
	}
	if strings.Contains(body, "Training Completed Filter") {
		t.Errorf("tab=scheduled tidak boleh menampilkan training completed")
	}
}
