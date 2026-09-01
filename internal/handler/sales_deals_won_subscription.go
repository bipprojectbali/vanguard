package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_deals_won_subscription.go — BL-21: deal Closed Won → auto-create Subscription.
// Jalur konversi pasca-menang (sebelumnya langganan HANYA lahir dari renewal + seed;
// deals.created_subscription_id tak pernah terisi). Dipanggil DealStage SEBELUM
// UpdateDealStage: arsitektur menjalankan seluruh request dalam SATU tx yang SELALU
// commit (Scope.run balik nil — tak ada sinyal rollback dari handler), jadi "atomik"
// (keputusan d) dicapai lewat URUTAN: create dulu; gagal → return lebih awal →
// UpdateDealStage tak pernah dipanggil → deal tetap stage lama.

// multiYearContractMonths = asumsi durasi kontrak "Multi-year" (3 tahun). Termin tak
// memuat jumlah tahun konkret; diekstrak agar bila bisnis mengonkretkan durasi (mis.
// field tahun di deal), satu tempat berubah, bukan tebakan tersebar.
const multiYearContractMonths = 36

// termContractMonths memetakan Termin Langganan deal → jumlah bulan kontrak. amount
// deal = NILAI PER TERMIN (keputusan b) → MRR = amount / bulan-kontrak. Cermin enum
// validSubscriptionTerms (sales_deals_form.go); billing_cycle memakai termin apa adanya
// (subs_billing_cycle_chk menerima Monthly/Annual/Multi-year).
var termContractMonths = map[string]int32{
	"Monthly":    1,
	"Annual":     12,
	"Multi-year": multiYearContractMonths,
}

// validInitialSubStatuses = status awal yang boleh dipilih user saat Closed Won
// (keputusan a: user memilih Trial/Active, bukan hardcode). Subset subs_status_chk;
// status lain (Suspended/Expired/…) tak masuk akal sebagai langganan yang baru lahir.
var validInitialSubStatuses = map[string]struct{}{
	"Active": {}, "Trial": {},
}

// wonSubStatusOptions = urutan tampil dropdown status di form Closed Won (Active
// didahulukan: deal menang paling sering langsung aktif). Isi dijaga cermin
// validInitialSubStatuses via test.
var wonSubStatusOptions = []string{"Active", "Trial"}

// subscriptionFromWonDeal membuat langganan dari deal yang baru di-Closed Won.
// Mengembalikan (sub, true) bila langganan lahir, (nil, true) bila di-SKIP karena
// idempoten (deal sudah menautkan langganan — cegah dobel saat Won→lain→Won), atau
// (nil, false) bila GAGAL/validasi tak lolos (response ?err sudah ditulis; pemanggil
// WAJIB berhenti tanpa mengubah stage). Semua lewat h.q(ctx) — tx ber-tenant, JANGAN
// h.DB.
func (h *Handler) subscriptionFromWonDeal(w http.ResponseWriter, r *http.Request, deal db.Deal, uid int64) (*db.Subscription, bool) {
	ctx := r.Context()
	idStr := strconv.FormatInt(deal.ID, 10)

	// Idempotensi: sudah tertaut → jangan buat lagi (re-Won tak menggandakan).
	if deal.CreatedSubscriptionID != nil {
		return nil, true
	}
	// subscriptions.plan_id NOT NULL; deals.plan_requested_id nullable → wajib ada.
	// Bukan diam-diam (keputusan d): ?err agar user set paket dulu sebelum menang.
	if deal.PlanRequestedID == nil {
		wsRedirect(w, r, "/deals/"+idStr, "plan_required")
		return nil, false
	}
	// Termin wajib: menentukan bulan-kontrak → MRR. Tanpa itu nilai tak terturunkan.
	term := deref(deal.SubscriptionTerm)
	months, ok := termContractMonths[term]
	if !ok {
		wsRedirect(w, r, "/deals/"+idStr, "term_required")
		return nil, false
	}
	// Status awal dari form (keputusan a). Wajib Trial/Active.
	status := strings.TrimSpace(r.FormValue("subscription_status"))
	if _, ok := validInitialSubStatuses[status]; !ok {
		wsRedirect(w, r, "/deals/"+idStr, "sub_status")
		return nil, false
	}
	// Konflik one-active (keputusan c: TOLAK) — hanya menggigit bila status Active
	// (idx_subs_one_active WHERE status='Active'); Trial boleh koeksis. Pre-check beri
	// pesan ramah; index tetap penjaga keras bila balapan.
	if status == "Active" {
		exists, err := h.q(ctx).HasActiveSubscriptionForPlan(ctx, db.HasActiveSubscriptionForPlanParams{
			AccountID: deal.AccountID,
			PlanID:    *deal.PlanRequestedID,
		})
		if err != nil {
			h.Log.Error("deals: won sub active-check", "deal_id", deal.ID, "err", err)
			wsRedirect(w, r, "/deals/"+idStr, "failed")
			return nil, false
		}
		if exists {
			wsRedirect(w, r, "/deals/"+idStr, "sub_active_exists")
			return nil, false
		}
	}

	// Nilai: amount = nilai per termin → MRR = amount / bulan-kontrak; ARR = MRR × 12.
	mrr := divNumericInt(deal.Amount, int64(months))
	arr := mulNumericInt(mrr, monthsPerYear)

	// Tanggal: mulai hari ini (zona app), berakhir + bulan-kontrak (kalender eksak).
	start := todayInAppTZ()
	startDate := dateOnly(start)
	endDate := dateOnly(start.AddDate(0, int(months), 0))

	tenantID := session.TenantID(ctx)
	code, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntitySubscription)
	if err != nil {
		h.Log.Error("deals: won sub code", "deal_id", deal.ID, "err", err)
		wsRedirect(w, r, "/deals/"+idStr, "failed")
		return nil, false
	}

	billing := term
	termMonths := months
	sub, err := h.q(ctx).CreateSubscription(ctx, db.CreateSubscriptionParams{
		TenantID:           tenantID,
		EntityCode:         &code,
		SubscriptionOwner:  deal.DealOwner,
		AccountID:          deal.AccountID,
		PlanID:             *deal.PlanRequestedID,
		SourceDealID:       &deal.ID,
		Status:             status,
		StartDate:          startDate,
		EndDate:            endDate,
		BillingCycle:       &billing,
		AutoRenew:          false,
		ContractTermMonths: &termMonths,
		Mrr:                mrr,
		Arr:                arr,
		CreatedBy:          &uid,
	})
	if err != nil {
		h.Log.Error("deals: won sub create", "deal_id", deal.ID, "err", err)
		wsRedirect(w, r, "/deals/"+idStr, "failed")
		return nil, false
	}
	return &sub, true
}

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
