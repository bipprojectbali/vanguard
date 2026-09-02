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
	writeJourney := canWriteCSJourney(ctx)
	form = applyCustomerSuccessMasking(form, existing, writeHealth, writeJourney, canWriteCSAdoption(ctx))

	// BL-26: normalisasi & keselarasan onboarding ↔ lifecycle HANYA saat section
	// Journey berhak ditulis aktor (F2) — bila di-mask ke nilai lama, jangan
	// galat palsu atau normalisasi paksa nilai yang bukan milik aksi ini.
	if writeJourney {
		// (c) progress terminal mengikuti status: Not Started→0, Completed→100
		// (field tersembunyi di UI tetap bisa kirim nilai basi — tegakkan di sini).
		form.OnboardingProgress = normalizeOnboardingProgress(form.OnboardingStatus, form.OnboardingProgress)
		// guard K1/K3 (kontradiksi mustahil) → tolak simpan, PRG ?err= (gotcha #16).
		if code, ok := checkOnboardingLifecycleConsistency(form); !ok {
			wsRedirect(w, r, accountPath+"/customer-success/edit", code)
			return
		}
	}

	// overall_health_score DIHITUNG, bukan dari form (tak ada field submit-nya):
	// rata-rata komponen non-NULL, dibulatkan. health_last_calculated hanya maju
	// ke sekarang bila section Health BENAR-BENAR ditulis kali ini (bukan di-mask
	// ke nilai lama) — kolom berarti "kapan terakhir dihitung ulang".
	overallScore := computeOverallHealthScore(form.AdoptionScore, form.EngagementScore, form.SupportScore, form.SentimentScore)
	// health_status kini TURUNAN skor, bukan input operator (BL-24): apa pun
	// yang dikirim form diabaikan, status mengikuti overall_health_score. Karena
	// overallScore sudah dihitung dari komponen yang TER-MASK ke nilai lama saat
	// section Health tak berhak ditulis, status ikut nilai lama tanpa special-case.
	form.HealthStatus = deriveHealthStatus(overallScore)

	// score_trend TURUNAN riwayat skor (BL-25): saat section Health ditulis kali
	// ini, geser overall_health_score LAMA → previous_* SEBELUM menyimpan nilai
	// baru, lalu deriveScoreTrend(previous, sekarang) mengisi arah (Improving/
	// Stable/Declining, dead-band healthTrendDeadband). Bila Health TAK ditulis
	// (di-mask ke nilai lama) pertahankan trend & previous_* existing — tak ada
	// perubahan skor untuk dibandingkan. Manual dari form DIABAIKAN (tak lagi
	// diparse; badge read-only, pola BL-24 untuk health_status). Snapshot pertama
	// (existing.OverallHealthScore NULL) → trend NULL ("—", belum ada dasar).
	healthLastCalculated := existing.HealthLastCalculated
	previousScore := existing.PreviousHealthScore
	previousCalculatedAt := existing.PreviousHealthCalculatedAt
	if writeHealth {
		form.ScoreTrend = deriveScoreTrend(existing.OverallHealthScore, overallScore, healthTrendDeadband)
		previousScore = existing.OverallHealthScore
		previousCalculatedAt = existing.HealthLastCalculated
		healthLastCalculated = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	} else {
		form.ScoreTrend = existing.ScoreTrend
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	hc := csHealthComputed{
		overallScore:         overallScore,
		healthLastCalculated: healthLastCalculated,
		previousScore:        previousScore,
		previousCalculatedAt: previousCalculatedAt,
	}
	okCode := "saved"
	if !exists {
		if err := h.createCustomerSuccessRow(ctx, tenantID, accountID, uid, form, hc); err != nil {
			h.Log.Error("customer_success: create", "err", err)
			wsRedirect(w, r, accountPath+"/customer-success/edit", "failed")
			return
		}
		okCode = "created"
	} else {
		if err := h.updateCustomerSuccessRow(ctx, accountID, uid, form, hc); err != nil {
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
