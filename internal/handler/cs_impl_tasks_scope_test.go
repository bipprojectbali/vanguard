package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// cs_impl_tasks_scope_test.go — bagian F3 (ownership) & KPI/filter dari
// cs_impl_tasks_test.go, dipisah agar tiap file di bawah ambang tipe Test (400).
// Setup & gerbang F2 tetap di cs_impl_tasks_test.go (paket sama).

// --- F3: ownership -------------------------------------------------------

// TestCSImplTasks_F3_CSMLihatBinaan: CSM melihat HANYA task dari desa di mana
// ia terdaftar sebagai assigned_csm/backup_csm/account_owner. Task desa milik
// user lain tidak tampil di daftar, dan status-update ditolak 404.
func TestCSImplTasks_F3_CSMLihatBinaan(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-csm-task@local", "member", 0).ID

	// Desa A: uid sebagai assigned_csm → task harus tampil ke uid (CSM).
	accA := env.seedAccount(t, "Desa Binaan CSM Task", nil, &uid, nil)
	_ = env.seedCSImplTaskRow(t, accA.ID, "Task Sendiri")

	// Desa B: other sebagai assigned_csm → task TIDAK tampil ke uid (CSM).
	accB := env.seedAccount(t, "Desa Orang Lain Task", nil, &other, nil)
	taskB := env.seedCSImplTaskRow(t, accB.ID, "Task Orang Lain")

	req := accountsReq(http.MethodGet, "/w/test/impl-tasks", nil, "")
	rec := env.runAccount(uid, "member", "csm", req, env.h.CSImplTasksList)

	if rec.Code != http.StatusOK {
		t.Fatalf("CSM harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Task Sendiri") {
		t.Errorf("CSM harus melihat task dari desa binaannya")
	}
	if strings.Contains(body, "Task Orang Lain") {
		t.Errorf("CSM TAK boleh melihat task desa binaan orang lain (F3 bocor)")
	}

	// F3 juga menutup jalur tulis: CSM tak bisa update status task orang lain.
	form := url.Values{"status": {"in_progress"}}
	upReq := accountsReq(http.MethodPost, "/w/test/impl-tasks/"+itoa(taskB.ID)+"/status", form, itoa(taskB.ID))
	upRec := env.runAccount(uid, "member", "csm", upReq, env.h.CSImplTaskUpdateStatus)
	if upRec.Code != http.StatusNotFound {
		t.Errorf("CSM update status task orang lain harus 404, got %d", upRec.Code)
	}
}

// TestCSImplTasks_F3_AdminLihatSemua: Admin (data_scope='all') melihat SEMUA
// task dalam workspace, termasuk dari desa yang assigned_csm-nya user lain.
func TestCSImplTasks_F3_AdminLihatSemua(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-task@local", "member", 0).ID

	accA := env.seedAccount(t, "Desa Alpha Task", &uid, nil, nil)
	_ = env.seedCSImplTaskRow(t, accA.ID, "Task Alpha")

	accB := env.seedAccount(t, "Desa Beta Task", nil, &other, nil)
	_ = env.seedCSImplTaskRow(t, accB.ID, "Task Beta")

	req := accountsReq(http.MethodGet, "/w/test/impl-tasks", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSImplTasksList)

	if rec.Code != http.StatusOK {
		t.Fatalf("Admin harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Task Alpha") {
		t.Errorf("Admin harus melihat Task Alpha")
	}
	if !strings.Contains(body, "Task Beta") {
		t.Errorf("Admin harus melihat Task Beta")
	}
}

// --- KPI sanity ------------------------------------------------------------

// TestCSImplTasks_KPISanity: CountCSImplTaskKPIs mengembalikan nilai
// non-negatif yang konsisten dengan data seed.
func TestCSImplTasks_KPISanity(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa KPI Task", &uid, nil, nil)

	taskA := env.seedCSImplTaskRow(t, acc.ID, "Task To Do")
	taskB := env.seedCSImplTaskRow(t, acc.ID, "Task Done")
	_, err := env.q.UpdateCSImplTaskStatus(t.Context(), db.UpdateCSImplTaskStatusParams{
		ID:         taskB.ID,
		TaskStatus: "done",
	})
	if err != nil {
		t.Fatalf("UpdateCSImplTaskStatus ke done: %v", err)
	}
	_ = taskA // masih 'to_do'

	kpis, err := env.q.CountCSImplTaskKPIs(t.Context(), db.CountCSImplTaskKPIsParams{
		ScopeAll: true,
		Uid:      &uid,
	})
	if err != nil {
		t.Fatalf("CountCSImplTaskKPIs: %v", err)
	}

	if kpis.TotalCount < 2 {
		t.Errorf("total harus ≥ 2, got %d", kpis.TotalCount)
	}
	if kpis.ToDoCount < 1 {
		t.Errorf("to_do harus ≥ 1, got %d", kpis.ToDoCount)
	}
	if kpis.DoneCount < 1 {
		t.Errorf("done harus ≥ 1, got %d", kpis.DoneCount)
	}
	if kpis.TotalCount < kpis.ToDoCount+kpis.DoneCount {
		t.Errorf("total (%d) < to_do+done (%d+%d) — inconsistent KPIs",
			kpis.TotalCount, kpis.ToDoCount, kpis.DoneCount)
	}
}

// --- tab filter --------------------------------------------------------

// TestCSImplTasks_StatusFilter: ?tab=to_do → hanya baris to_do tampil; baris
// done tidak tampil.
func TestCSImplTasks_StatusFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Filter Task", &uid, nil, nil)

	_ = env.seedCSImplTaskRow(t, acc.ID, "Task To Do Filter")

	taskDone := env.seedCSImplTaskRow(t, acc.ID, "Task Done Filter")
	_, err := env.q.UpdateCSImplTaskStatus(t.Context(), db.UpdateCSImplTaskStatusParams{
		ID:         taskDone.ID,
		TaskStatus: "done",
	})
	if err != nil {
		t.Fatalf("UpdateCSImplTaskStatus: %v", err)
	}

	req := accountsReq(http.MethodGet, "/w/test/impl-tasks?tab=to_do", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSImplTasksList)

	if rec.Code != http.StatusOK {
		t.Fatalf("filter to_do harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Task To Do Filter") {
		t.Errorf("tab=to_do harus menampilkan task to_do")
	}
	if strings.Contains(body, "Task Done Filter") {
		t.Errorf("tab=to_do tidak boleh menampilkan task done")
	}
}
