package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// playbooks_update.go — aksi UPDATE playbook + helper loadPlaybook. Dipisah dari
// playbooks.go (guard tulis + form baru/sunting + create) agar keduanya di bawah
// ambang tipe Route/Handler (150). Aturan tulis & warisan pemuatan baris sama;
// satu paket.
// PlaybookUpdate — POST /w/{workspace}/playbooks/{id}. Menyimpan sunting
// PROFIL. is_active TAK disentuh (SetPlaybookActive) → status draf
// dipertahankan.
func (h *Handler) PlaybookUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePlaybookWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadPlaybook(w, r, id); !ok {
		return
	}

	form, errCode := parsePlaybookForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/playbooks/"+strconv.FormatInt(id, 10)+"/edit", errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdatePlaybook(ctx, db.UpdatePlaybookParams{
		PlaybookName:     form.PlaybookName,
		TriggerScenario:  form.TriggerScenario,
		Description:      form.Description,
		Steps:            form.Steps,
		RecommendedOwner: form.RecommendedOwner,
		UpdatedBy:        &uid,
		ID:               id,
	}); err != nil {
		h.Log.Error("playbooks: update", "err", err)
		wsRedirect(w, r, "/playbooks/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "playbook.update", session.TenantID(ctx), map[string]string{
		"playbook_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/playbooks", "saved")
}

// loadPlaybook memuat satu playbook katalog. playbooks TANPA F3 (milik
// workspace) → cukup RLS. Tak ada → 404. Termasuk playbook draf
// (is_active=false) agar bisa disunting/diaktifkan kembali. Mengembalikan
// (playbook, true) atau menulis 404/500 & (zero, false).
func (h *Handler) loadPlaybook(w http.ResponseWriter, r *http.Request, id int64) (db.Playbook, bool) {
	p, err := h.q(r.Context()).GetPlaybook(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Playbook{}, false
		}
		h.Log.Error("playbooks: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Playbook{}, false
	}
	return p, true
}
