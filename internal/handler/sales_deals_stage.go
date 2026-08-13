package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_deals_stage.go — AKSI ganti tahap pipeline & hapus Deal. Dipecah dari
// sales_deals.go (create) untuk file health; gerbang & konvensi sama.

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
