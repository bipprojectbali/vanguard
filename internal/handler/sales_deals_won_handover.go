package handler

import (
	"context"
	"errors"
	"strconv"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5"
)

// sales_deals_won_handover.go — BL-74: deal Closed Won → auto-create baris
// customer_success (handover Sales→CS). Sampai kini SATU-SATUNYA pemicu dari Sales
// ke CS adalah pembuatan subscription (BL-21); tabel customer_success (yang memuat
// onboarding_status) hanya lahir MANUAL saat seseorang membuka tab CS desa. Akibat:
// tim CS tak dapat sinyal terstruktur, onboarding bisa terlewat.
//
// customer_success.account_id UNIQUE REFERENCES accounts(id) (00019) — terikat ke
// DESA, bukan subscription (tak ada kolom subscription_id). Maka onboarding
// INDEPENDEN dari status langganan: pemicu yang benar = Deal Closed Won, apa pun
// status subscription awal (Trial ATAU Active), BUKAN "saat subscription Active"
// (menunggu Active = handover terlambat). Independen pula dari BL-73 (aktivasi
// Trial→Active urusan komersial terpisah; onboarding tak menunggu aktivasi).

// handoverCSFromWonDeal membuat baris customer_success default untuk desa deal yang
// baru di-Closed Won. FAIL-SOFT (keputusan user 7 Sep): kegagalan hanya di-log, TAK
// membatalkan deal Won / subscription — baris CS bisa dibuat manual belakangan lewat
// tab CS, sedangkan subscription = inti komersial. Di arsitektur satu-tx yang selalu
// commit, query gagal akan meracuni tx → SELURUH request rollback; maka seluruh
// baca-lalu-tulis dikurung db.WithSavepoint agar kegagalannya terurung ke savepoint
// (deal + subscription tetap selamat).
//
// IDEMPOTEN: account_id UNIQUE menolak baris kedua & pelanggarannya meng-abort tx —
// jadi WAJIB SELECT dulu (pola allocEntityCode). Baris CS sudah ada → SKIP, JANGAN
// overwrite onboarding/health yang mungkin sudah diisi CS. Default baris baru:
// onboarding_status="Not Started" + lifecycle_stage="Onboarding" (turunan health
// dibiarkan NULL; operator/CSM isi kemudian). Kombinasi ini aman terhadap guard
// konsistensi K1/K3 (bukan post-onboarding stage, go-live kosong). created_by = aktor
// Sales yang meng-Won-kan deal. Notifikasi CSM SENGAJA di luar cakupan (Opsi B).
func (h *Handler) handoverCSFromWonDeal(ctx context.Context, deal db.Deal, tenantID, uid int64) {
	stage := lifecycleOnboarding
	status := onboardingNotStarted
	created := false

	err := db.WithSavepoint(ctx, h.q(ctx), func(q *db.Queries) error {
		// Idempotensi: baris sudah ada → skip (release savepoint tanpa menulis).
		if _, err := q.GetCustomerSuccessByAccountID(ctx, deal.AccountID); err == nil {
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// Belum ada → buat baris handover minimal (reuse query CreateCustomerSuccess).
		if _, err := q.CreateCustomerSuccess(ctx, db.CreateCustomerSuccessParams{
			TenantID:         tenantID,
			AccountID:        deal.AccountID,
			LifecycleStage:   &stage,
			OnboardingStatus: &status,
			CreatedBy:        &uid,
		}); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		// Fail-soft: log & lanjut — deal Won + subscription tetap sah.
		h.Log.Error("deals: won cs handover", "deal_id", deal.ID, "account_id", deal.AccountID, "err", err)
		return
	}
	if created {
		h.auditWorkspace(ctx, uid, "cs.handover.created", tenantID, map[string]string{
			"deal_id":    strconv.FormatInt(deal.ID, 10),
			"account_id": strconv.FormatInt(deal.AccountID, 10),
		})
	}
}
