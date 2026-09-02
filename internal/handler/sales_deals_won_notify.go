package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
)

// sales_deals_won_notify.go — notifyWonSubscription dipisah dari
// sales_deals_won_subscription.go (file health, ambang Route/Handler 150). Satu
// paket: dipanggil DealStage setelah langganan lahir dari deal menang (BL-21).

// notifyWonSubscription memberi tahu owner langganan (= deal owner) dan semua Customer
// Success bahwa langganan lahir dari deal menang. Fail-soft: gagal daftar/notify tak
// membatalkan apa pun (baris langganan sudah tertulis). Payload = entity_code (bukan
// PII). Owner di-dedupe agar tak menerima dua notifikasi bila ia juga ber-peran CS.
func (h *Handler) notifyWonSubscription(ctx context.Context, tenantID int64, sub db.Subscription) {
	seen := map[int64]struct{}{}
	code := deref(sub.EntityCode)
	notifyOne := func(uid int64) {
		if _, dup := seen[uid]; dup {
			return
		}
		seen[uid] = struct{}{}
		h.notify(ctx, uid, tenantID, "subscription.created.from_deal", notifPayload{EntityCode: code})
	}
	if sub.SubscriptionOwner != nil {
		notifyOne(*sub.SubscriptionOwner)
	}
	ids, err := h.q(ctx).ListMembersByBusinessRole(ctx, db.ListMembersByBusinessRoleParams{
		TenantID: tenantID, BusinessRole: authz.BusinessRoleCSM,
	})
	if err != nil {
		h.Log.Error("deals: won sub list cs", "tenant_id", tenantID, "err", err)
		return
	}
	for _, uid := range ids {
		notifyOne(uid)
	}
}
