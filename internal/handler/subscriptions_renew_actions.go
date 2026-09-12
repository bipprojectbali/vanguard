package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renew_actions.go — dua jalur eksekusi perpanjangan (upsell &
// straight) + kloning paket, dipisah dari subscriptions_renew.go (entry point
// SubscriptionRenew) untuk file health.

// renewUpsell membuat baris renewal 'PendingApproval' (JANGAN expire baris lama —
// keputusan itu milik Manager) lalu memberi tahu semua Manager.
func (h *Handler) renewUpsell(w http.ResponseWriter, r *http.Request, old db.Subscription, code string, newMRR, newARR pgtype.Numeric, uid, tenantID int64, idStr string) {
	ctx := r.Context()
	sub, err := h.q(ctx).CreateSubscription(ctx,
		renewParams(old, code, uid, newMRR, newARR, "PendingApproval", optTrim("Pending"), "Upsell", "In Progress", todayInAppTZ()))
	if err != nil {
		h.Log.Error("subscriptions: renew upsell", "previous_id", old.ID, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	// Kloning paket dari langganan lama (BL-88 PR2b): renewal mewarisi komposisi
	// paket. Status PendingApproval → item parent_active=false (trigger) → tak bentrok
	// dgn item lama yg masih Active; aktivasi menyusul di approve (expire lama dulu).
	h.cloneSubscriptionItems(ctx, old.ID, sub.ID, tenantID)
	h.notifyManagersUpsell(ctx, tenantID, sub)
	h.auditWorkspace(ctx, uid, "subscription.renew.upsell", tenantID, map[string]string{
		"subscription_id": strconv.FormatInt(sub.ID, 10),
		"previous_id":     idStr,
	})
	wsRedirectOK(w, r, "/subscriptions/"+strconv.FormatInt(sub.ID, 10), "renew_pending")
}

// renewStraight meng-Expired baris lama LALU membuat baris 'Active' baru — urutan
// itu invarian (idx_subs_one_active menolak 2 Active); keduanya di satu h.q tx.
func (h *Handler) renewStraight(w http.ResponseWriter, r *http.Request, old db.Subscription, code string, newMRR, newARR pgtype.Numeric, uid, tenantID int64, idStr string) {
	ctx := r.Context()
	if err := h.q(ctx).UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		Status: "Expired", UpdatedBy: &uid, ID: old.ID,
	}); err != nil {
		h.Log.Error("subscriptions: renew expire old", "previous_id", old.ID, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	sub, err := h.q(ctx).CreateSubscription(ctx,
		renewParams(old, code, uid, newMRR, newARR, "Active", nil, "Manual", "Renewed", todayInAppTZ()))
	if err != nil {
		h.Log.Error("subscriptions: renew create", "previous_id", old.ID, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	// Kloning paket (BL-88 PR2b): baris lama SUDAH Expired (item parent_active=false),
	// baris baru Active → item kloning parent_active=true tanpa langgar
	// idx_subscription_items_one_active. Urutan expire-dulu itu invarian.
	h.cloneSubscriptionItems(ctx, old.ID, sub.ID, tenantID)
	h.auditWorkspace(ctx, uid, "subscription.renew", tenantID, map[string]string{
		"subscription_id": strconv.FormatInt(sub.ID, 10),
		"previous_id":     idStr,
	})
	wsRedirectOK(w, r, "/subscriptions/"+strconv.FormatInt(sub.ID, 10), "renewed")
}

// cloneSubscriptionItems menyalin baris subscription_items dari langganan lama ke
// baris renewal baru (BL-88 PR2b): renewal mewarisi komposisi paket. account_id &
// parent_active DITURUNKAN trigger dari status parent baru (Active → aktif,
// PendingApproval → tidak) — cukup oper kolom snapshot. FAIL-SOFT: gagal per-item
// di-Log, tak menggagalkan renewal (parent sudah lahir), selaras pola Won.
func (h *Handler) cloneSubscriptionItems(ctx context.Context, fromID, toID, tenantID int64) {
	items, err := h.q(ctx).ListSubscriptionItems(ctx, fromID)
	if err != nil {
		h.Log.Error("subscriptions: renew clone items list", "previous_id", fromID, "err", err)
		return
	}
	for _, it := range items {
		if _, err := h.q(ctx).AddSubscriptionItem(ctx, db.AddSubscriptionItemParams{
			SubscriptionID: toID,
			TenantID:       tenantID,
			PlanID:         it.PlanID,
			Quantity:       it.Quantity,
			UnitPrice:      it.UnitPrice,
			DiscountPct:    it.DiscountPct,
			Subtotal:       it.Subtotal,
			Mrr:            it.Mrr,
			Arr:            it.Arr,
			LineNo:         it.LineNo,
		}); err != nil {
			h.Log.Error("subscriptions: renew clone item add", "sub_id", toID, "err", err)
		}
	}
}
