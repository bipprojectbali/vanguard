package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_new.go — form GET Sales Activity (buat/sunting). Opsi picker
// (target polimorfik & kontak) di sales_activities_options.go. Aksi POST di
// sales_activities.go dkk. Dipisah agar tiap file di bawah ambang file-health.

// ActivityNew — GET /w/{workspace}/activities/new?target=type:id. Form buat
// TUNGGAL (BL-19): Jenis dipilih di dropdown dalam form (tak lagi lewat ?kind=),
// nilai awal "task". target opsional (dari tombol "+ Log Aktivitas" di timeline
// entitas): pre-seleksi dropdown target bila nilainya ada di opsi; target tak ada /
// sudah terhapus → dropdown kosong (bukan error — target_id bukan FK, entitas bisa
// terhapus).
func (h *Handler) ActivityNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}

	// Baca ?target= dari timeline: "type:id" (e.g. "account:42"). Tak sah / kosong
	// → "" (dropdown target tampil tanpa pilihan awal, bukan error).
	preTarget := strings.TrimSpace(r.URL.Query().Get("target"))
	if _, _, ok := parseActivityTarget(preTarget); !ok {
		preTarget = ""
	}

	targets, err := h.activityTargetOptions(ctx)
	if err != nil {
		h.Log.Error("activities: target options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Form tunggal: grup field "call" bisa dipilih klien → opsi kontak WAJIB dimuat
	// walau kind awal "task" (pass "call" agar loader tak melewatinya).
	contacts, err := h.activityContactOptions(ctx, "call")
	if err != nil {
		h.Log.Error("activities: contact options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Tambah Aktivitas", "/activities",
		panel.ActivityForm(panel.ActivityFormView{
			Base:            base,
			Action:          base + "/activities",
			IsEdit:          false,
			Err:             wsErrMsg(r.URL.Query().Get("err")),
			Kind:            "task", // nilai awal signal $kind (dropdown Jenis)
			Kinds:           activityKindOptions,
			TargetValue:     preTarget,
			Targets:         targets,
			Contacts:        contacts,
			Priorities:      activityPriorityOptions,
			Statuses:        taskStatusOptions,
			MeetingStatuses: meetingStatusOptions,
			MeetingTypes:    meetingTypeOptions,
			Directions:      activityDirectionOptions,
			CallResults:     callResultOptions,
			Channels:        channelOptions,
		}))
}

// ActivityEdit — GET /w/{workspace}/activities/{id}/edit. Form terisi. kind &
// target immutable (target ditampilkan read-only). Di luar cakupan F3 → 404.
//
// F4: Notes prefill lewat maskInternalNotes — role yang bukan Admin/CSM
// (Manager/Sales, satu-satunya lain pemegang crm:sales_activity write) tak
// pernah menerima isi catatan asli ke browser, sekalipun via textarea form.
// ActivityUpdate (sales_activities_update.go) MEMPERTAHANKAN nilai lama saat
// role tak berhak submit — simetris, form kosong ini tak akan menimpa catatan
// asli di DB. Diperbaiki audit FLS M9.
func (h *Handler) ActivityEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadOwnedActivity(w, r, id)
	if !ok {
		return
	}
	br := session.BusinessRole(ctx)

	contacts, err := h.activityContactOptions(ctx, a.Kind)
	if err != nil {
		h.Log.Error("activities: contact options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	idStr := strconv.FormatInt(a.ID, 10)
	h.renderWorkspaceShell(w, r, "Sunting Aktivitas", "/activities",
		panel.ActivityForm(panel.ActivityFormView{
			Base:            base,
			Action:          base + "/activities/" + idStr,
			IsEdit:          true,
			Err:             wsErrMsg(r.URL.Query().Get("err")),
			Kind:            a.Kind,
			TargetValue:     a.TargetType + ":" + strconv.FormatInt(a.TargetID, 10),
			TargetLabel:     h.targetLabel(ctx, a.TargetType, a.TargetID),
			Subject:         a.Subject,
			DueDate:         dateStr(a.DueDate),
			Priority:        deref(a.Priority),
			Status:          deref(a.Status),
			ContactID:       int32ContactStr(a.ContactID),
			Contacts:        contacts,
			Direction:       deref(a.Direction),
			ActivityAt:      dateTimeStr(a.ActivityAt),
			Duration:        int32PtrStr(a.DurationMin),
			CallResult:      deref(a.CallResult),
			Channel:         deref(a.Channel),
			StartAt:         dateTimeStr(a.StartAt),
			EndAt:           dateTimeStr(a.EndAt),
			Location:        deref(a.Location),
			MeetingType:     deref(a.MeetingType),
			Body:            deref(a.Body),
			Notes:           maskInternalNotes(deref(a.Notes), br),
			Priorities:      activityPriorityOptions,
			Statuses:        taskStatusOptions,
			MeetingStatuses: meetingStatusOptions,
			MeetingTypes:    meetingTypeOptions,
			Directions:      activityDirectionOptions,
			CallResults:     callResultOptions,
			Channels:        channelOptions,
		}))
}
