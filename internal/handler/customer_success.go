package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// customer_success.go — gerbang F2 per-section (Health/Journey+Onboarding/
// Adoption, Modul 6 slice B1) + halaman BACA. SATU baris `customer_success`
// per desa tapi TIGA objek Casbin berbeda (crm:health/crm:journey/crm:adoption)
// — section yang aktor tak berhak baca disembunyikan di view (di sini), section
// yang tak berhak tulis di-mask saat SAVE (customer_success_save.go), bukan di
// gerbang GET.
//
// F3 diwarisi desa induk (loadOwnedAccount) — CS TANPA ownership sendiri, sama
// seperti kb_articles/playbooks/sla_policies TANPA F3, tapi di sini lewat account
// bukan lewat RLS/workspace polos (mirip contacts.go).

func canReadCSHealth(ctx context.Context) bool  { return authz.CanBusiness(ctx, "crm:health", "read") }
func canWriteCSHealth(ctx context.Context) bool { return authz.CanBusiness(ctx, "crm:health", "write") }

func canReadCSJourney(ctx context.Context) bool { return authz.CanBusiness(ctx, "crm:journey", "read") }
func canWriteCSJourney(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:journey", "write")
}

func canReadCSAdoption(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:adoption", "read")
}
func canWriteCSAdoption(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:adoption", "write")
}

// canReadCS = boleh membuka halaman DETAIL bila berhak membaca MINIMAL satu
// section (mis. Support hanya Health) — section yang tak berhak disembunyikan
// di view, bukan seluruh halaman ditolak.
func canReadCS(ctx context.Context) bool {
	return canReadCSHealth(ctx) || canReadCSJourney(ctx) || canReadCSAdoption(ctx)
}

// canWriteCS = boleh membuka form SUNTING bila berhak menulis MINIMAL satu
// section — masking per-section terjadi saat SAVE, bukan saat gerbang GET ini.
func canWriteCS(ctx context.Context) bool {
	return canWriteCSHealth(ctx) || canWriteCSJourney(ctx) || canWriteCSAdoption(ctx)
}

// renderCSForbidden — 403 + penjelasan; mirror renderAccountsForbidden.
func (h *Handler) renderCSForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Customer Success", "/accounts", panel.CustomerSuccessForbidden())
}

// CustomerSuccessDetail — GET /w/{workspace}/accounts/{id}/customer-success.
// F3 via loadOwnedAccount; F2 minimal-satu-section via canReadCS. Baris CS
// absen (pgx.ErrNoRows) → empty-state, BUKAN 404: desanya tetap ada, cuma
// belum pernah diisi Customer Success-nya.
func (h *Handler) CustomerSuccessDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canReadCS(ctx) {
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
	exists := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("customer_success: get", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		exists = false
		cs = db.CustomerSuccess{AccountID: accountID}
	}

	base := wsPath(slugFromRequest(r), "")
	v := customerSuccessDetailView(ctx, base, account, cs, exists)
	h.renderWorkspaceShell(w, r, account.VillageName+" · Customer Success", "/accounts",
		panel.CustomerSuccessDetail(v))
}

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
	v := panel.CustomerSuccessFormView{
		Base:        accountBase,
		AccountName: account.VillageName,
		Action:      accountBase + "/customer-success",
		Err:         customerSuccessErrMsg(r.URL.Query().Get("err")),
		Fields:      customerSuccessFormFields(cs),

		CanWriteHealth:   canWriteCSHealth(ctx),
		CanWriteJourney:  canWriteCSJourney(ctx),
		CanWriteAdoption: canWriteCSAdoption(ctx),

		HealthStatuses:     healthStatusOptions,
		ScoreTrends:        scoreTrendOptions,
		LifecycleStages:    lifecycleStageOptions,
		OnboardingStatuses: onboardingStatusOptions,
		LoginFrequencies:   loginFrequencyOptions,
		UsageTrends:        usageTrendOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Customer Success", "/accounts", panel.CustomerSuccessForm(v))
}
