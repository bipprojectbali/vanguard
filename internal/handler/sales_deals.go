package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_deals.go — AKSI atas Deal: form buat/sunting, create, update, ganti stage,
// soft-delete. Dipisah dari sales_deals_page.go (baca): aksi tumbuh dengan aturan
// TULIS (F2 write), halaman dengan aturan LIHAT (F2 read + F3). Meniru sales_leads.go.
//
// Gerbang SEMUA aksi = canWriteDealsPerm (crm:deals write). Ownership F3 menjaga
// baris yang di-EDIT/HAPUS/PINDAH-STAGE lewat loadOwnedDeal (404 di luar cakupan).
// RLS tetap mengurung workspace. Jalur create utama = konversi lead (sales_convert.go);
// DealNew/DealCreate melayani deal berdiri sendiri (mis. renewal tanpa lead awal).

// dealAccountPickerLimit membatasi jumlah desa yang ditawarkan di dropdown form
// deal berdiri sendiri. Bukan daftar berkeyset penuh — sekadar picker; desa di
// luar batas tetap bisa jadi tujuan lewat konversi lead. Named-const, bukan angka
// telanjang (aturan hardcode).
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
		Terms:    subscriptionTermOptions,
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
		SubscriptionTerm:  form.SubscriptionTerm,
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
	v := panel.DealFormView{
		Base:     base,
		Action:   base + "/deals/" + idStr,
		IsEdit:   true,
		Err:      wsErrMsg(r.URL.Query().Get("err")),
		Fields:   dealFormFields(d),
		Types:    dealTypeOptions,
		Terms:    subscriptionTermOptions,
		Accounts: accounts,
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
		SubscriptionTerm:  form.SubscriptionTerm,
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

// DealStage — POST /w/{workspace}/deals/{id}/stage. Memindah tahap pipeline (aksi
// tersendiri, bukan efek edit). Closed Won/Lost WAJIB win_loss_reason → else
// ?err=win_loss (validasi di handler, bukan CHECK DB, agar pesan bisa diperbaiki
// user). loss_notes hanya relevan saat Closed Lost; disimpan apa adanya.
func (h *Handler) DealStage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	// Muat untuk menegakkan ownership (F3); isi baris tak dipakai — stage baru
	// datang dari form, bukan dari nilai lama.
	if _, ok := h.loadOwnedDeal(w, r, id); !ok {
		return
	}
	idStr := strconv.FormatInt(id, 10)

	stage := strings.TrimSpace(r.FormValue("stage"))
	if _, valid := validDealStages[stage]; !valid {
		wsRedirect(w, r, "/deals/"+idStr, "stage")
		return
	}
	winLoss := optTrim(r.FormValue("win_loss_reason"))
	lossNotes := optTrim(r.FormValue("loss_notes"))
	// Stage terminal menuntut alasan menang/kalah — jejak "kenapa" wajib ada saat
	// deal ditutup. Stage aktif tak menuntutnya.
	terminal := stage == "Closed Won" || stage == "Closed Lost"
	if terminal && winLoss == nil {
		wsRedirect(w, r, "/deals/"+idStr, "win_loss")
		return
	}
	if !terminal {
		// Pindah kembali ke stage aktif → bersihkan hasil (tak ada menang/kalah lagi).
		winLoss, lossNotes = nil, nil
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).UpdateDealStage(ctx, db.UpdateDealStageParams{
		Stage:         stage,
		WinLossReason: winLoss,
		LossNotes:     lossNotes,
		UpdatedBy:     &uid,
		ID:            id,
	}); err != nil {
		h.Log.Error("deals: stage", "err", err)
		wsRedirect(w, r, "/deals/"+idStr, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "deal.stage", session.TenantID(ctx), map[string]string{
		"deal_id": idStr, "stage": stage,
	})
	wsRedirectOK(w, r, "/deals/"+idStr, "staged")
}

// DealDelete — POST /w/{workspace}/deals/{id}/delete. Soft-delete (reversibel di
// DB via deleted_at; jejak audit mencatat siapa & kapan). FK leads.converted_deal_id
// tak putus (baris tetap ada).
func (h *Handler) DealDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedDeal(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteDeal(ctx, db.SoftDeleteDealParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("deals: delete", "err", err)
		wsRedirect(w, r, "/deals/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "deal.delete", session.TenantID(ctx), map[string]string{
		"deal_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/deals", "deleted")
}

// loadOwnedDeal memuat satu deal & menegakkan F3: di luar cakupan aktor → 404
// (kembaran DealDetail). Mengembalikan (deal, true) bila boleh, atau menulis
// 404/500 & (zero, false).
func (h *Handler) loadOwnedDeal(w http.ResponseWriter, r *http.Request, id int64) (db.Deal, bool) {
	ctx := r.Context()
	d, err := h.q(ctx).GetDeal(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Deal{}, false
		}
		h.Log.Error("deals: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Deal{}, false
	}
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), d.DealOwner) {
		http.NotFound(w, r)
		return db.Deal{}, false
	}
	return d, true
}

// dealAccountOptions memuat desa dalam cakupan aktor (F3) sebagai pilihan dropdown
// form deal. Label = kode + nama (kode dulu agar terurut & jelas). Satu query
// berbatas (bukan N+1, bukan seluruh tabel).
func (h *Handler) dealAccountOptions(ctx context.Context) ([]panel.AccountMemberOption, error) {
	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	cursorAt, cursorID := firstPageCursor()
	rows, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsSales:         filter.IsOwn,
		IsCsm:           filter.IsOwn,
		Uid:             &uid,
		PageSize:        dealAccountPickerLimit,
	})
	if err != nil {
		return nil, err
	}
	opts := make([]panel.AccountMemberOption, 0, len(rows))
	for _, a := range rows {
		label := a.VillageName
		if a.EntityCode != nil && *a.EntityCode != "" {
			label = *a.EntityCode + " — " + a.VillageName
		}
		opts = append(opts, panel.AccountMemberOption{ID: a.ID, Label: label})
	}
	return opts, nil
}

// dealFormFields memetakan Deal → nilai prefill form (semua string; nil → "").
func dealFormFields(d db.Deal) panel.DealFormFields {
	return panel.DealFormFields{
		DealName:          d.DealName,
		AccountID:         strconv.FormatInt(d.AccountID, 10),
		DealType:          deref(d.DealType),
		Amount:            numericStr(d.Amount),
		Probability:       probabilityStr(d.Probability),
		ExpectedCloseDate: dateStr(d.ExpectedCloseDate),
		ForecastCategory:  deref(d.ForecastCategory),
		NextStep:          deref(d.NextStep),
		SubscriptionTerm:  deref(d.SubscriptionTerm),
		Competitor:        deref(d.Competitor),
	}
}
