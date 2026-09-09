package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_action.go — helper bersama jalur mutasi langganan (renew / approve /
// churn, M5-3c): pemuatan ber-ownership, perakitan param renewal, parse MRR, dan
// notifikasi Manager. Dipisah dari handler HTTP agar tiap file di bawah batas 150.

// loadOwnedSubscription memuat satu langganan lalu menegakkan F3 (ownership):
// di luar cakupan → 404 (keberadaan baris tak diungkap). Dipakai renew & churn.
// Mengembalikan (sub, true) hanya bila baris ada DAN dalam cakupan pemanggil.
func (h *Handler) loadOwnedSubscription(w http.ResponseWriter, r *http.Request, id int64) (db.Subscription, bool) {
	ctx := r.Context()
	s, err := h.q(ctx).GetSubscription(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Subscription{}, false
		}
		h.Log.Error("subscriptions: load", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Subscription{}, false
	}
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), s.SubscriptionOwner) {
		http.NotFound(w, r)
		return db.Subscription{}, false
	}
	return s, true
}

// renewParams merakit baris renewal dari baris lama: salin account/plan/owner/
// auto_renew, tautkan previous_subscription_id + previous_value (= MRR lama, snapshot
// nilai sebelum renewal), pasang MRR/ARR & status baru. Periode baru dimulai `start`
// (zona app) sepanjang termin lama (BL-126: renewal WAJIB mengisi start/end — tanpa
// ini detail langganan hasil perpanjangan menampilkan Mulai/Berakhir kosong).
func renewParams(old db.Subscription, code string, uid int64, newMRR, newARR pgtype.Numeric, status string, approval *string, renewalType, renewalStatus string, start time.Time) db.CreateSubscriptionParams {
	months := renewTermMonths(old)
	term := int32(months)
	return db.CreateSubscriptionParams{
		TenantID:               old.TenantID,
		EntityCode:             &code,
		SubscriptionOwner:      old.SubscriptionOwner,
		AccountID:              old.AccountID,
		PlanID:                 old.PlanID,
		PreviousSubscriptionID: &old.ID,
		Status:                 status,
		ApprovalStatus:         approval,
		StartDate:              dateOnly(start),
		EndDate:                dateOnly(start.AddDate(0, months, 0)),
		BillingCycle:           old.BillingCycle,
		AutoRenew:              old.AutoRenew,
		ContractTermMonths:     &term,
		Mrr:                    newMRR,
		Arr:                    newARR,
		PreviousValue:          old.Mrr,
		RenewalType:            &renewalType,
		RenewalStatus:          &renewalStatus,
		CreatedBy:              &uid,
	}
}

// renewTermMonths = panjang termin (bulan) periode renewal: pakai
// contract_term_months langganan lama bila terisi; jika NULL/nol, fallback
// monthsPerYear (12). Menurunkan end_date = start + termin agar konsisten dgn
// pembuatan langganan dari Deal Won (sales_deals_won_subscription.go).
func renewTermMonths(old db.Subscription) int {
	if old.ContractTermMonths != nil && *old.ContractTermMonths > 0 {
		return int(*old.ContractTermMonths)
	}
	return monthsPerYear
}

// renewMRR menentukan MRR periode baru: kosong = ikut MRR lama (perpanjangan datar);
// diisi = wajib angka ≥ 0. Mengembalikan (nilai, "") sukses atau (_, kodeErr).
func renewMRR(raw string, oldMRR pgtype.Numeric) (pgtype.Numeric, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return oldMRR, ""
	}
	n, code := optNumeric(cleanThousands(raw), "new_mrr")
	if code != "" {
		return pgtype.Numeric{}, code
	}
	if !numericNonNegative(n) {
		return pgtype.Numeric{}, "new_mrr"
	}
	return n, ""
}

// notifyManagersUpsell memberi tahu SEMUA Manager workspace bahwa ada renewal
// upsell menunggu persetujuan. Fail-soft: gagal daftar/notify tak membatalkan
// renewal (baris Pending sudah tertulis). Payload = entity_code (bukan PII).
func (h *Handler) notifyManagersUpsell(ctx context.Context, tenantID int64, sub db.Subscription) {
	ids, err := h.q(ctx).ListMembersByBusinessRole(ctx, db.ListMembersByBusinessRoleParams{
		TenantID: tenantID, BusinessRole: authz.BusinessRoleManager,
	})
	if err != nil {
		h.Log.Error("subscriptions: list managers", "tenant_id", tenantID, "err", err)
		return
	}
	for _, uid := range ids {
		h.notify(ctx, uid, tenantID, "renewal.upsell.pending", notifPayload{EntityCode: deref(sub.EntityCode)})
	}
}
