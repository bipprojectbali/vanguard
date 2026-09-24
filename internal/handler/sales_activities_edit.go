package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/ui/pages/panel"
)

// sales_activities_edit.go — form GET Sales Activity (sunting). Dipisah dari
// sales_activities_new.go (ActivityNew) agar file itu di bawah ambang
// file-health.

// ActivityEdit — GET /w/{workspace}/activities/{id}/edit. Form terisi. kind &
// target immutable (target ditampilkan read-only). Di luar cakupan F3 → 404.
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

	// Kontak difilter mengikuti Target (BL-164), hanya dimuat bila kind call/chat
	// (optimisasi, sama seperti guard lama). ContactID TETAP dari a.ContactID
	// tersimpan (JANGAN ditimpa preselect resolver — itu untuk create-mode).
	var contacts []panel.AccountMemberOption
	var leadInfo *panel.LeadContactInfoView
	if a.Kind == "call" || a.Kind == "chat" {
		var err error
		contacts, _, leadInfo, err = h.activityContactsForTarget(ctx, a.TargetType, a.TargetID)
		if err != nil {
			h.Log.Error("activities: contact options", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
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
			LeadContactInfo: leadInfo,
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
			Notes:           deref(a.Notes),
			Priorities:      activityPriorityOptions,
			Statuses:        taskStatusOptions,
			MeetingStatuses: meetingStatusOptions,
			MeetingTypes:    meetingTypeOptions,
			Directions:      activityDirectionOptions,
			CallResults:     callResultOptions,
			Channels:        channelOptions,
		}))
}
