package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_churn.go — tandai langganan berhenti (churn, M5-3c). Gerbang =
// canChurnSubscriptions (crm:churn write; admin/manager/csm). Churn = SATU aksi
// bisnis: set status 'Churned' + seluruh kolom churn sekaligus (ChurnSubscription),
// lost_value_mrr = MRR saat ini (nilai yang hilang). Navigasi native POST → 303.

// churnReasonOptions / churnTypeOptions = domain enum churn (cermin subs_churn_*_chk,
// migrasi 00012). SATU sumber: dipakai validasi (inList) DAN dropdown view — tak bisa
// menyimpang. Alasan/tipe kosong = churn tanpa rincian (keduanya nullable).
var churnReasonOptions = []string{
	"Budget", "No Adoption", "Change of Leadership", "Competitor", "Dissatisfaction", "Feature Gap",
}
var churnTypeOptions = []string{"Voluntary", "Involuntary"}

// churnForm = masukan churn yang sudah tervalidasi (pointer = boleh NULL).
type churnForm struct {
	Reason  *string
	Type    *string
	Notes   *string
	WinBack *bool
}

// SubscriptionChurn — POST /w/{slug}/subscriptions/{id}/churn. Tandai langganan
// churn. Hanya langganan Active/Trial yang bisa di-churn (status akhir tak diputar
// balik). lost_value_mrr = MRR baris saat ini.
func (h *Handler) SubscriptionChurn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canChurnSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	sub, ok := h.loadOwnedSubscription(w, r, id)
	if !ok {
		return
	}
	idStr := strconv.FormatInt(id, 10)
	if sub.Status != "Active" && sub.Status != "Trial" {
		wsRedirect(w, r, "/subscriptions/"+idStr, "sub_not_active")
		return
	}
	form, errCode := parseChurnForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/subscriptions/"+idStr, errCode)
		return
	}
	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	if err := h.q(ctx).ChurnSubscription(ctx, db.ChurnSubscriptionParams{
		Status:           "Churned",
		CancellationDate: pgtype.Date{Time: time.Now(), Valid: true},
		ChurnReason:      form.Reason,
		ChurnType:        form.Type,
		ChurnNotes:       form.Notes,
		LostValueMrr:     sub.Mrr,
		WinBackEligible:  form.WinBack,
		UpdatedBy:        &uid,
		ID:               id,
	}); err != nil {
		h.Log.Error("subscriptions: churn", "subscription_id", id, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	h.auditWorkspace(ctx, uid, "subscription.churn", tenantID, map[string]string{
		"subscription_id": idStr,
	})
	wsRedirectOK(w, r, "/subscriptions/"+idStr, "churned")
}

// parseChurnForm memvalidasi masukan churn. Alasan/tipe wajib dari domain bila diisi
// (di luar domain → kode error, bukan diam-diam disimpan). Notes opsional; win_back =
// checkbox (hadir = true). Mengembalikan (form, "") sukses atau (_, kodeErr).
func parseChurnForm(fv func(string) string) (churnForm, string) {
	var f churnForm
	if reason := strings.TrimSpace(fv("churn_reason")); reason != "" {
		if !inList(churnReasonOptions, reason) {
			return churnForm{}, "churn_reason"
		}
		f.Reason = &reason
	}
	if ctype := strings.TrimSpace(fv("churn_type")); ctype != "" {
		if !inList(churnTypeOptions, ctype) {
			return churnForm{}, "churn_type"
		}
		f.Type = &ctype
	}
	f.Notes = optTrim(fv("churn_notes"))
	wb := strings.TrimSpace(fv("win_back_eligible")) != ""
	f.WinBack = &wb
	return f, ""
}

// inList = keanggotaan sederhana pada slice pendek (domain enum). Menghindari peta
// terpisah yang bisa menyimpang dari daftar opsi view (satu sumber).
func inList(opts []string, v string) bool {
	for _, o := range opts {
		if o == v {
			return true
		}
	}
	return false
}
