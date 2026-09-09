package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_deals_update.go — AKSI sunting PROFIL Deal (form + simpan). Dipecah dari
// sales_deals.go (create) untuk file health; gerbang & konvensi sama.

// DealEdit — GET /w/{workspace}/deals/{id}/edit. Form terisi profil deal (bukan
// stage). Mensyaratkan F3: hanya yang boleh MELIHAT baris yang boleh menyuntingnya.
func (h *Handler) DealEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	d, ok := h.loadOwnedDeal(w, r, id)
	if !ok {
		return
	}
	accounts, err := h.dealAccountOptions(ctx)
	if err != nil {
		h.Log.Error("deals: account options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	idStr := strconv.FormatInt(d.ID, 10)
	fields := dealFormFields(d)
	// Prefill nama desa terpilih untuk input teks pemilih typeahead (BL-9); juga
	// menjamin opsi desa ini hadir di <datalist> walau di luar batas picker.
	fields.SelectedAccountLabel = h.accountLabel(ctx, d.AccountID)
	v := panel.DealFormView{
		Base:               base,
		Action:             base + "/deals/" + idStr,
		IsEdit:             true,
		Err:                wsErrMsg(r.URL.Query().Get("err")),
		Fields:             fields,
		Types:              dealTypeOptions,
		ForecastCategories: forecastCategoryOptions,
		Accounts:           accounts,
	}
	h.renderWorkspaceShell(w, r, "Sunting Deal", "/deals", panel.DealForm(v))
}

// DealUpdate — POST /w/{workspace}/deals/{id}. Menyimpan sunting PROFIL. stage,
// entity_code, deal_owner, hasil (win_loss/closed_date), primary_contact_id, &
// plan_requested_id TAK disentuh form → dipertahankan apa adanya (owner tak diam-
// diam berpindah; kontak & paket hasil konversi tak terhapus oleh edit profil).
func (h *Handler) DealUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	d, ok := h.loadOwnedDeal(w, r, id)
	if !ok {
		return
	}

	form, errCode := parseDealForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/deals/"+strconv.FormatInt(id, 10)+"/edit", errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateDeal(ctx, db.UpdateDealParams{
		DealName:          form.DealName,
		AccountID:         form.AccountID,
		DealOwner:         d.DealOwner,        // pertahankan pemilik
		PrimaryContactID:  d.PrimaryContactID, // pertahankan kontak utama hasil konversi
		PlanRequestedID:   d.PlanRequestedID,  // pertahankan paket diminta
		DealType:          form.DealType,
		Amount:            form.Amount,
		Probability:       form.Probability,
		ExpectedCloseDate: form.ExpectedCloseDate,
		ForecastCategory:  form.ForecastCategory,
		NextStep:          form.NextStep,
		SubscriptionTerm:  d.SubscriptionTerm, // BL-88: termin milik quote; pertahankan nilai lama, tak diedit di form deal
		Competitor:        form.Competitor,
		UpdatedBy:         &uid,
		ID:                id,
	}); err != nil {
		h.Log.Error("deals: update", "err", err)
		wsRedirect(w, r, "/deals/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "deal.update", session.TenantID(ctx), map[string]string{
		"deal_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/deals/"+strconv.FormatInt(id, 10), "saved")
}
