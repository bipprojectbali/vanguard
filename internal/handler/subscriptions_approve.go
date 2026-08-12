package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// subscriptions_approve.go — keputusan Manager atas renewal Upsell yang menunggu
// (M5-3c). Gerbang = canApproveRenewal (crm:renewal_mgmt approve; hanya admin/
// manager). Approve → Expired baris lama LALU baris Pending jadi Active (satu tx,
// invarian idx_subs_one_active). Reject → baris Pending jadi Cancelled; baris lama
// tetap Active. Kedua jalur memberi tahu pemilik langganan (fail-soft).

// loadPendingRenewal memuat baris yang menunggu persetujuan. 404 bila tak ada;
// redirect ?err=not_pending bila status bukan 'PendingApproval' (sudah diputus).
// Mengembalikan (sub, idStr, true) hanya bila baris masih menunggu.
func (h *Handler) loadPendingRenewal(w http.ResponseWriter, r *http.Request) (db.Subscription, string, bool) {
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return db.Subscription{}, "", false
	}
	ctx := r.Context()
	idStr := strconv.FormatInt(id, 10)
	sub, err := h.q(ctx).GetSubscription(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Subscription{}, "", false
		}
		h.Log.Error("subscriptions: pending load", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Subscription{}, "", false
	}
	if sub.Status != "PendingApproval" {
		wsRedirect(w, r, "/subscriptions/"+idStr, "not_pending")
		return db.Subscription{}, "", false
	}
	return sub, idStr, true
}

// SubscriptionRenewApprove — POST /w/{slug}/subscriptions/{id}/approve. Setujui
// renewal Upsell: Expired baris lama dulu, lalu baris Pending → Active (satu tx).
func (h *Handler) SubscriptionRenewApprove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canApproveRenewal(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	sub, idStr, ok := h.loadPendingRenewal(w, r)
	if !ok {
		return
	}
	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	if sub.PreviousSubscriptionID != nil {
		if err := h.q(ctx).UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
			Status: "Expired", UpdatedBy: &uid, ID: *sub.PreviousSubscriptionID,
		}); err != nil {
			h.Log.Error("subscriptions: approve expire old", "previous_id", *sub.PreviousSubscriptionID, "err", err)
			wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
			return
		}
	}
	approved, err := h.q(ctx).ApproveRenewal(ctx, db.ApproveRenewalParams{
		ApprovedBy: &uid, UpdatedBy: &uid, ID: sub.ID,
	})
	if err != nil {
		h.Log.Error("subscriptions: approve", "subscription_id", sub.ID, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	if approved.SubscriptionOwner != nil {
		h.notify(ctx, *approved.SubscriptionOwner, tenantID, "renewal.approved",
			notifPayload{EntityCode: deref(approved.EntityCode)})
	}
	h.auditWorkspace(ctx, uid, "subscription.renew.approve", tenantID, map[string]string{
		"subscription_id": idStr,
	})
	wsRedirectOK(w, r, "/subscriptions/"+idStr, "renew_approved")
}

// SubscriptionRenewReject — POST /w/{slug}/subscriptions/{id}/reject. Tolak renewal
// Upsell: baris Pending → Cancelled. Baris lama tak disentuh (langganan berjalan).
func (h *Handler) SubscriptionRenewReject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canApproveRenewal(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	sub, idStr, ok := h.loadPendingRenewal(w, r)
	if !ok {
		return
	}
	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	rejected, err := h.q(ctx).RejectRenewal(ctx, db.RejectRenewalParams{
		ApprovedBy: &uid, UpdatedBy: &uid, ID: sub.ID,
	})
	if err != nil {
		h.Log.Error("subscriptions: reject", "subscription_id", sub.ID, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	if rejected.SubscriptionOwner != nil {
		h.notify(ctx, *rejected.SubscriptionOwner, tenantID, "renewal.rejected",
			notifPayload{EntityCode: deref(rejected.EntityCode)})
	}
	h.auditWorkspace(ctx, uid, "subscription.renew.reject", tenantID, map[string]string{
		"subscription_id": idStr,
	})
	wsRedirectOK(w, r, "/subscriptions/"+idStr, "renew_rejected")
}
