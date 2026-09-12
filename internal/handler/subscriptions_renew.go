package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/session"
)

// subscriptions_renew.go — perpanjangan langganan (M5-3c). Renewal = INSERT baris
// BARU (previous_subscription_id + previous_value snapshot), BUKAN memperpanjang
// tanggal. Dua jalur:
//
//   - Straight (MRR baru ≤ lama): Expired baris lama LALU CreateSubscription
//     baris 'Active', dalam SATU h.q(ctx) tx — invarian idx_subs_one_active.
//   - Upsell   (MRR baru  > lama): CreateSubscription baris 'PendingApproval'
//     (koeksis dgn baris lama yg masih Active); baris lama BELUM disentuh.
//     Manager memutuskan lewat approve/reject (subscriptions_approve.go).
//
// Navigasi = native POST → http.Redirect 303 (gotcha #16), bukan SSE. Helper
// bersama (load, params, parse, notify) di subscriptions_action.go. Eksekusi
// jalur upsell/straight & kloning paket di subscriptions_renew_actions.go.

// monthsPerYear = faktor ARR dari MRR (ARR = MRR × 12). Konstanta protokol
// (setahun 12 bulan), bukan ambang bisnis yang bisa berubah antar-environment.
const monthsPerYear = 12

// SubscriptionRenew — POST /w/{slug}/subscriptions/{id}/renew. Perpanjang satu
// langganan Active. Upsell → PendingApproval + notifikasi Manager; selain itu →
// Expired lama + Active baru dalam satu tx.
func (h *Handler) SubscriptionRenew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canRenewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	old, ok := h.loadOwnedSubscription(w, r, id)
	if !ok {
		return
	}
	idStr := strconv.FormatInt(id, 10)
	if old.Status != "Active" {
		wsRedirect(w, r, "/subscriptions/"+idStr, "sub_not_active")
		return
	}
	newMRR, errCode := renewMRR(r.FormValue("new_mrr"), old.Mrr)
	if errCode != "" {
		wsRedirect(w, r, "/subscriptions/"+idStr, errCode)
		return
	}
	newARR := mulNumericInt(newMRR, monthsPerYear)
	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	code, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntitySubscription)
	if err != nil {
		h.Log.Error("subscriptions: renew code", "subscription_id", id, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}

	if numericGreater(newMRR, old.Mrr) {
		h.renewUpsell(w, r, old, code, newMRR, newARR, uid, tenantID, idStr)
		return
	}
	h.renewStraight(w, r, old, code, newMRR, newARR, uid, tenantID, idStr)
}
