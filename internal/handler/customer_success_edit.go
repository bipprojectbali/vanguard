package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// customer_success_edit.go — form GET sunting Customer Success (CustomerSuccessEdit:
// gate canWriteCS, prefill + badge read-only BL-24/25 + peringatan BL-26), dipisah
// dari customer_success.go (gerbang F2 + halaman BACA) demi ambang tipe Route/Handler
// (150). Package sama; masking per-section tetap saat SAVE.

// CustomerSuccessEdit — GET /w/{workspace}/accounts/{id}/customer-success/edit.
// Gerbang canWriteCS (minimal satu section); prefill dari baris existing atau
// kosong (jalur create pertama kali).
func (h *Handler) CustomerSuccessEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteCS(ctx) {
		h.renderCSForbidden(w, r)
		return
	}
	accountID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	account, ok := h.loadOwnedAccount(w, r, accountID)
	if !ok {
		return
	}

	cs, err := h.q(ctx).GetCustomerSuccessByAccountID(ctx, accountID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		h.Log.Error("customer_success: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	accountBase := base + "/accounts/" + strconv.FormatInt(accountID, 10)
	// Status kesehatan = badge read-only (BL-24): precompute label+kelas badge
	// dari status TERSIMPAN di handler (view murni-data). nil → "—"/ghost.
	healthLabel, healthBadge := healthScoreStatus(cs.HealthStatus)
	// Tren skor = badge read-only (BL-25): label panah + kelas dari nilai TERSIMPAN.
	trendLabel, trendBadge := healthScoreTrend(cs.ScoreTrend), scoreTrendBadge(cs.ScoreTrend)
	// BL-26: peringatan keselarasan (K2/K4) atas baris TERSIMPAN, HANYA bila
	// aktor berhak menulis Journey (kartu itu memang dirender untuknya).
	var warnings []string
	if canWriteCSJourney(ctx) {
		warnings = onboardingConsistencyWarnings(cs)
	}
	v := panel.CustomerSuccessFormView{
		Base:        accountBase,
		AccountName: account.VillageName,
		Action:      accountBase + "/customer-success",
		Err:         customerSuccessErrMsg(r.URL.Query().Get("err")),
		Fields:      customerSuccessFormFields(cs),
		Warnings:    warnings,

		CanWriteHealth:   canWriteCSHealth(ctx),
		CanWriteJourney:  canWriteCSJourney(ctx),
		CanWriteAdoption: canWriteCSAdoption(ctx),

		HealthStatusLabel:  healthLabel,
		HealthStatusBadge:  healthBadge,
		ScoreTrendLabel:    trendLabel,
		ScoreTrendBadge:    trendBadge,
		LifecycleStages:    lifecycleStageOptions,
		OnboardingStatuses: onboardingStatusOptions,
		LoginFrequencies:   loginFrequencyOptions,
		UsageTrends:        usageTrendOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Customer Success", "/accounts", panel.CustomerSuccessForm(v))
}
