package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/ui/pages/panel"

	"github.com/starfederation/datastar-go/datastar"
)

// sales_activities_contact_options.go — reload SSE opsi Kontak saat Target
// berubah di form Aktivitas create-mode (BL-164). Dipicu picker Target lewat
// ui.OnChangePostQuery (account_typeahead.go TriggerURL), bukan navigasi — jadi
// SSE fragmen parsial sah di sini (gotcha #16, bukan pelanggaran).

// ActivityContactOptions — POST /w/{slug}/activities/contact-options?target=type:id.
// "target" dibaca dari QUERY STRING (bukan body form) — ui.OnChangePostQuery
// sengaja tak memakai {contentType:'form'}: itu men-checkValidity() SELURUH
// <form> aktivitas (termasuk field required lain spt Subjek) sebelum kirim,
// jadi @post gagal diam-diam saat form belum lengkap (bug dilaporkan user:
// Target berubah, Kontak tak ikut berubah). Balas SSE PatchElements 2 fragmen
// (kartu Call & Chat) agar Kontak ikut Target terbaru, apa pun kind yang
// sedang aktif di client. Target tak sah/di luar cakupan → fragmen kosong
// (bukan error, form tetap terisi).
func (h *Handler) ActivityContactOptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}
	target := strings.TrimSpace(r.URL.Query().Get("target"))

	var contacts []panel.AccountMemberOption
	var preselect string
	var leadInfo *panel.LeadContactInfoView
	if targetType, id, ok := parseActivityTarget(target); ok {
		var err error
		contacts, preselect, leadInfo, err = h.activityContactsForTarget(ctx, targetType, id)
		if err != nil {
			h.Log.Error("activities: contact options refresh", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	view := panel.ActivityFormView{ContactID: preselect, Contacts: contacts, LeadContactInfo: leadInfo}
	var call, chat strings.Builder
	if err := panel.ContactFieldFragment("contact-field-call", view).Render(&call); err != nil {
		h.Log.Error("activities: render contact fragment (call)", "err", err)
		return
	}
	if err := panel.ContactFieldFragment("contact-field-chat", view).Render(&chat); err != nil {
		h.Log.Error("activities: render contact fragment (chat)", "err", err)
		return
	}

	sse := datastar.NewSSE(w, r)
	_ = sse.PatchElements(call.String())
	_ = sse.PatchElements(chat.String())
}
