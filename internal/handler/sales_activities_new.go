package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/ui/pages/panel"
)

// sales_activities_new.go — form GET Sales Activity (buat). Opsi picker (target
// polimorfik & kontak) di sales_activities_options.go. Aksi POST di
// sales_activities.go dkk. ActivityEdit dipisah ke sales_activities_edit.go
// agar file ini di bawah ambang file-health.

// ActivityNew — GET /w/{workspace}/activities/new?target=type:id. Form buat
// TUNGGAL (BL-19): Jenis dipilih di dropdown dalam form (tak lagi lewat ?kind=),
// nilai awal "task". target opsional (dari tombol "+ Log Aktivitas" di timeline
// entitas): pre-seleksi dropdown target bila nilainya ada di opsi; target tak ada /
// sudah terhapus → dropdown kosong (bukan error — target_id bukan FK, entitas bisa
// terhapus).
//
// currentPath via activityCurrentPathFromQuery (BL-161 lanjutan — sumber: user 14
// Sep): tombol "Tambah Aktivitas" di AllActivitiesList (/activity-log) menautkan
// "?from=log" ke sini; tanpa itu form ini SELALU hardcode "/activities" walau
// diklik dari menu "Activities", jadi sidebar keliru menyala "Sales Activities".
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

	// Kontak difilter mengikuti Target (BL-164): hanya bisa diresolusi bila
	// preTarget terisi (dari "+ Log Aktivitas" di timeline entitas). Target
	// dipilih SETELAH halaman dimuat (alur menu global) → Kontak mulai kosong,
	// direfresh via SSE saat Target berubah (ActivityContactOptions).
	var contacts []panel.AccountMemberOption
	var contactID string
	var leadInfo *panel.LeadContactInfoView
	if preTarget != "" {
		targetType, id, _ := parseActivityTarget(preTarget)
		contacts, contactID, leadInfo, err = h.activityContactsForTarget(ctx, targetType, id)
		if err != nil {
			h.Log.Error("activities: contact options", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Tambah Aktivitas", activityCurrentPathFromQuery(r),
		panel.ActivityForm(panel.ActivityFormView{
			Base:            base,
			Action:          base + "/activities",
			IsEdit:          false,
			Err:             wsErrMsg(r.URL.Query().Get("err")),
			Kind:            "task", // nilai awal signal $kind (dropdown Jenis)
			Kinds:           activityKindOptions,
			TargetValue:     preTarget,
			Targets:         targets,
			ContactID:       contactID,
			Contacts:        contacts,
			LeadContactInfo: leadInfo,
			Priorities:      activityPriorityOptions,
			Statuses:        taskStatusOptions,
			MeetingStatuses: meetingStatusOptions,
			MeetingTypes:    meetingTypeOptions,
			Directions:      activityDirectionOptions,
			CallResults:     callResultOptions,
			Channels:        channelOptions,
		}))
}
