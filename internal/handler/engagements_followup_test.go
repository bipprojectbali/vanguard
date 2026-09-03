package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// engagements_followup_test.go — BL-30: aksi tindak-lanjut transisi status
// Engagements + FIX data-loss outcome/next_due_date. Dipisah dari
// engagements_test.go (file-health, rule 8). Menjaga:
//
//   - #1 Data-loss: aksi status-only (Skip/Plan Ulang) TAK menimpa
//     outcome/next_due_date/scheduled_at lama jadi NULL (query COALESCE).
//   - #3 Reschedule benar-benar mengubah scheduled_at; tanpa tanggal ditolak.
//   - (c) Jejak reschedule lama→baru tercatat di metadata audit.
//   - Parser murni parseEngagementStatusForm (tanpa HTTP/DB).
//
// fvFrom didefinisikan di cs_trainings_followup_test.go (paket sama).

// --- unit murni: parseEngagementStatusForm --------------------------------

// TestParseEngagementStatusForm_Validation: status wajib+enum; rescheduled
// tanpa scheduled_at ditolak; datetime tak terurai → "datetime"; field kosong →
// nil/invalid (bukan galat) agar COALESCE menjaga nilai lama.
func TestParseEngagementStatusForm_Validation(t *testing.T) {
	cases := []struct {
		name    string
		in      map[string]string
		wantErr string
	}{
		{"status kosong", map[string]string{}, "status"},
		{"status non-enum", map[string]string{"status": "cancelled"}, "status"},
		{"done polos (semua kosong)", map[string]string{"status": "done"}, ""},
		{"done dgn outcome", map[string]string{"status": "done", "outcome": "ok"}, ""},
		{"skipped polos", map[string]string{"status": "skipped"}, ""},
		{"planned polos", map[string]string{"status": "planned"}, ""},
		{"rescheduled tanpa jadwal", map[string]string{"status": "rescheduled"}, "required"},
		{"rescheduled dgn jadwal", map[string]string{"status": "rescheduled", "scheduled_at": "2026-07-20T14:00"}, ""},
		{"scheduled_at tak terurai", map[string]string{"status": "done", "scheduled_at": "bukan-tanggal"}, "datetime"},
		{"next_due_date tak terurai", map[string]string{"status": "done", "next_due_date": "bukan-tanggal"}, "required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, code := parseEngagementStatusForm(fvFrom(c.in))
			if code != c.wantErr {
				t.Errorf("errCode = %q, mau %q", code, c.wantErr)
			}
		})
	}
}

// TestParseEngagementStatusForm_FieldsParsed: field terisi terurai benar —
// outcome di-trim, scheduled_at jadi UTC valid, next_due_date valid.
func TestParseEngagementStatusForm_FieldsParsed(t *testing.T) {
	f, code := parseEngagementStatusForm(fvFrom(map[string]string{
		"status": "rescheduled", "outcome": "  Rapat tuntas  ",
		"scheduled_at": "2026-07-20T14:00", "next_due_date": "2026-08-01",
	}))
	if code != "" {
		t.Fatalf("errCode tak diharapkan: %q", code)
	}
	if f.Outcome == nil || *f.Outcome != "Rapat tuntas" {
		t.Errorf("outcome harus di-trim 'Rapat tuntas', got %v", f.Outcome)
	}
	want := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	if !f.ScheduledAt.Valid || !f.ScheduledAt.Time.Equal(want) {
		t.Errorf("scheduled_at harus %v, got %v (valid=%v)", want, f.ScheduledAt.Time, f.ScheduledAt.Valid)
	}
	if !f.NextDue.Valid {
		t.Error("next_due_date harus valid")
	}
}

// --- integrasi DB: regresi bug #1 (data-loss) -----------------------------

// TestEngagements_StatusOnlyPreservesFields: BUG #1 — setelah outcome +
// next_due_date terisi, aksi status-only "Plan Ulang" (hanya kirim status) TAK
// boleh menimpanya jadi NULL. Query COALESCE penjaganya.
func TestEngagements_StatusOnlyPreservesFields(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Preserve Eng", &uid, nil, nil)
	eng := env.seedEngagementRow(t, acc.ID, "Engagement Preserve") // scheduled 2026-06-15 10:00 UTC

	// Tegakkan keadaan awal: done + outcome + next_due_date (via query langsung).
	outcome := "Hasil pertemuan awal"
	nd := pgtype.Date{Time: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	if _, err := env.q.UpdateEngagementStatus(t.Context(), db.UpdateEngagementStatusParams{
		ID: eng.ID, Status: "done", Outcome: &outcome, NextDueDate: nd,
	}); err != nil {
		t.Fatalf("seed done+outcome+next_due: %v", err)
	}

	// Aksi status-only "Plan Ulang" — hanya kirim status, TANPA outcome/tanggal.
	reopen := url.Values{"status": {"planned"}}
	req := accountsReq(http.MethodPost, "/w/test/engagements/"+itoa(eng.ID)+"/status", reopen, itoa(eng.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementUpdateStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("Plan Ulang harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	got, err := env.q.GetEngagement(t.Context(), eng.ID)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if got.Status != "planned" {
		t.Errorf("status harus planned, got %q", got.Status)
	}
	if got.Outcome == nil || *got.Outcome != outcome {
		t.Errorf("REGRESI #1: outcome terhapus/berubah oleh aksi status-only, got %v", got.Outcome)
	}
	wantND := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !got.NextDueDate.Valid || !got.NextDueDate.Time.Equal(wantND) {
		t.Errorf("REGRESI #1: next_due_date terhapus oleh aksi status-only, got %v (valid=%v)", got.NextDueDate.Time, got.NextDueDate.Valid)
	}
	wantSched := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	if !got.ScheduledAt.Time.Equal(wantSched) {
		t.Errorf("REGRESI #1: scheduled_at berubah tanpa reschedule, got %v", got.ScheduledAt.Time)
	}
}

// TestEngagements_RescheduleUpdatesScheduledAt: #3 — Reschedule membawa jadwal
// baru → scheduled_at berubah; tanpa jadwal → err=required + tak berubah.
func TestEngagements_RescheduleUpdatesScheduledAt(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Reschedule Eng", &uid, nil, nil)
	eng := env.seedEngagementRow(t, acc.ID, "Engagement Reschedule") // 2026-06-15 10:00 UTC

	resc := url.Values{"status": {"rescheduled"}, "scheduled_at": {"2026-07-20T14:00"}}
	req := accountsReq(http.MethodPost, "/w/test/engagements/"+itoa(eng.ID)+"/status", resc, itoa(eng.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementUpdateStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("Reschedule harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	got, err := env.q.GetEngagement(t.Context(), eng.ID)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	want := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	if !got.ScheduledAt.Time.Equal(want) {
		t.Errorf("scheduled_at harus %v, got %v", want, got.ScheduledAt.Time)
	}
	if got.Status != "rescheduled" {
		t.Errorf("status harus rescheduled, got %q", got.Status)
	}

	// Reschedule tanpa jadwal → ditolak, scheduled_at tak berubah.
	bad := url.Values{"status": {"rescheduled"}}
	req2 := accountsReq(http.MethodPost, "/w/test/engagements/"+itoa(eng.ID)+"/status", bad, itoa(eng.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.EngagementUpdateStatus)
	if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "required") {
		t.Errorf("Reschedule tanpa jadwal harus err=required, got %q", loc)
	}
	got2, err := env.q.GetEngagement(t.Context(), eng.ID)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if !got2.ScheduledAt.Time.Equal(want) {
		t.Errorf("scheduled_at tak boleh berubah dari reschedule invalid, got %v", got2.ScheduledAt.Time)
	}
}

// TestEngagements_RescheduleAuditTrail: (c) — Reschedule mencatat jejak
// scheduled_at_old→new di metadata audit (engagements tak punya history kolom).
func TestEngagements_RescheduleAuditTrail(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Audit Eng", &uid, nil, nil)
	eng := env.seedEngagementRow(t, acc.ID, "Engagement Audit") // 2026-06-15 10:00 UTC

	resc := url.Values{"status": {"rescheduled"}, "scheduled_at": {"2026-07-20T14:00"}}
	req := accountsReq(http.MethodPost, "/w/test/engagements/"+itoa(eng.ID)+"/status", resc, itoa(eng.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.EngagementUpdateStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("Reschedule harus 303, got %d", rec.Code)
	}

	logs, err := env.q.ListAuditLogs(t.Context(), 50)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	var found bool
	for _, l := range logs {
		if l.Action != "engagement.status_update" {
			continue
		}
		var meta map[string]string
		if err := json.Unmarshal(l.Metadata, &meta); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if meta["status"] != "rescheduled" {
			continue
		}
		if !strings.HasPrefix(meta["scheduled_at_old"], "2026-06-15T10:00") {
			t.Errorf("scheduled_at_old harus jadwal lama, got %q", meta["scheduled_at_old"])
		}
		if !strings.HasPrefix(meta["scheduled_at_new"], "2026-07-20T14:00") {
			t.Errorf("scheduled_at_new harus jadwal baru, got %q", meta["scheduled_at_new"])
		}
		found = true
	}
	if !found {
		t.Error("jejak reschedule (engagement.status_update rescheduled) tak tercatat di audit")
	}
}
