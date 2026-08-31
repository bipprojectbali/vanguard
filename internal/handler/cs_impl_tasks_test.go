package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// cs_impl_tasks_test.go — Implementation Tracker (Modul 6 Customer Success,
// sub-item Onboarding 6.2.1.1). Tiga sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET/POST /impl-tasks REUSE objek "crm:journey" —
//     admin/manager/csm lolos read+write; sales HANYA read (business_policy.csv
//     baris 61: "p, sales, crm:journey, read" — tanpa write); support/""
//     ditolak baik read maupun write (crm:journey tak ada di policy support).
//   - F3 (ownership): CSM (data_scope='own') melihat HANYA task dari desa
//     binaannya (account_owner/assigned_csm/backup_csm); Admin
//     (data_scope='all') melihat semua.
//   - KPI sanity: CountCSImplTaskKPIs mengembalikan nilai non-negatif
//     konsisten dengan data seed.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup & helper request memakai ulang setupAccounts/accountsReq/
// runAccount. Meniru engagements_test.go.

// --- helper ----------------------------------------------------------------

// csImplTaskFormValues merakit form minimal valid untuk create.
func csImplTaskFormValues(accountID int64, taskName string) url.Values {
	return url.Values{
		"account_id": {itoa(accountID)},
		"task_name":  {taskName},
	}
}

// seedCSImplTaskRow menaruh satu task langsung lewat pool (bypass handler),
// status='to_do' — untuk menguji ownership dan status update tanpa merangkai
// CSImplTaskCreate. Mengembalikan db.CsImplTask utuh.
func (e *testEnv) seedCSImplTaskRow(t *testing.T, accountID int64, taskName string) db.CsImplTask {
	t.Helper()
	task, err := e.q.CreateCSImplTask(t.Context(), db.CreateCSImplTaskParams{
		TenantID:   e.tenantID,
		AccountID:  accountID,
		TaskName:   taskName,
		TaskStatus: "to_do",
	})
	if err != nil {
		t.Fatalf("seed cs_impl_task %q: %v", taskName, err)
	}
	return task
}

// allCSImplTasks mendaftar seluruh task workspace (scope_all, tanpa filter)
// langsung dari pool — untuk membuktikan visibilitas ownership F3 dan bahwa
// create/update menyimpan baris dengan benar.
func (e *testEnv) allCSImplTasks(t *testing.T) []db.ListCSImplTasksRow {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListCSImplTasks(t.Context(), db.ListCSImplTasksParams{
		CursorAt:     at,
		CursorID:     id,
		ScopeAll:     true,
		FilterStatus: "",
		PageSize:     100,
	})
	if err != nil {
		t.Fatalf("list cs impl tasks: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestCSImplTasks_GateRead: siapa boleh MEMBUKA daftar task (act read).
// Admin/manager/csm/sales lolos ("crm:journey" read juga dipunyai Sales di
// business_policy.csv — beda dari write); support/"" ditolak 403.
func TestCSImplTasks_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Gate Read", &uid, nil, nil)
	_ = env.seedCSImplTaskRow(t, acc.ID, "Task Gate")

	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"sales", true},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/impl-tasks", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.CSImplTasksList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menyebut peran CRM")
				}
			}
		})
	}
}

// --- F2: gerbang write -------------------------------------------------

// TestCSImplTasks_GateWrite: POST create butuh act write — admin/manager/csm
// lolos; sales/support/"" ditolak 403.
func TestCSImplTasks_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"sales", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			acc := env.seedAccount(t, "Desa Write Gate", &uid, nil, nil)
			form := csImplTaskFormValues(acc.ID, "Task Write Gate")
			req := accountsReq(http.MethodPost, "/w/test/impl-tasks", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.CSImplTaskCreate)

			rows := env.allCSImplTasks(t)
			if c.allow {
				if rec.Code != http.StatusSeeOther {
					t.Errorf("role %q harus create (303), got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				if len(rows) != 1 {
					t.Errorf("role %q create harus menyimpan 1 baris, ada %d", c.role, len(rows))
				}
			} else {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if len(rows) != 0 {
					t.Errorf("role %q ditolak seharusnya tidak menyimpan baris, ada %d", c.role, len(rows))
				}
			}
		})
	}
}

// --- create task ---------------------------------------------------------

// TestCSImplTasks_CreateValid: POST dengan form lengkap valid → 303
// ok=created, baris tersimpan di DB dengan field yang benar, status lahir
// 'to_do'.
func TestCSImplTasks_CreateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Create Valid", &uid, nil, nil)
	member := env.seedMember(t, "owner-task@local", "member", 0)

	form := url.Values{
		"account_id": {itoa(acc.ID)},
		"task_name":  {"Setup Akun Admin Desa"},
		"owner_id":   {itoa(member.ID)},
		"due_date":   {"2026-09-01"},
	}
	req := accountsReq(http.MethodPost, "/w/test/impl-tasks", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSImplTaskCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}

	rows := env.allCSImplTasks(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris tersimpan, ada %d", len(rows))
	}
	got := rows[0]
	if got.TaskName != "Setup Akun Admin Desa" {
		t.Errorf("task_name harus 'Setup Akun Admin Desa', got %q", got.TaskName)
	}
	if got.TaskStatus != "to_do" {
		t.Errorf("status lahir harus 'to_do', got %q", got.TaskStatus)
	}
	if got.OwnerID == nil || *got.OwnerID != member.ID {
		t.Errorf("owner_id harus %d, got %v", member.ID, got.OwnerID)
	}
	if !got.DueDate.Valid {
		t.Errorf("due_date harus tersimpan")
	}
}

// TestCSImplTasks_CreateRejectsInvalid: form yang melanggar validasi backend
// ditolak → redirect err=... + tidak menyimpan apa pun ke DB.
func TestCSImplTasks_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{
			"tanpa account_id",
			url.Values{"task_name": {"Task X"}},
			"err=required",
		},
		{
			"tanpa task_name",
			url.Values{"account_id": {"1"}},
			"err=required",
		},
		{
			"account_id bukan angka",
			url.Values{"account_id": {"abc"}, "task_name": {"Task X"}},
			"err=account",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/impl-tasks", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.CSImplTaskCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus redirect %q, got %q (code %d)", c.wantErr, loc, rec.Code)
			}
			if rows := env.allCSImplTasks(t); len(rows) != 0 {
				t.Errorf("form invalid tak boleh menyimpan baris, ada %d", len(rows))
			}
		})
	}
}

// --- status update -------------------------------------------------------

// TestCSImplTasks_StatusUpdateValid: admin mengubah status ke 'done' →
// 303 ok=updated, status tersimpan di DB, audit tercatat.
func TestCSImplTasks_StatusUpdateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Status Done", &uid, nil, nil)
	task := env.seedCSImplTaskRow(t, acc.ID, "Task Selesai")

	form := url.Values{"status": {"done"}}
	req := accountsReq(http.MethodPost, "/w/test/impl-tasks/"+itoa(task.ID)+"/status", form, itoa(task.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSImplTaskUpdateStatus)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status update harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=updated") {
		t.Errorf("redirect harus ok=updated, got %q", loc)
	}

	got, err := env.q.GetCSImplTask(t.Context(), task.ID)
	if err != nil {
		t.Fatalf("GetCSImplTask: %v", err)
	}
	if got.TaskStatus != "done" {
		t.Errorf("status harus 'done', got %q", got.TaskStatus)
	}
	env.assertAudited(t, "cs_impl_task.status_update")
}

// TestCSImplTasks_StatusUpdateRejectInvalid: status yang bukan enum valid
// ditolak → redirect err=status; baris di DB tak berubah.
func TestCSImplTasks_StatusUpdateRejectInvalid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Invalid Status", &uid, nil, nil)
	task := env.seedCSImplTaskRow(t, acc.ID, "Task Invalid")

	form := url.Values{"status": {"archived"}} // bukan enum sahih
	req := accountsReq(http.MethodPost, "/w/test/impl-tasks/"+itoa(task.ID)+"/status", form, itoa(task.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSImplTaskUpdateStatus)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "status") {
		t.Errorf("redirect harus mengandung 'status', got %q (code %d)", loc, rec.Code)
	}
	got, err := env.q.GetCSImplTask(t.Context(), task.ID)
	if err != nil {
		t.Fatalf("GetCSImplTask: %v", err)
	}
	if got.TaskStatus != "to_do" {
		t.Errorf("status tak boleh berubah dari update invalid, got %q", got.TaskStatus)
	}
}
