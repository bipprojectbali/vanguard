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
// user). Closed Lost JUGA WAJIB loss_reason_code (picklist, BL-44) → ?err=loss_reason.
// loss_notes hanya relevan saat Closed Lost; disimpan apa adanya.
func (h *Handler) DealStage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	// Muat untuk menegakkan ownership (F3); baris dipakai create-from-deal (BL-21)
	// bila stage jadi Closed Won (account/owner/plan/amount/termin diturunkan darinya).
	deal, ok := h.loadOwnedDeal(w, r, id)
	if !ok {
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
	lossCode := optTrim(r.FormValue("loss_reason_code"))
	// Stage terminal menuntut alasan menang/kalah — jejak "kenapa" wajib ada saat
	// deal ditutup. Stage aktif tak menuntutnya.
	terminal := stage == "Closed Won" || stage == "Closed Lost"
	if terminal && winLoss == nil {
		wsRedirect(w, r, "/deals/"+idStr, "win_loss")
		return
	}
	// BL-44 (3a): Closed Lost WAJIB kode alasan kalah terstruktur (picklist) untuk
	// laporan Win/Loss yang bersih; harus salah satu nilai valid. Kode HANYA relevan
	// saat kalah → dikosongkan untuk stage lain (Won & stage aktif).
	if stage == "Closed Lost" {
		if lossCode == nil {
			wsRedirect(w, r, "/deals/"+idStr, "loss_reason")
			return
		}
		if _, ok := validLossReasonCodes[*lossCode]; !ok {
			wsRedirect(w, r, "/deals/"+idStr, "loss_reason")
			return
		}
	} else {
		lossCode = nil
	}
	if !terminal {
		// Pindah kembali ke stage aktif → bersihkan hasil (tak ada menang/kalah lagi).
		winLoss, lossNotes = nil, nil
	}

	uid := session.UserID(ctx)

	// BL-21: Closed Won → buat langganan LEBIH DULU. Atomik via urutan — gagal/validasi
	// tak lolos → subscriptionFromWonDeal menulis ?err & return false → kita berhenti
	// SEBELUM UpdateDealStage → deal tetap stage lama (keputusan d). newSub nil bila
	// di-skip idempoten (deal sudah menautkan langganan).
	var newSub *db.Subscription
	if stage == "Closed Won" {
		sub, ok := h.subscriptionFromWonDeal(w, r, deal, uid)
		if !ok {
			return
		}
		newSub = sub
	}

	if err := h.q(ctx).UpdateDealStage(ctx, db.UpdateDealStageParams{
		Stage:          stage,
		WinLossReason:  winLoss,
		LossReasonCode: lossCode,
		LossNotes:      lossNotes,
		UpdatedBy:      &uid,
		ID:             id,
	}); err != nil {
		h.Log.Error("deals: stage", "err", err)
		wsRedirect(w, r, "/deals/"+idStr, "failed")
		return
	}

	tenantID := session.TenantID(ctx)
	// Tautan balik deal→langganan + audit + notify hanya bila langganan benar-benar
	// lahir dari transisi ini (dalam tx yang SAMA → tautan dua-arah atomik).
	if newSub != nil {
		subIDStr := strconv.FormatInt(newSub.ID, 10)
		if err := h.q(ctx).SetDealCreatedSubscription(ctx, db.SetDealCreatedSubscriptionParams{
			CreatedSubscriptionID: &newSub.ID, UpdatedBy: &uid, ID: id,
		}); err != nil {
			h.Log.Error("deals: link created subscription", "deal_id", id, "err", err)
			wsRedirect(w, r, "/deals/"+idStr, "failed")
			return
		}
		h.auditWorkspace(ctx, uid, "subscription.created.from_deal", tenantID, map[string]string{
			"deal_id": idStr, "subscription_id": subIDStr,
		})
		h.notifyWonSubscription(ctx, tenantID, *newSub)
	}

	h.auditWorkspace(ctx, uid, "deal.stage", tenantID, map[string]string{
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
