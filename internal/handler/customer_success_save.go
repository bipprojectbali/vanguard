package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_save.go — POST simpan Customer Success (get-then-branch, F2
// masking per-section). SATU baris `customer_success` per desa tapi TIGA objek
// Casbin — aktor yang hanya berhak menulis satu section (mis. Support: Health
// saja) tetap mengirim form utuh (view menyembunyikan kartu yang tak berhak
// SEHINGGA field-nya tak pernah terkirim); di sini kita jangan percaya form
// kosong = "kosongkan section itu" — section yang tak berhak ditulis WAJIB
// dipertahankan dari baris existing (nil/zero bila baris belum ada).

// CustomerSuccessSave — POST /w/{workspace}/accounts/{id}/customer-success.
// Gerbang canWriteCS (minimal satu section) + F3 via loadOwnedAccount.
func (h *Handler) CustomerSuccessSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteCS(ctx) {
		h.renderCSForbidden(w, r)
		return
	}
	accountID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedAccount(w, r, accountID); !ok {
		return
	}
	accountPath := "/accounts/" + strconv.FormatInt(accountID, 10)

	form, errCode := parseCustomerSuccessForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, accountPath+"/customer-success/edit", errCode)
		return
	}

	existing, err := h.q(ctx).GetCustomerSuccessByAccountID(ctx, accountID)
	exists := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("customer_success: get", "err", err)
			wsRedirect(w, r, accountPath+"/customer-success/edit", "failed")
			return
		}
		exists = false
		existing = db.CustomerSuccess{AccountID: accountID}
	}

	// F2 masking per-section: aktor yang tak berhak menulis section ini
	// mempertahankan nilai LAMA (nil/zero bila baris belum ada — section yang
	// tak berhak ditulis pada baris baru otomatis NULL, bukan galat). Logika
	// murni di applyCustomerSuccessMasking (customer_success_mask.go) agar bisa
	// diuji langsung tanpa HTTP.
	writeHealth := canWriteCSHealth(ctx)
	form = applyCustomerSuccessMasking(form, existing, writeHealth, canWriteCSJourney(ctx), canWriteCSAdoption(ctx))

	// overall_health_score DIHITUNG, bukan dari form (tak ada field submit-nya):
	// rata-rata komponen non-NULL, dibulatkan. health_last_calculated hanya maju
	// ke sekarang bila section Health BENAR-BENAR ditulis kali ini (bukan di-mask
	// ke nilai lama) — kolom berarti "kapan terakhir dihitung ulang".
	overallScore := computeOverallHealthScore(form.AdoptionScore, form.EngagementScore, form.SupportScore, form.SentimentScore)
	healthLastCalculated := existing.HealthLastCalculated
	if writeHealth {
		healthLastCalculated = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	okCode := "saved"
	if !exists {
		if _, err := h.q(ctx).CreateCustomerSuccess(ctx, db.CreateCustomerSuccessParams{
			TenantID:             tenantID,
			AccountID:            accountID,
			OverallHealthScore:   overallScore,
			HealthStatus:         form.HealthStatus,
			AdoptionScore:        form.AdoptionScore,
			EngagementScore:      form.EngagementScore,
			SupportScore:         form.SupportScore,
			SentimentScore:       form.SentimentScore,
			ScoreTrend:           form.ScoreTrend,
			HealthLastCalculated: healthLastCalculated,
			LifecycleStage:       form.LifecycleStage,
			StageEntryDate:       form.StageEntryDate,
			OnboardingStatus:     form.OnboardingStatus,
			KickoffDate:          form.KickoffDate,
			TargetGoLiveDate:     form.TargetGoLiveDate,
			ActualGoLiveDate:     form.ActualGoLiveDate,
			OnboardingProgress:   form.OnboardingProgress,
			LastLoginDate:        form.LastLoginDate,
			ActiveUsers:          form.ActiveUsers,
			LoginFrequency:       form.LoginFrequency,
			FeatureAdoptionRate:  form.FeatureAdoptionRate,
			KeyFeaturesUsed:      form.KeyFeaturesUsed,
			UsageTrend:           form.UsageTrend,
			CreatedBy:            &uid,
		}); err != nil {
			h.Log.Error("customer_success: create", "err", err)
			wsRedirect(w, r, accountPath+"/customer-success/edit", "failed")
			return
		}
		okCode = "created"
	} else {
		if _, err := h.q(ctx).UpdateCustomerSuccess(ctx, db.UpdateCustomerSuccessParams{
			OverallHealthScore:   overallScore,
			HealthStatus:         form.HealthStatus,
			AdoptionScore:        form.AdoptionScore,
			EngagementScore:      form.EngagementScore,
			SupportScore:         form.SupportScore,
			SentimentScore:       form.SentimentScore,
			ScoreTrend:           form.ScoreTrend,
			HealthLastCalculated: healthLastCalculated,
			LifecycleStage:       form.LifecycleStage,
			StageEntryDate:       form.StageEntryDate,
			OnboardingStatus:     form.OnboardingStatus,
			KickoffDate:          form.KickoffDate,
			TargetGoLiveDate:     form.TargetGoLiveDate,
			ActualGoLiveDate:     form.ActualGoLiveDate,
			OnboardingProgress:   form.OnboardingProgress,
			LastLoginDate:        form.LastLoginDate,
			ActiveUsers:          form.ActiveUsers,
			LoginFrequency:       form.LoginFrequency,
			FeatureAdoptionRate:  form.FeatureAdoptionRate,
			KeyFeaturesUsed:      form.KeyFeaturesUsed,
			UsageTrend:           form.UsageTrend,
			UpdatedBy:            &uid,
			AccountID:            accountID,
		}); err != nil {
			h.Log.Error("customer_success: update", "err", err)
			wsRedirect(w, r, accountPath+"/customer-success/edit", "failed")
			return
		}
	}

	h.auditWorkspace(ctx, uid, "customer_success.save", tenantID, map[string]string{
		"account_id": strconv.FormatInt(accountID, 10),
	})
	wsRedirectOK(w, r, accountPath+"/customer-success", okCode)
}
