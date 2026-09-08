package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_deals.go — AKSI CREATE Deal: gerbang tulis bersama, form kosong, & create.
// Sunting ada di sales_deals_update.go, ganti-stage/hapus di sales_deals_stage.go,
// helper bersama (loader/opsi/prefill) di sales_deals_helpers.go — dipecah dari
// satu file 367-baris semata untuk file health; gerbang & konvensi sama persis.
//
// Gerbang SEMUA aksi = canWriteDealsPerm (crm:deals write). Ownership F3 menjaga
// baris yang di-EDIT/HAPUS/PINDAH-STAGE lewat loadOwnedDeal (404 di luar cakupan).
// Jalur create utama = konversi lead (sales_convert.go); DealNew/DealCreate
// melayani deal berdiri sendiri (mis. renewal tanpa lead awal).

// dealAccountPickerLimit membatasi jumlah desa yang ditawarkan di dropdown form
// deal berdiri sendiri. Bukan daftar berkeyset penuh — sekadar picker; desa di
// luar batas tetap bisa jadi tujuan lewat konversi lead.
const dealAccountPickerLimit = 500

// requireDealWrite = gerbang tulis bersama. false & menulis penolakan bila aktor
// tak berhak (izin F2 write; read-only workspace ditolak lebih awal dgn pesan jelas).
func (h *Handler) requireDealWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteDealsPerm(r.Context()) {
		h.renderDealsForbidden(w, r)
		return false
	}
	return true
}

// DealNew — GET /w/{workspace}/deals/new. Form kosong untuk deal berdiri sendiri.
// Memuat pilihan desa dalam cakupan aktor (F3).
func (h *Handler) DealNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	accounts, err := h.dealAccountOptions(ctx)
	if err != nil {
		h.Log.Error("deals: account options", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.DealFormView{
		Base:     base,
		Action:   base + "/deals",
		IsEdit:   false,
		Err:      wsErrMsg(r.URL.Query().Get("err")),
		Types:    dealTypeOptions,
		Accounts: accounts,
	}
	h.renderWorkspaceShell(w, r, "Tambah Deal", "/deals", panel.DealForm(v))
}

// DealCreate — POST /w/{workspace}/deals. Membuat deal berdiri sendiri. entity_code
// dialokasikan DALAM tx ber-tenant yang sama (h.q) → atomik. Stage lahir di
// 'Prospecting'; deal_owner = pembuat (dasar F3 ScopeOwn).
func (h *Handler) DealCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	form, errCode := parseDealForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/deals/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	code, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityDeal)
	if err != nil {
		h.Log.Error("deals: generate code", "err", err)
		wsRedirect(w, r, "/deals/new", "failed")
		return
	}

	d, err := h.q(ctx).CreateDeal(ctx, db.CreateDealParams{
		TenantID:          tenantID,
		EntityCode:        &code,
		DealName:          form.DealName,
		AccountID:         form.AccountID,
		DealOwner:         &uid, // pembuat = pemilik awal (dasar F3 ScopeOwn)
		Stage:             dealInitialStage,
		DealType:          form.DealType,
		Amount:            form.Amount,
		Probability:       form.Probability,
		ExpectedCloseDate: form.ExpectedCloseDate,
		ForecastCategory:  form.ForecastCategory,
		NextStep:          form.NextStep,
		SubscriptionTerm:  nil, // BL-88: termin milik quote, bukan deal
		CreatedBy:         &uid,
	})
	if err != nil {
		h.Log.Error("deals: create", "err", err)
		wsRedirect(w, r, "/deals/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "deal.create", tenantID, map[string]string{
		"deal_id": strconv.FormatInt(d.ID, 10), "code": code,
	})
	wsRedirectOK(w, r, "/deals/"+strconv.FormatInt(d.ID, 10), "created")
}
