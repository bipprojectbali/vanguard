package handler

import (
	"context"
	"errors"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// subscriptions_detail_view.go — subDetailView (BL-154 redesign kartu 2-kolom),
// dipisah dari subscriptions_detail.go (handler HTTP murni) demi ambang tipe
// Route/Handler (150). Package sama; F4 maskARR/maskSubscriptionARR identik.

// subDetailView merakit detail lengkap + F4 (ARR disamarkan) + riwayat rantai
// renewal + (BL-154) Health/Onboarding lintas-modul, audit sistem, & source deal.
// Nama plan & desa diresolusi best-effort (di luar tenant/terhapus → label
// cadangan, tak menggagalkan halaman).
func (h *Handler) subDetailView(ctx context.Context, base string, s db.Subscription, names map[int64]string) panel.SubDetailView {
	// BL-58/BL-169: canSeeARR/canSeeSubscriptionARR = kapabilitas Casbin ter-matriks
	// yang SAMA (disatukan BL-169) — satu bool dihitung sekali, dipakai untuk semua
	// nilai komersial (MRR/ARR/Subtotal) di halaman ini.
	canARR := canSeeSubscriptionARR(ctx)
	now := todayInAppTZ()

	// Item paket (BL-88 PR2b): best-effort (gagal → nil, detail tetap terbaca).
	// >1 item → label "N paket" (parent plan_id NULL); 1 item → nama paket item
	// itu; 0 item (langganan lama tanpa item) → planLabel(parent) cadangan.
	items, err := h.q(ctx).ListSubscriptionItemsWithPlan(ctx, s.ID)
	if err != nil {
		h.Log.Error("subscriptions: items", "err", err)
	}
	planDisplay := h.planLabel(ctx, s.PlanID)
	switch {
	case len(items) > 1:
		planDisplay = strconv.Itoa(len(items)) + " paket"
	case len(items) == 1 && items[0].PlanName != nil && *items[0].PlanName != "":
		planDisplay = *items[0].PlanName
	}

	// BL-154 Status & Lifecycle: Health Score + Onboarding, lookup lintas-modul
	// ke customer_success (satu baris per account, sumber SAMA dgn BL-114).
	// Best-effort (mirror customerSuccessSummaryFor, accounts_rollups.go):
	// pgx.ErrNoRows → field kosong ("—" via view), bukan gagal request. SENGAJA
	// tanpa gate F2 crm:health tambahan — konsisten preseden kartu ringkasan
	// Account Detail: F3 ownership subscription yang sudah ditegakkan handler
	// cukup. FLS: Health Score & Onboarding TANPA masking (fls.go melarang
	// menambah gate; sejalan customerSuccessSummaryFor).
	var healthLabel, healthBadge, onboardingStatus, onboardingProgress, activatedAt string
	if cs, csErr := h.q(ctx).GetCustomerSuccessByAccountID(ctx, s.AccountID); csErr != nil {
		if !errors.Is(csErr, pgx.ErrNoRows) {
			h.Log.Error("subscriptions: customer success", "err", csErr)
		}
	} else {
		healthLabel, healthBadge = healthScoreStatus(cs.HealthStatus)
		onboardingStatus = deref(cs.OnboardingStatus)
		onboardingProgress = probabilityStr(cs.OnboardingProgress)
		activatedAt = dateStr(cs.ActualGoLiveDate) // "Activated At" ~ go-live aktual (lebih dekat maknanya drpd approved_at/start_date)
	}

	// BL-154 Renewal: status derivasi & sisa hari REUSE persis logika kartu/tab
	// Renewals (subscriptions_renewals_row.go) agar badge di sini selaras.
	renewalStatusLabel, renewalStatusBadge := renewalDerivedStatus(now, s.Status, s.EndDate, s.RenewalStatus)
	daysToRenewal := daysLeftLabel(now, s.EndDate)
	// Tipe Perpanjangan = renewal_type bila terisi, fallback Auto/Manual dari
	// auto_renew (renewalTypeLabel) — menggabungkan 2 field lama (RenewalType +
	// AutoRenew) jadi satu field mockup, tanpa regresi data (keputusan desain BL-154).
	renewalTypeDisplay := renewalTypeLabel(s.RenewalType, s.AutoRenew)
	prevToCurrent := maskARR(formatRupiah(s.PreviousValue), canARR) + " → " + maskARR(formatRupiah(s.Mrr), canARR)

	// BL-154 System & Audit: pembuat/pengubah + waktu (mirror dealSystemAuditCard/
	// auditViewFor — metadata operasional, bukan data sensitif, tanpa masking F4).
	// Source deal: dealLabel best-effort (mirror accountLabel/planLabel), nil →
	// "—" tanpa query (bukan error — banyak langganan tak berasal dari deal).
	sourceDealLabel := "—"
	sourceDealHref := ""
	if s.SourceDealID != nil {
		sourceDealLabel = h.dealLabel(ctx, *s.SourceDealID)
		sourceDealHref = base + "/deals/" + strconv.FormatInt(*s.SourceDealID, 10)
	}

	return panel.SubDetailView{
		Base:         base,
		ID:           s.ID,
		EntityCode:   deref(s.EntityCode),
		Village:      h.accountLabel(ctx, s.AccountID),
		AccountID:    s.AccountID,
		Plan:         planDisplay,
		Items:        subItemRows(items, canARR),
		Status:       s.Status,
		MRR:          maskARR(formatRupiah(s.Mrr), canARR),
		ARR:          maskSubscriptionARR(formatRupiah(s.Arr), canARR),
		BillingCycle: deref(s.BillingCycle),
		Start:        dateStr(s.StartDate),
		End:          dateStr(s.EndDate),
		Seats:        int32Str(s.QuantitySeats),
		PaymentState: deref(s.PaymentStatus),
		Owner:        ownerName(s.SubscriptionOwner, names),
		Chain:        h.renewalChainView(ctx, s.ID, canARR),

		// BL-154 — kartu Identitas & Langganan (tambahan): Contract Term.
		ContractTerm: contractTermStr(s.ContractTermMonths),
		// BL-154 — kartu Financials (tambahan): Discount (%). Format polos angka
		// (mirror sales_quotes_detail.go: Discount: numericStr(it.DiscountPct)),
		// unit "%" di label kolom, bukan disisipkan ke nilai.
		Discount: numericStr(s.DiscountPct),

		// BL-154 — kartu Status & Lifecycle.
		HealthLabel:        healthLabel,
		HealthBadgeClass:   healthBadge,
		OnboardingStatus:   onboardingStatus,
		OnboardingProgress: onboardingProgress,
		ActivatedAt:        activatedAt,

		// BL-154 — kartu Renewal.
		DaysToRenewal:      daysToRenewal,
		RenewalTypeLabel:   renewalTypeDisplay,
		RenewalStatusLabel: renewalStatusLabel,
		RenewalStatusClass: renewalStatusBadge,
		PrevToCurrent:      prevToCurrent,

		// BL-154 — kartu System & Audit.
		CreatedByName:   ownerName(s.CreatedBy, names),
		CreatedAt:       fmtDateTime(s.CreatedAt),
		UpdatedByName:   ownerName(s.UpdatedBy, names),
		UpdatedAt:       fmtDateTime(s.UpdatedAt),
		SourceDealLabel: sourceDealLabel,
		SourceDealHref:  sourceDealHref,

		// Flag aksi (M5-3c) diprecompute di sini — view murni-data (tak panggil authz).
		CanRenew:     canRenewSubscriptions(ctx),
		CanChurn:     canChurnSubscriptions(ctx),
		CanApprove:   canApproveRenewal(ctx),
		CanActivate:  canActivateSubscriptions(ctx),
		ChurnReasons: churnReasonOptions,
		ChurnTypes:   churnTypeOptions,
	}
}

// contractTermStr memformat contract_term_months (*int32) opsional: nil → "".
// Unit "bulan" ditaruh di label kolom (konsisten formatter lain di paket ini —
// int32Str/numericStr tak menyisipkan unit ke nilai).
func contractTermStr(months *int32) string {
	return int32Str(months)
}
