package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_deals_stage_bulk.go — BL-75: pindah tahap SATU ATAU BANYAK deal
// sekaligus dari drag-drop papan Kanban (dealboard.js). Endpoint TUNGGAL
// dipakai baik drag satu kartu maupun multiselect banyak kartu — gotcha #16
// (native POST → 303) melarang tiap kartu memicu navigasi sendiri, jadi drag
// tak bisa reuse /deals/{id}/stage. Kontrol non-drag (halaman detail,
// dealStageControl) tetap ADA & tak tersentuh — ini murni jalur TAMBAHAN.
//
// Best-effort per-baris (BUKAN all-or-nothing): tx request selalu commit
// (Scope.run tak punya sinyal rollback dari handler — sama seperti dicatat
// subscriptionFromWonDealCore), jadi baris tak-valid (di luar cakupan F3,
// transisi tak sah, validasi terminal gagal) SKIP & dihitung, baris valid
// tetap diproses. Redirect PRG tunggal di akhir ke papan (/deals), bukan ke
// detail per-deal.

// dealStageBulkCap = batas jumlah deal_id per request (guardrail; drag-drop
// UI wajar tak pernah memilih ratusan kartu sekaligus). ID kelebihan
// diabaikan, bukan menolak seluruh request.
const dealStageBulkCap = 50

// DealStageBulk — POST /w/{workspace}/deals/stage-bulk. Kontrak form (dirakit
// dealboard.js, native, bukan Datastar):
//   - stage        — SATU nilai, tahap tujuan untuk seluruh seleksi.
//   - deal_id      — hidden input berulang (name sama), satu per kartu.
//   - mode         — "shared" | "individual" (hanya relevan saat stage terminal).
//   - shared:     win_loss_reason, loss_reason_code, loss_notes, subscription_status
//     (satu field, dipakai semua deal).
//   - individual: field sama tapi bersufiks id, mis. win_loss_reason__<id>.
func (h *Handler) DealStageBulk(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		wsRedirect(w, r, "/deals", "stage_bulk_failed")
		return
	}
	stage := strings.TrimSpace(r.PostForm.Get("stage"))
	if _, valid := validDealStages[stage]; !valid {
		wsRedirect(w, r, "/deals", "stage")
		return
	}
	terminal := stage == "Closed Won" || stage == "Closed Lost"
	individual := r.PostForm.Get("mode") == "individual"

	ids := dedupDealIDs(r.PostForm["deal_id"], dealStageBulkCap)
	if len(ids) == 0 {
		wsRedirect(w, r, "/deals", "stage_bulk_failed")
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	q := h.q(ctx)
	moved := 0

	for _, id := range ids {
		deal, ok := h.loadOwnedDealSilent(ctx, id)
		if !ok {
			continue
		}
		// BL-159/173: sequential-only — cermin persis DealStage (jaring backend,
		// UI sudah membatasi target drop lewat StageRulesJSON).
		if !isValidDealStageTransition(deal.Stage, stage) {
			continue
		}
		idStr := strconv.FormatInt(id, 10)

		winLoss := optTrim(dealBulkField(r, "win_loss_reason", idStr, individual))
		lossNotes := optTrim(dealBulkField(r, "loss_notes", idStr, individual))
		lossCode := optTrim(dealBulkField(r, "loss_reason_code", idStr, individual))
		if terminal && winLoss == nil {
			continue
		}
		if stage == "Closed Lost" {
			if lossCode == nil {
				continue
			}
			if _, ok := validLossReasonCodes[*lossCode]; !ok {
				continue
			}
		} else {
			lossCode = nil
		}
		if !terminal {
			winLoss, lossNotes = nil, nil
		}

		var newSub *db.Subscription
		if stage == "Closed Won" {
			status := strings.TrimSpace(dealBulkField(r, "subscription_status", idStr, individual))
			sub, reason := h.subscriptionFromWonDealCore(ctx, deal, uid, status)
			if reason != "" {
				continue
			}
			newSub = sub
		}

		if err := q.UpdateDealStage(ctx, db.UpdateDealStageParams{
			Stage:          stage,
			WinLossReason:  winLoss,
			LossReasonCode: lossCode,
			LossNotes:      lossNotes,
			PrevStage:      deal.Stage,
			UpdatedBy:      &uid,
			ID:             id,
		}); err != nil {
			h.Log.Error("deals: stage bulk", "deal_id", id, "err", err)
			continue
		}

		if newSub != nil {
			subIDStr := strconv.FormatInt(newSub.ID, 10)
			if err := q.SetDealCreatedSubscription(ctx, db.SetDealCreatedSubscriptionParams{
				CreatedSubscriptionID: &newSub.ID, UpdatedBy: &uid, ID: id,
			}); err != nil {
				h.Log.Error("deals: stage bulk link subscription", "deal_id", id, "err", err)
			} else {
				h.auditWorkspace(ctx, uid, "subscription.created.from_deal", tenantID, map[string]string{
					"deal_id": idStr, "subscription_id": subIDStr,
				})
				h.notifyWonSubscription(ctx, tenantID, *newSub)
			}
		}
		if stage == "Closed Won" {
			h.handoverCSFromWonDeal(ctx, deal, tenantID, uid)
		}

		h.auditWorkspace(ctx, uid, "deal.stage", tenantID, map[string]string{
			"deal_id": idStr, "stage": stage,
		})
		moved++
	}

	if moved == 0 {
		wsRedirect(w, r, "/deals", "stage_bulk_failed")
		return
	}
	wsRedirectOK(w, r, "/deals", "staged_bulk")
}

// dealBulkField membaca field reason sesuai mode: shared → nama polos (satu
// nilai berlaku semua deal); individual → nama bersufiks `__<id>` (nilai
// per-deal, diisi dealboard.js dari baris ter-clone di modal).
func dealBulkField(r *http.Request, name, idStr string, individual bool) string {
	if individual {
		return r.PostForm.Get(name + "__" + idStr)
	}
	return r.PostForm.Get(name)
}

// dedupDealIDs mem-parse & dedup daftar deal_id mentah dari form, dibatasi
// cap (ID lewat parse gagal/duplikat diabaikan senyap — bukan alasan menolak
// seluruh request; validasi sesungguhnya tetap per-baris di loop pemanggil).
func dedupDealIDs(raw []string, limit int) []int64 {
	seen := make(map[int64]struct{}, len(raw))
	out := make([]int64, 0, len(raw))
	for _, s := range raw {
		if len(out) >= limit {
			break
		}
		id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
