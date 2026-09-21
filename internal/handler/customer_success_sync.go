package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/desaplus"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// customer_success_sync.go — POST tombol "Sinkron dari Desa+" (BL-27): tarik
// telemetry produk (lastActivity/activeUsers/topFeatures) dan timpa HANYA 3
// kolom Adoption/Usage yang punya padanan API (last_login_date/active_users/
// key_features_used) + usage_data_source. Section lain (Health/Lifecycle/
// Onboarding, field manual Adoption lain) SELALU dipertahankan dari baris
// existing — pemetaan murni di customer_success_sync_map.go (unit-testable
// tanpa DB/HTTP).

// desaPlusClient — client HTTP opsional (BL-27), nil bila DESA_PLUS_URL/TOKEN
// kosong. Diinject saat startup lewat SetDesaPlusClient (dipanggil run.go),
// BUKAN disimpan di Handler — pola sama SetCSSPath dkk (lihat CLAUDE.md).
var desaPlusClient *desaplus.Client

// SetDesaPlusClient dipanggil run.go SEKALI saat startup bila
// cfg.DesaPlusEnabled(). Nil (default) = fitur mati, tombol sync tak tampil
// (lihat CanSync di customer_success_view.go).
func SetDesaPlusClient(c *desaplus.Client) {
	desaPlusClient = c
}

// CustomerSuccessSync — POST /w/{workspace}/accounts/{id}/customer-success/sync.
// Gerbang tulis cermin persis CustomerSuccessSave (Adoption numpang
// crm:journey, BL-169) + gerbang tambahan khusus telemetry (client
// terkonfigurasi, desa berkode, lalu hasil panggilan API itu sendiri).
func (h *Handler) CustomerSuccessSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteCSJourney(ctx) || IsReadOnly(ctx) {
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
	accountPath := "/accounts/" + strconv.FormatInt(accountID, 10)

	// BL-114: kunci sama persis CustomerSuccessSave — desa non-pelanggan tak
	// boleh disinkron walau tombol ter-POST langsung.
	hasLiveSub, err := h.q(ctx).AccountHasLiveSubscription(ctx, accountID)
	if err != nil {
		h.Log.Error("customer_success: sync live subscription", "err", err)
		wsRedirect(w, r, accountPath+"/customer-success", "failed")
		return
	}
	if !hasLiveSub {
		wsRedirect(w, r, accountPath+"/customer-success", "")
		return
	}

	if desaPlusClient == nil {
		wsRedirect(w, r, accountPath+"/customer-success", "desaplus_disabled")
		return
	}
	if account.VillageCode == nil || *account.VillageCode == "" {
		wsRedirect(w, r, accountPath+"/customer-success", "desaplus_no_code")
		return
	}

	summary, err := desaPlusClient.VillageSummary(ctx, *account.VillageCode)
	if err != nil {
		errCode := "desaplus_failed"
		switch {
		case errors.Is(err, desaplus.ErrVillageNotFound):
			errCode = "desaplus_not_found"
		case errors.Is(err, desaplus.ErrUnauthorized):
			errCode = "desaplus_unauthorized"
		}
		// err sudah pesan berkonteks TANPA token (lihat desaplus.Client) — aman dilog.
		h.Log.Warn("customer_success: sync desa-plus gagal", "account_id", accountID, "err", err)
		wsRedirect(w, r, accountPath+"/customer-success", errCode)
		return
	}

	existing, err := h.q(ctx).GetCustomerSuccessByAccountID(ctx, accountID)
	exists := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("customer_success: sync get existing", "err", err)
			wsRedirect(w, r, accountPath+"/customer-success", "failed")
			return
		}
		exists = false
		existing = db.CustomerSuccess{AccountID: accountID}
	}

	form := formFromCustomerSuccess(existing)
	form.LastLoginDate, form.ActiveUsers, form.KeyFeaturesUsed = mapDesaPlusSummary(existing, summary)
	hc := csHealthComputed{
		overallScore:         existing.OverallHealthScore,
		healthLastCalculated: existing.HealthLastCalculated,
		previousScore:        existing.PreviousHealthScore,
		previousCalculatedAt: existing.PreviousHealthCalculatedAt,
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	if !exists {
		if err := h.createCustomerSuccessRow(ctx, tenantID, accountID, uid, form, hc, "Product Telemetry"); err != nil {
			h.Log.Error("customer_success: sync create", "err", err)
			wsRedirect(w, r, accountPath+"/customer-success", "failed")
			return
		}
	} else if err := h.updateCustomerSuccessRow(ctx, accountID, uid, form, hc, "Product Telemetry"); err != nil {
		h.Log.Error("customer_success: sync update", "err", err)
		wsRedirect(w, r, accountPath+"/customer-success", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "customer_success.sync", tenantID, map[string]string{
		"account_id": strconv.FormatInt(accountID, 10),
	})
	wsRedirectOK(w, r, accountPath+"/customer-success", "synced")
}
