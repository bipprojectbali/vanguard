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

// engagements_test.go — Engagements / Check-ins (Modul 6 slice 6.5, wireframe 6.5).
// Tiga sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET /engagements butuh crm:engagements read —
//     admin/manager/csm lolos; sales/support/"" ditolak 403. POST butuh write —
//     admin/manager/csm lolos; sales/support/"" ditolak 403.
//   - F3 (ownership): CSM (data_scope='own') melihat HANYA engagement dari desa
//     binaannya; Admin (data_scope='all') melihat semua. Tidak ada override Support
//     (berbeda dengan tickets) — Support gagal F2 sebelum mencapai F3.
//   - KPI sanity: CountEngagementKPIs mengembalikan nilai non-negatif konsisten
//     dengan data seed.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup & helper request memakai ulang setupAccounts/accountsReq/
// runAccount. Meniru tickets_test.go / health_score_test.go.

// --- helper ----------------------------------------------------------------

// engagementFormValues merakit form minimal yang valid untuk create.
// scheduled_at dalam format datetime-local; engagement_type default 'touch_point'.
func engagementFormValues(accountID int64, subject string) url.Values {
	return url.Values{
		"account_id":      {itoa(accountID)},
		"subject":         {subject},
		"engagement_type": {"touch_point"},
		"scheduled_at":    {"2026-06-15T10:00"},
	}
}

// seedEngagementRow menaruh satu engagement langsung lewat pool (bypass handler),
// status='planned', type='touch_point' — untuk menguji ownership dan status update
// tanpa merangkai EngagementCreate. Mengembalikan db.Engagement utuh.
func (e *testEnv) seedEngagementRow(t *testing.T, accountID int64, subject string) db.Engagement {
	t.Helper()
	at := pgtype.Timestamptz{
		Time:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
	eng, err := e.q.CreateEngagement(t.Context(), db.CreateEngagementParams{
		TenantID:       e.tenantID,
		AccountID:      accountID,
		Subject:        subject,
		EngagementType: "touch_point",
		ScheduledAt:    at,
		Status:         "planned",
	})
	if err != nil {
		t.Fatalf("seed engagement %q: %v", subject, err)
	}
	return eng
}

// allEngagements mendaftar seluruh engagement workspace (scope_all, tanpa filter)
// langsung dari pool — untuk membuktikan visibilitas ownership F3 dan bahwa
// create/update menyimpan baris dengan benar.
func (e *testEnv) allEngagements(t *testing.T) []db.ListEngagementsRow {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListEngagements(t.Context(), db.ListEngagementsParams{
		CursorScheduledAt: at,
		CursorID:          id,
		ScopeAll:          true,
		FilterStatus:      "",
		FilterType:        "",
		PageSize:          100,
	})
	if err != nil {
		t.Fatalf("list engagements: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestEngagements_GateRead: siapa boleh MEMBUKA daftar engagements (act read).
// Admin/manager/csm lolos; sales/support/"" ditolak 403 — crm:engagements tidak
// ada di policy sales/support sama sekali.
func TestEngagements_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	// Seed akun + satu engagement agar JOIN di ListEngagements tidak error.
	acc := env.seedAccount(t, "Desa Gate Read", &uid, nil, nil)
	_ = env.seedEngagementRow(t, acc.ID, "Engagement Gate")

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
			req := accountsReq(http.MethodGet, "/w/test/engagements", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.EngagementsList)
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

// TestEngagements_GateWrite: POST create butuh act write — admin/manager/csm
// lolos (semua punya crm:engagements write); sales/support/"" ditolak 403.
func TestEngagements_GateWrite(t *testing.T) {
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
			form := engagementFormValues(acc.ID, "Engagement Write Gate")
			req := accountsReq(http.MethodPost, "/w/test/engagements", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.EngagementCreate)

			rows := env.allEngagements(t)
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

// --- create engagement -------------------------------------------------

// TestEngagements_CreateValid: POST dengan form lengkap valid → 303 ok=created,
// baris tersimpan di DB dengan field yang benar.
func TestEngagements_CreateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Create Valid", &uid, nil, nil)

	form := url.Values{
		"account_id":      {itoa(acc.ID)},
		"subject":         {"Kunjungan Bulanan"},
		"engagement_type": {"check_in"},
		"scheduled_at":    {"2026-07-01T09:30"},
		"frequency":       {"monthly"},
		"channel":         {"whatsapp"},
	}
	req := accountsReq(http.MethodPost, "/w/test/engagements", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}

	rows := env.allEngagements(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris tersimpan, ada %d", len(rows))
	}
	got := rows[0]
	if got.Subject != "Kunjungan Bulanan" {
		t.Errorf("subject harus 'Kunjungan Bulanan', got %q", got.Subject)
	}
	if got.EngagementType != "check_in" {
		t.Errorf("engagement_type harus 'check_in', got %q", got.EngagementType)
	}
	if got.Status != "planned" {
		t.Errorf("status default harus 'planned', got %q", got.Status)
	}
}

// TestEngagements_CreateRejectsInvalid: form yang melanggar validasi backend
// ditolak → redirect ke /new?err=... + tidak menyimpan apa pun ke DB.
func TestEngagements_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{
			"tanpa account_id",
			url.Values{"subject": {"Eng X"}, "engagement_type": {"touch_point"}, "scheduled_at": {"2026-06-01T10:00"}},
			"err=required",
		},
		{
			"tanpa subject",
			url.Values{"account_id": {"1"}, "engagement_type": {"touch_point"}, "scheduled_at": {"2026-06-01T10:00"}},
			"err=required",
		},
		{
			"tipe asing",
			url.Values{"account_id": {"1"}, "subject": {"Eng X"}, "engagement_type": {"zoom_call"}, "scheduled_at": {"2026-06-01T10:00"}},
			"err=type",
		},
		{
			"tanpa scheduled_at",
			url.Values{"account_id": {"1"}, "subject": {"Eng X"}, "engagement_type": {"touch_point"}},
			"err=required",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/engagements", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus redirect %q, got %q (code %d)", c.wantErr, loc, rec.Code)
			}
			if rows := env.allEngagements(t); len(rows) != 0 {
				t.Errorf("form invalid tak boleh menyimpan baris, ada %d", len(rows))
			}
		})
	}
}

// --- status update -------------------------------------------------------

// TestEngagements_StatusUpdateDone: admin mengubah status ke 'done' →
// 303 ok=updated, status tersimpan di DB.
func TestEngagements_StatusUpdateDone(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Status Done", &uid, nil, nil)
	eng := env.seedEngagementRow(t, acc.ID, "Engagement Selesai")

	form := url.Values{"status": {"done"}, "outcome": {"Rapat berjalan lancar"}}
	req := accountsReq(http.MethodPost, "/w/test/engagements/"+itoa(eng.ID)+"/status", form, itoa(eng.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementUpdateStatus)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status update harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=updated") {
		t.Errorf("redirect harus ok=updated, got %q", loc)
	}

	// Verifikasi langsung via GetEngagement.
	got, err := env.q.GetEngagement(t.Context(), eng.ID)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if got.Status != "done" {
		t.Errorf("status harus 'done', got %q", got.Status)
	}
	env.assertAudited(t, "engagement.status_update")
}

// TestEngagements_StatusUpdateRejectInvalid: status yang bukan enum valid
// ditolak → redirect err=status; baris di DB tak berubah.
func TestEngagements_StatusUpdateRejectInvalid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Invalid Status", &uid, nil, nil)
	eng := env.seedEngagementRow(t, acc.ID, "Engagement Invalid")

	form := url.Values{"status": {"cancelled"}} // bukan enum sahih
	req := accountsReq(http.MethodPost, "/w/test/engagements/"+itoa(eng.ID)+"/status", form, itoa(eng.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementUpdateStatus)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "status") {
		t.Errorf("redirect harus mengandung 'status', got %q (code %d)", loc, rec.Code)
	}
	got, err := env.q.GetEngagement(t.Context(), eng.ID)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if got.Status != "planned" {
		t.Errorf("status tak boleh berubah dari update invalid, got %q", got.Status)
	}
}

// --- F3: ownership engagements ----------------------------------------

// TestEngagements_F3_CSMLihatBinaan: CSM melihat HANYA engagement dari desa
// di mana ia terdaftar sebagai assigned_csm/backup_csm/account_owner.
// Engagement desa milik user lain tidak tampil di daftar.
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

