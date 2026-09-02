package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_activities_update.go — AKSI update, ubah status, & soft-delete Sales
// Activity. Dipecah dari sales_activities.go (create) untuk file health; gerbang
// & konvensi sama.

// ActivityUpdate — POST /w/{workspace}/activities/{id}. Menyimpan sunting field
// (per kind baris — form dibentuk oleh a.Kind). kind, target, owner, & status
// TAK disentuh: kind/target immutable; owner tak diam-diam berpindah; status
// punya jalur khusus (ActivityStatus). reminder_at belum ber-form v1 → dipertahankan.
//
// F4: Notes DIPERTAHANKAN (bukan diambil dari form) bila role tak berhak lihat
// (!canSeeInternalNotes) — form GET (ActivityEdit) sudah mengosongkan
// textarea-nya (maskInternalNotes) buat role itu, jadi POST-nya SELALU kosong.
// Tanpa penjagaan ini, save oleh Manager/Sales akan menimpa catatan Admin/CSM
// yang sudah ada dengan string kosong — kebocoran F4 lewat jalur tulis, bukan
// baca. Diperbaiki audit FLS M9 (simetris dgn ActivityEdit/activityDetailView).
func (h *Handler) ActivityUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireActivityWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadOwnedActivity(w, r, id)
	if !ok {
		return
	}
	editPath := "/activities/" + strconv.FormatInt(id, 10) + "/edit"

	form, errCode := parseActivityForm(r.FormValue, a.Kind)
	if errCode != "" {
		wsRedirect(w, r, editPath, errCode)
		return
	}

	notes := form.Notes
	if br := session.BusinessRole(ctx); !canSeeInternalNotes(br) {
		notes = a.Notes // role tak berhak → pertahankan nilai lama, abaikan form
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateActivity(ctx, db.UpdateActivityParams{
		Subject:     form.Subject,
		OwnerID:     a.OwnerID,    // pertahankan pemilik
		ReminderAt:  a.ReminderAt, // pertahankan (belum ber-form v1)
		Notes:       notes,
		DueDate:     form.DueDate,
		Priority:    form.Priority,
		ContactID:   form.ContactID,
		Direction:   form.Direction,
		ActivityAt:  form.ActivityAt,
		DurationMin: form.DurationMin,
		CallResult:  form.CallResult,
		StartAt:     form.StartAt,
		EndAt:       form.EndAt,
		Location:    form.Location,
		MeetingType: form.MeetingType,
		Channel:     form.Channel,
		Body:        form.Body,
		UpdatedBy:   &uid,
		ID:          id,
	}); err != nil {
		h.Log.Error("activities: update", "err", err)
		wsRedirect(w, r, editPath, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "activity.update", session.TenantID(ctx), map[string]string{
		"activity_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/activities/"+strconv.FormatInt(id, 10), "saved")
}
