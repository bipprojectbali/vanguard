package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// subscriptions_activate.go — aktivasi langganan Trial → Active (BL-73). Menutup
// jalan buntu status Trial: deal Closed Won boleh memilih status awal Trial, tetapi
// sebelum ini tak ada jalur transisi ke Active (renew butuh Active, approve hanya
// PendingApproval). Trial tak diakui pendapatan (semua agregasi MRR/ARR di report &
// dashboard FILTER status='Active'), tak bisa di-renew, satu-satunya keluar = churn/
// Expired. Gerbang = canActivateSubscriptions (crm:renewals write; admin/manager) —
// aktivasi sekelas renewal ("bikin langganan hidup"). Navigasi native POST → 303
// (gotcha #16). Cakupan HANYA Trial→Active; renew-dari-Trial (Opsi B) di luar cakupan.

// SubscriptionActivate — POST /w/{slug}/subscriptions/{id}/activate. Naikkan satu
// langganan Trial → Active. Menolak bila bukan Trial (sub_not_trial) atau sudah ada
// langganan Active untuk (account, plan) yang sama (sub_dup_active). Cek satu-Active
// WAJIB di sini: pre-check Won dilewati untuk Trial (sales_deals_won_subscription.go)
// & idx_subscription_items_one_active cuma WHERE parent_active — membiarkan UPDATE
// (status→Active memicu resync parent_active item) melanggar partial-unique meng-abort
// seluruh tx ber-tenant.
func (h *Handler) SubscriptionActivate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canActivateSubscriptions(ctx) {
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
	if sub.Status != "Trial" {
		wsRedirect(w, r, "/subscriptions/"+idStr, "sub_not_trial")
		return
	}
	// Cek satu-Active SAAT aktivasi (pre-check beri pesan ramah; index tetap penjaga
	// keras bila balapan). Trial boleh koeksis dgn Active, tapi menaikkan ke Active
	// akan melanggar idx_subscription_items_one_active bila sudah ada item Active plan
	// sama di langganan lain. BL-88 PR2b: cek lintas SEMUA paket langganan ini (item),
	// bukan sekadar sub.PlanID parent (yang bisa NULL utk multi-paket).
	exists, err := h.q(ctx).HasActiveItemConflictForSubscription(ctx, id)
	if err != nil {
		h.Log.Error("subscriptions: activate active-check", "subscription_id", id, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	if exists {
		wsRedirect(w, r, "/subscriptions/"+idStr, "sub_dup_active")
		return
	}
	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	if err := h.q(ctx).UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		Status: "Active", UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("subscriptions: activate", "subscription_id", id, "err", err)
		wsRedirect(w, r, "/subscriptions/"+idStr, "failed")
		return
	}
	h.auditWorkspace(ctx, uid, "subscription.activated", tenantID, map[string]string{
		"subscription_id": idStr,
	})
	wsRedirectOK(w, r, "/subscriptions/"+idStr, "activated")
}
