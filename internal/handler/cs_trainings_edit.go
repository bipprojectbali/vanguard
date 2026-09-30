package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// cs_trainings_edit.go — BL-180: edit data training dari modal di kolom Aksi
// (topik, trainer, perkiraan peserta, catatan). Desa & tanggal/jam TAK bisa
// diubah (tanggal lewat "Jadwal Ulang"). completed/cancelled terkunci di query.

// csTrainingEditForm = data terurai form edit training.
type csTrainingEditForm struct {
	TrainingTopic string
	TrainerID     *int64
	Participants  *int32
	Notes         *string
}

// parseCSTrainingEditForm: topik wajib; trainer/peserta/catatan opsional
// (kosong → NULL = dikosongkan).
func parseCSTrainingEditForm(fv func(string) string) (csTrainingEditForm, string) {
	topic := strings.TrimSpace(fv("training_topic"))
	if topic == "" {
		return csTrainingEditForm{}, "required"
	}
	f := csTrainingEditForm{TrainingTopic: topic}
	if s := fv("trainer_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			f.TrainerID = &id
		}
	}
	if s := fv("participants"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 32); err == nil && n >= 0 {
			p := int32(n)
			f.Participants = &p
		}
	}
	if s := strings.TrimSpace(fv("notes")); s != "" {
		f.Notes = &s
	}
	return f, ""
}

// CSTrainingUpdate — POST /w/{workspace}/trainings/{id}. Gerbang F2 tulis + F3.
func (h *Handler) CSTrainingUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSTrainingWrite(w, r) {
		return
	}
	ctx := r.Context()

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	form, errCode := parseCSTrainingEditForm(r.FormValue)
	if errCode != "" {
		csTrainingsBack(w, r, "", errCode)
		return
	}
	if _, ok := h.loadCSTrainingInScope(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateCSTraining(ctx, db.UpdateCSTrainingParams{
		TrainingTopic: form.TrainingTopic,
		TrainerID:     form.TrainerID,
		Participants:  form.Participants,
		Notes:         form.Notes,
		UpdatedBy:     &uid,
		ID:            id,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			csTrainingsBack(w, r, "", "locked")
			return
		}
		h.Log.Error("cs_trainings: update", "err", err)
		csTrainingsBack(w, r, "", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "cs_training.update", session.TenantID(ctx), map[string]string{
		"training_id": strconv.FormatInt(id, 10),
	})
	csTrainingsBack(w, r, "updated", "")
}

// csTrainingsBack mengalihkan (303) ke daftar training dengan tab/pencarian/
// filter desa yang sama seperti saat form dikirim (dijahit view ke action form),
// plus kode PRG ?ok= / ?err=. Nilai tak sahih diabaikan (fail-soft → tanpa filter).
func csTrainingsBack(w http.ResponseWriter, r *http.Request, okCode, errCode string) {
	q := url.Values{}
	if t := r.FormValue("tab"); t == csTrainingTabAll || isValidEnum(t, csTrainingStatusValues) {
		q.Set("tab", t)
	}
	if s := strings.TrimSpace(r.FormValue("q")); s != "" {
		q.Set("q", s)
	}
	if a, err := strconv.ParseInt(r.FormValue("account"), 10, 64); err == nil && a > 0 {
		q.Set("account", strconv.FormatInt(a, 10))
	}
	if okCode != "" {
		q.Set("ok", okCode)
	}
	if errCode != "" {
		q.Set("err", errCode)
	}
	dest := wsPath(slugFromRequest(r), "/trainings")
	if enc := q.Encode(); enc != "" {
		dest += "?" + enc
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}
