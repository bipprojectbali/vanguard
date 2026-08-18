package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sales_activities_notes_test.go — F4: masking Catatan Internal (Activity.Notes,
// fls.go). Dipecah dari sales_activities_test.go untuk file health (rule 8).
//
// Hanya admin/manager/sales yang bisa mencapai handler ini (crm:sales_activity
// write, satu-satunya pemegang selain admin — CSM/Support ditolak F2 lebih
// dulu, TestActivitiesList_ForbiddenWithoutPerm). Di antara ketiganya, HANYA
// admin ada di allow-list canSeeInternalNotes (admin/csm) — manager & sales
// adalah leak vector nyata yang dibuktikan di sini. Regresi audit FLS M9 gap 2.

const activityInternalNote = "Catatan rahasia pelanggan"

// seedSalesActivityWithNotes = seedSalesActivity + kolom Notes terisi (kind
// "task", satu-satunya kind Inti yang punya field notes di form v1).
func (e *testEnv) seedSalesActivityWithNotes(t *testing.T, targetType string, targetID int64, subject, notes string, owner *int64) db.Activity {
	t.Helper()
	a, err := e.q.CreateActivity(t.Context(), db.CreateActivityParams{
		TenantID:        e.tenantID,
		Kind:            "task",
		Subject:         subject,
		TargetType:      targetType,
		TargetID:        targetID,
		OwnerID:         owner,
		ActivityContext: ptr(activityContextSales),
		Notes:           ptr(notes),
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed activity w/ notes %s: %v", subject, err)
	}
	return a
}

// TestActivityDetail_NotesMasked: manager/sales (bukan admin/csm) tak melihat
// isi Notes di halaman detail — kosong, bukan penanda "•••" (maskInternalNotes
// mengosongkan, lihat komentar fls.go).
func TestActivityDetail_NotesMasked(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Notes", &uid, nil, nil)
	a := env.seedSalesActivityWithNotes(t, "account", acc.ID, "Tugas Bernotes", activityInternalNote, &uid)

	for _, role := range []string{"manager", "sales"} {
		t.Run("role="+role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/activities/"+itoa(a.ID), nil, itoa(a.ID))
			rec := env.runAccount(uid, "owner", role, req, env.h.ActivityDetail)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), activityInternalNote) {
				t.Errorf("role %q TAK BOLEH melihat Catatan Internal", role)
			}
		})
	}
}

// TestActivityDetail_NotesVisible_Admin: admin (allow-list) tetap melihat isi
// Notes apa adanya — memastikan fix tak over-masking role yang berhak.
func TestActivityDetail_NotesVisible_Admin(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Notes Admin", &uid, nil, nil)
	a := env.seedSalesActivityWithNotes(t, "account", acc.ID, "Tugas Bernotes Admin", activityInternalNote, &uid)

	req := accountsReq(http.MethodGet, "/activities/"+itoa(a.ID), nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ActivityDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), activityInternalNote) {
		t.Errorf("admin harus melihat Catatan Internal apa adanya")
	}
}

// TestActivityEdit_NotesMasked: form sunting (textarea prefill) tak menyertakan
// isi Notes bagi manager/sales — simetris dgn detail. Tanpa ini, textarea kosong
// SECARA VISUAL tapi HTML-nya bisa saja masih membawa nilai asli; dites langsung
// pada body respons.
func TestActivityEdit_NotesMasked(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Notes Edit", &uid, nil, nil)
	a := env.seedSalesActivityWithNotes(t, "account", acc.ID, "Tugas Edit", activityInternalNote, &uid)

	req := accountsReq(http.MethodGet, "/activities/"+itoa(a.ID)+"/edit", nil, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.ActivityEdit)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), activityInternalNote) {
		t.Errorf("form sunting TAK BOLEH memuat Catatan Internal bagi manager")
	}
}

// TestActivityUpdate_NotesPreserved: manager/sales mengirim POST dengan field
// notes (mis. textarea kosong karena prefill sudah tersamar, atau nilai lain
// hasil manipulasi) — nilai lama TETAP DIPERTAHANKAN, form diabaikan. Tanpa
// penjagaan ini, save oleh role tak berhak akan MENIMPA catatan Admin/CSM yang
// sudah ada — kebocoran F4 lewat jalur tulis.
func TestActivityUpdate_NotesPreserved(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Notes Update", &uid, nil, nil)
	a := env.seedSalesActivityWithNotes(t, "account", acc.ID, "Tugas Update", activityInternalNote, &uid)

	form := url.Values{}
	form.Set("subject", "Tugas Update (disunting manager)")
	form.Set("notes", "Coba timpa dari manager")

	req := accountsReq(http.MethodPost, "/activities/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.ActivityUpdate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	got, err := env.q.GetActivity(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}
	if got.Notes == nil || *got.Notes != activityInternalNote {
		t.Errorf("Notes harus tetap %q, got %v", activityInternalNote, got.Notes)
	}
	if got.Subject != "Tugas Update (disunting manager)" {
		t.Errorf("field lain (subject) harus tetap tersimpan, got %q", got.Subject)
	}
}

// TestActivityUpdate_NotesUpdated_Admin: admin (allow-list) TETAP bisa mengubah
// Notes lewat form — memastikan fix preserve-on-save tak diam-diam mengunci
// field ini bagi role yang memang berhak.
func TestActivityUpdate_NotesUpdated_Admin(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Notes Update Admin", &uid, nil, nil)
	a := env.seedSalesActivityWithNotes(t, "account", acc.ID, "Tugas Update Admin", activityInternalNote, &uid)

	form := url.Values{}
	form.Set("subject", "Tugas Update Admin")
	form.Set("notes", "Catatan baru dari admin")

	req := accountsReq(http.MethodPost, "/activities/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ActivityUpdate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}

	got, err := env.q.GetActivity(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}
	if got.Notes == nil || *got.Notes != "Catatan baru dari admin" {
		t.Errorf("admin harus bisa mengubah Notes, got %v", got.Notes)
	}
}
