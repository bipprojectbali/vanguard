package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// cs_trainings_followup_test.go — BL-28: aksi tindak-lanjut transisi status
// Training + FIX data-loss attendance/participants. Dipisah dari
// cs_trainings_test.go (file-health, rule 8). Menjaga:
//
//   - #1 Data-loss: aksi status-only (Batal/Buka Ulang) TAK menimpa
//     attendance/participants/notes lama jadi NULL (query COALESCE).
//   - #2 Jadwal Ulang benar-benar mengubah training_date; tanpa tanggal ditolak.
//   - #3 Catatan (notes) tersimpan saat Selesai.
//   - Validasi attendance rentang 0–100.
//   - Parser murni parseCSTrainingStatusForm (tanpa HTTP/DB).

// --- unit murni: parseCSTrainingStatusForm --------------------------------

func fvFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestParseCSTrainingStatusForm_Boundary: attendance 0/100 sah; <0 & >100
// ditolak; status non-enum ditolak; rescheduled tanpa tanggal ditolak;
// field kosong → nil/invalid (bukan galat) agar COALESCE menjaga nilai lama.
func TestParseCSTrainingStatusForm_Boundary(t *testing.T) {
	cases := []struct {
		name    string
		in      map[string]string
		wantErr string
	}{
		{"status kosong", map[string]string{}, "status"},
		{"status non-enum", map[string]string{"status": "archived"}, "status"},
		{"completed polos (semua kosong)", map[string]string{"status": "completed"}, ""},
		{"attendance batas bawah 0", map[string]string{"status": "completed", "attendance": "0"}, ""},
		{"attendance batas atas 100", map[string]string{"status": "completed", "attendance": "100"}, ""},
		{"attendance negatif", map[string]string{"status": "completed", "attendance": "-1"}, "attendance"},
		{"attendance >100", map[string]string{"status": "completed", "attendance": "100.01"}, "attendance"},
		{"attendance non-angka", map[string]string{"status": "completed", "attendance": "abc"}, "attendance"},
		{"rescheduled tanpa tanggal", map[string]string{"status": "rescheduled"}, "required"},
		{"rescheduled dgn tanggal", map[string]string{"status": "rescheduled", "training_date": "2026-07-20T14:00"}, ""},
		{"tanggal tak terurai", map[string]string{"status": "completed", "training_date": "bukan-tanggal"}, "datetime"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, code := parseCSTrainingStatusForm(fvFrom(c.in))
			if code != c.wantErr {
				t.Errorf("errCode = %q, mau %q", code, c.wantErr)
			}
		})
	}
}

// TestParseCSTrainingStatusForm_FieldsParsed: field terisi terurai benar —
// participants angka, notes trim, attendance valid.
func TestParseCSTrainingStatusForm_FieldsParsed(t *testing.T) {
	f, code := parseCSTrainingStatusForm(fvFrom(map[string]string{
		"status": "completed", "attendance": "87.5", "participants": "30", "notes": "  Semua hadir  ",
	}))
	if code != "" {
		t.Fatalf("errCode tak diharapkan: %q", code)
	}
	if !f.Attendance.Valid {
		t.Error("attendance harus valid")
	}
	if f.Participants == nil || *f.Participants != 30 {
		t.Errorf("participants harus 30, got %v", f.Participants)
	}
	if f.Notes == nil || *f.Notes != "Semua hadir" {
		t.Errorf("notes harus di-trim 'Semua hadir', got %v", f.Notes)
	}
}

// --- integrasi DB: regresi bug #1 (data-loss) -----------------------------

// TestCSTrainings_StatusOnlyPreservesFields: BUG #1 — setelah Selesai mengisi
// attendance+participants+notes, aksi status-only "Buka Ulang" (hanya kirim
// status) TAK boleh menimpanya jadi NULL. Query COALESCE penjaganya.
func TestCSTrainings_StatusOnlyPreservesFields(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Preserve", &uid, nil, nil)
	tr := env.seedCSTrainingRow(t, acc.ID, "Training Preserve")

	// 1) Selesai dengan hasil lengkap.
	done := url.Values{
		"status": {"completed"}, "attendance": {"88"},
		"participants": {"20"}, "notes": {"Peserta lengkap"},
	}
	req := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", done, itoa(tr.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingUpdateStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("Selesai harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	// 2) Buka Ulang — status-only, TANPA attendance/participants/notes.
	reopen := url.Values{"status": {"scheduled"}}
	req2 := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", reopen, itoa(tr.ID))
	if rec := env.runAccount(uid, "owner", "admin", req2, env.h.CSTrainingUpdateStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("Buka Ulang harus 303, got %d", rec.Code)
	}

	got, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	if got.TrainingStatus != "scheduled" {
		t.Errorf("status harus scheduled, got %q", got.TrainingStatus)
	}
	if !got.Attendance.Valid {
		t.Error("REGRESI #1: attendance terhapus jadi NULL oleh aksi status-only")
	}
	if got.Participants == nil || *got.Participants != 20 {
		t.Errorf("REGRESI #1: participants harus tetap 20, got %v", got.Participants)
	}
	if got.Notes == nil || *got.Notes != "Peserta lengkap" {
		t.Errorf("REGRESI #1: notes harus tetap, got %v", got.Notes)
	}
}

// TestCSTrainings_RescheduleUpdatesDate: #2 — Jadwal Ulang membawa tanggal
// baru → training_date berubah; tanpa tanggal → err=required + tak berubah.
func TestCSTrainings_RescheduleUpdatesDate(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Reschedule", &uid, nil, nil)
	tr := env.seedCSTrainingRow(t, acc.ID, "Training Reschedule") // 2026-06-15 10:00 UTC

	newDate := "2026-07-20T14:00"
	resc := url.Values{"status": {"rescheduled"}, "training_date": {newDate}}
	req := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", resc, itoa(tr.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingUpdateStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("Jadwal Ulang harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	got, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	want := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	if !got.TrainingDate.Time.Equal(want) {
		t.Errorf("training_date harus %v, got %v", want, got.TrainingDate.Time)
	}
	if got.TrainingStatus != "rescheduled" {
		t.Errorf("status harus rescheduled, got %q", got.TrainingStatus)
	}

	// Jadwal Ulang tanpa tanggal → ditolak, tanggal tak berubah.
	bad := url.Values{"status": {"rescheduled"}}
	req2 := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", bad, itoa(tr.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.CSTrainingUpdateStatus)
	if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "required") {
		t.Errorf("Jadwal Ulang tanpa tanggal harus err=required, got %q", loc)
	}
	got2, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	if !got2.TrainingDate.Time.Equal(want) {
		t.Errorf("tanggal tak boleh berubah dari reschedule invalid, got %v", got2.TrainingDate.Time)
	}
}

// TestCSTrainings_CompleteSavesNotes: #3 — Selesai dengan catatan tersimpan.
func TestCSTrainings_CompleteSavesNotes(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Notes", &uid, nil, nil)
	tr := env.seedCSTrainingRow(t, acc.ID, "Training Notes")

	form := url.Values{"status": {"completed"}, "notes": {"Materi keuangan tuntas"}}
	req := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", form, itoa(tr.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingUpdateStatus); rec.Code != http.StatusSeeOther {
		t.Fatalf("Selesai harus 303, got %d", rec.Code)
	}
	got, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	if got.Notes == nil || *got.Notes != "Materi keuangan tuntas" {
		t.Errorf("notes harus tersimpan, got %v", got.Notes)
	}
}

// TestCSTrainings_AttendanceRangeRejected: attendance di luar 0–100 ditolak
// (err=attendance), baris tak berubah.
func TestCSTrainings_AttendanceRangeRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Attendance Range", &uid, nil, nil)
	tr := env.seedCSTrainingRow(t, acc.ID, "Training Range")

	form := url.Values{"status": {"completed"}, "attendance": {"150"}}
	req := accountsReq(http.MethodPost, "/w/test/trainings/"+itoa(tr.ID)+"/status", form, itoa(tr.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.CSTrainingUpdateStatus)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "attendance") {
		t.Errorf("attendance >100 harus err=attendance, got %q", loc)
	}
	got, err := env.q.GetCSTraining(t.Context(), tr.ID)
	if err != nil {
		t.Fatalf("GetCSTraining: %v", err)
	}
	if got.TrainingStatus != "scheduled" {
		t.Errorf("status tak boleh berubah dari attendance invalid, got %q", got.TrainingStatus)
	}
}
