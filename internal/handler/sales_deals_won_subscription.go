package handler

import (
	"net/http"
	"strconv"

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
//
// Validasi & pengumpulan input (quote+termin, status, item quote, plan induk,
// cek one-active) di sales_deals_won_subscription_input.go.

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
	in, ok := h.resolveWonSubscriptionInputs(w, r, deal)
	if !ok {
		return nil, false
	}
	months := in.months

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

	billing := in.term
	termMonths := months
	sub, err := h.q(ctx).CreateSubscription(ctx, db.CreateSubscriptionParams{
		TenantID:           tenantID,
		EntityCode:         &code,
		SubscriptionOwner:  deal.DealOwner,
		AccountID:          deal.AccountID,
		PlanID:             in.parentPlan,
		SourceDealID:       &deal.ID,
		Status:             in.status,
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

	// BL-88 PR2a: salin baris quote_items → subscription_items (tulis ganda). Snapshot
	// komersial (unit_price/discount_pct/subtotal) mengikuti item quote; mrr/arr per-item
	// diturunkan dari subtotal item (bukan grand_total) — Σ item.mrr bisa selisih tipis
	// dari parent.mrr bila quote punya diskon/pembulatan, item mengikuti subtotalnya.
	// FAIL-SOFT: parent langganan sudah lahir; gagal buat item TAK menggagalkan Won
	// (hanya di-log). items sudah diambil resolveWonSubscriptionInputs (dipakai turunkan
	// plan_id parent).
	for _, it := range in.items {
		imrr := divNumericInt(it.Subtotal, int64(months))
		iarr := mulNumericInt(imrr, monthsPerYear)
		if _, err := h.q(ctx).AddSubscriptionItem(ctx, db.AddSubscriptionItemParams{
			SubscriptionID: sub.ID,
			TenantID:       tenantID,
			PlanID:         it.PlanID,
			Quantity:       it.Quantity,
			UnitPrice:      it.UnitPrice,
			DiscountPct:    it.DiscountPct,
			Subtotal:       it.Subtotal,
			Mrr:            imrr,
			Arr:            iarr,
			LineNo:         it.LineNo,
		}); err != nil {
			h.Log.Error("deals: won sub item add", "sub_id", sub.ID, "err", err)
		}
	}
	return &sub, true
}
