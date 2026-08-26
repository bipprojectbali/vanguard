package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_trainings_test.go — Training Schedule (Modul 6 Customer Success,
// sub-item Onboarding 6.2.1.2). Tiga sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET/POST /trainings REUSE objek "crm:journey" —
//     admin/manager/csm lolos read+write; sales HANYA read (business_policy.csv
//     baris 61: "p, sales, crm:journey, read" — tanpa write); support/""
//     ditolak baik read maupun write (crm:journey tak ada di policy support).
//   - F3 (ownership): CSM (data_scope='own') melihat HANYA training dari desa
//     binaannya (account_owner/assigned_csm/backup_csm); Admin
//     (data_scope='all') melihat semua.
//   - KPI sanity: CountCSTrainingKPIs mengembalikan nilai non-negatif
//     konsisten dengan data seed.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup & helper request memakai ulang setupAccounts/accountsReq/
// runAccount. Meniru cs_impl_tasks_test.go / engagements_test.go.

// --- helper ----------------------------------------------------------------

// csTrainingFormValues merakit form minimal valid untuk create. training_date
// dalam format datetime-local (WAJIB — kolom NOT NULL).
func csTrainingFormValues(accountID int64, topic string) url.Values {
	return url.Values{
		"account_id":     {itoa(accountID)},
		"training_topic": {topic},
		"training_date":  {"2026-06-15T10:00"},
	}
}

// seedCSTrainingRow menaruh satu training langsung lewat pool (bypass
// handler), status='scheduled' — untuk menguji ownership dan status update
// tanpa merangkai CSTrainingCreate. Mengembalikan db.CsTraining utuh.
func (e *testEnv) seedCSTrainingRow(t *testing.T, accountID int64, topic string) db.CsTraining {
	t.Helper()
	at := pgtype.Timestamptz{
		Time:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
	tr, err := e.q.CreateCSTraining(t.Context(), db.CreateCSTrainingParams{
		TenantID:       e.tenantID,
		AccountID:      accountID,
		TrainingTopic:  topic,
		TrainingDate:   at,
		TrainingStatus: "scheduled",
	})
	if err != nil {
		t.Fatalf("seed cs_training %q: %v", topic, err)
	}
	return tr
}

// allCSTrainings mendaftar seluruh training workspace (scope_all, tanpa
// filter) langsung dari pool — untuk membuktikan visibilitas ownership F3
// dan bahwa create/update menyimpan baris dengan benar.
func (e *testEnv) allCSTrainings(t *testing.T) []db.ListCSTrainingsRow {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListCSTrainings(t.Context(), db.ListCSTrainingsParams{
		CursorAt:     at,
		CursorID:     id,
		ScopeAll:     true,
		FilterStatus: "",
		PageSize:     100,
	})
	if err != nil {
		t.Fatalf("list cs trainings: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestCSTrainings_GateRead: siapa boleh MEMBUKA daftar training (act read).
// Admin/manager/csm/sales lolos ("crm:journey" read juga dipunyai Sales di
// business_policy.csv — beda dari write); support/"" ditolak 403.
func TestCSTrainings_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Gate Read Training", &uid, nil, nil)
	_ = env.seedCSTrainingRow(t, acc.ID, "Training Gate")

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
			req := accountsReq(http.MethodGet, "/w/test/trainings", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.CSTrainingsList)
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

// TestCSTrainings_GateWrite: POST create butuh act write — admin/manager/csm
// lolos; sales/support/"" ditolak 403.
func TestCSTrainings_GateWrite(t *testing.T) {
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
			acc := env.seedAccount(t, "Desa Write Gate Training", &uid, nil, nil)
			form := csTrainingFormValues(acc.ID, "Training Write Gate")
			req := accountsReq(http.MethodPost, "/w/test/trainings", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.CSTrainingCreate)

			rows := env.allCSTrainings(t)
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

// --- create training ---------------------------------------------------

// TestCSTrainings_CreateValid: POST dengan form lengkap valid → 303
// ok=created, baris tersimpan di DB dengan field yang benar, status lahir
// 'scheduled'.
func TestCSTrainings_CreateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Create Valid Training", &uid, nil, nil)
	member := env.seedMember(t, "trainer-training@local", "member", 0)

	form := url.Values{
		"account_id":     {itoa(acc.ID)},
		"training_topic": {"Onboarding Aplikasi Kas Desa"},
		"training_date":  {"2026-07-01T09:00"},
		"trainer_id":     {itoa(member.ID)},
		"participants":   {"25"},
	}
	req := accountsReq(http.MethodPost, "/w/test/trainings", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}

	rows := env.allCSTrainings(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris tersimpan, ada %d", len(rows))
	}
	got := rows[0]
	if got.TrainingTopic != "Onboarding Aplikasi Kas Desa" {
		t.Errorf("training_topic harus 'Onboarding Aplikasi Kas Desa', got %q", got.TrainingTopic)
	}
	if got.TrainingStatus != "scheduled" {
		t.Errorf("status lahir harus 'scheduled', got %q", got.TrainingStatus)
	}
	if got.TrainerID == nil || *got.TrainerID != member.ID {
		t.Errorf("trainer_id harus %d, got %v", member.ID, got.TrainerID)
	}
	if got.Participants == nil || *got.Participants != 25 {
		t.Errorf("participants harus 25, got %v", got.Participants)
	}
	if !got.TrainingDate.Valid {
		t.Errorf("training_date harus tersimpan")
	}
}

// TestCSTrainings_CreateRejectsInvalid: form yang melanggar validasi backend
// ditolak → redirect err=... + tidak menyimpan apa pun ke DB.
func TestCSTrainings_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{
			"tanpa account_id",
			url.Values{"training_topic": {"Training X"}, "training_date": {"2026-06-15T10:00"}},
			"err=required",
		},
		{
			"tanpa training_topic",
			url.Values{"account_id": {"1"}, "training_date": {"2026-06-15T10:00"}},
			"err=required",
		},
		{
			"tanpa training_date",
			url.Values{"account_id": {"1"}, "training_topic": {"Training X"}},
			"err=required",
		},
		{
			"account_id bukan angka",
			url.Values{"account_id": {"abc"}, "training_topic": {"Training X"}, "training_date": {"2026-06-15T10:00"}},
			"err=account",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/trainings", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus redirect %q, got %q (code %d)", c.wantErr, loc, rec.Code)
			}
			if rows := env.allCSTrainings(t); len(rows) != 0 {
				t.Errorf("form invalid tak boleh menyimpan baris, ada %d", len(rows))
			}
		})
	}
}

// --- status update -------------------------------------------------------

// TestCSTrainings_StatusUpdateValid: admin mengubah status ke 'completed' +
// attendance + participants → 303 ok=updated, field tersimpan di DB, audit
// tercatat.
func TestCSTrainings_StatusUpdateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Status Completed", &uid, nil, nil)
	tr := env.seedCSTrainingRow(t, acc.ID, "Training Selesai")

	form := url.Values{
		"status":       {"completed"},
		"attendance":   {"87.5"},
		"participants": {"30"},
	}
	req := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", form, itoa(tr.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingUpdateStatus)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status update harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=updated") {
		t.Errorf("redirect harus ok=updated, got %q", loc)
	}

	got, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	if got.TrainingStatus != "completed" {
		t.Errorf("status harus 'completed', got %q", got.TrainingStatus)
	}
	if !got.Attendance.Valid {
		t.Errorf("attendance harus tersimpan")
	}
	if got.Participants == nil || *got.Participants != 30 {
		t.Errorf("participants harus 30, got %v", got.Participants)
	}
	env.assertAudited(t, "cs_training.status_update")
}

// TestCSTrainings_StatusUpdateRejectInvalid: status yang bukan enum valid
// ditolak → redirect err=status; baris di DB tak berubah.
func TestCSTrainings_StatusUpdateRejectInvalid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Invalid Status Training", &uid, nil, nil)
	tr := env.seedCSTrainingRow(t, acc.ID, "Training Invalid")

	form := url.Values{"status": {"archived"}} // bukan enum sahih
	req := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", form, itoa(tr.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingUpdateStatus)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "status") {
		t.Errorf("redirect harus mengandung 'status', got %q (code %d)", loc, rec.Code)
	}
	got, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	if got.TrainingStatus != "scheduled" {
		t.Errorf("status tak boleh berubah dari update invalid, got %q", got.TrainingStatus)
	}
}

// TestCSTrainings_StatusUpdateRejectInvalidAttendance: attendance yang bukan
// angka ditolak → redirect err=attendance; baris di DB tak berubah.
func TestCSTrainings_StatusUpdateRejectInvalidAttendance(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Invalid Attendance", &uid, nil, nil)
	tr := env.seedCSTrainingRow(t, acc.ID, "Training Attendance Invalid")

	form := url.Values{"status": {"completed"}, "attendance": {"abc"}}
	req := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", form, itoa(tr.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingUpdateStatus)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "attendance") {
		t.Errorf("redirect harus mengandung 'attendance', got %q (code %d)", loc, rec.Code)
	}
	got, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	if got.TrainingStatus != "scheduled" {
		t.Errorf("status tak boleh berubah dari update invalid, got %q", got.TrainingStatus)
	}
}

// F3 (ownership), KPI sanity, dan tab filter dipindah ke
// cs_trainings_ownership_test.go (file-health split, rule 8 CLAUDE.md) —
// lihat TestCSTrainings_F3_CSMLihatBinaan, TestCSTrainings_F3_AdminLihatSemua,
// TestCSTrainings_KPISanity, TestCSTrainings_StatusFilter di sana.
