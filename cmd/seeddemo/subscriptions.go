package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions.go — ~28 langganan tersebar ke status yang BISA DICAPAI app
// (Trial/Active/Expired/Cancelled/Churned/PendingApproval). "Suspended" SENGAJA
// TAK di-seed (BL-22): nilai cadangan belum di-wire, tak ada aksi yang
// menghasilkannya — menyeed-nya cuma memunculkan baris di state mustahil saat QC.
// Enum DB tetap menerima "Suspended". Fokus dasbor Renewals: ≥10 Active
// dengan end_date di jendela 30 hari 2026-08-26..2026-09-25, sisanya Active
// end_date jauh (baseline ARR, sebagian sudah 'Renewed'). idx_subs_one_active
// (1 Active per tenant+account+plan) dijaga dgn memberi tiap baris Active
// account BERBEDA (index desa unik), plan boleh berulang.

const renewalWindowStart = 0 // hari sejak "hari ini" run — awal jendela due
const renewalWindowEnd = 30  // akhir jendela due (30 hari, sama dgn ListRenewals)
const dueSubsCount = 10      // Active dgn end_date di jendela due
const farActiveCount = 3     // Active dgn end_date jauh (baseline ARR)
const churnedCount = 6
const trialCount = 3
const expiredCount = 2
const pendingApprovalCount = 2
const cancelledCount = 2

func seedSubscriptions(ctx context.Context, q *db.Queries, tenantID int64, tag string, rng *rand.Rand, owner *int64, accounts []accountInfo, plans []int64) ([]int64, error) {
	var ids []int64
	seq := 0
	today := time.Now()

	// ---- Active — jendela due (2026-08-26 s.d. 2026-09-25) ----
	for i := 0; i < dueSubsCount; i++ {
		acc := accounts[seq%len(accounts)]
		planID := plans[seq%len(plans)]
		days := renewalWindowStart + rng.Intn(renewalWindowEnd-renewalWindowStart+1)
		s, err := createSubscription(ctx, q, tenantID, seq, "Active", rng, owner, acc.ID, planID,
			today.AddDate(-1, 0, 0), today.AddDate(0, 0, days))
		if err != nil {
			return ids, fmt.Errorf("subscription due #%d: %w", i+1, err)
		}
		ids = append(ids, s.ID)
		seq++

		// Sebagian diberi status/tahap renewal manual (UpdateCSRenewalAction)
		// agar Renewals report tak hanya berisi kolom kosong.
		if i%2 == 0 {
			stage := pick(rng, []string{"Outreach", "Negotiation", "Not Started"})
			risk := pick(rng, []string{"Low", "Medium", "High"})
			plan := "Hubungi PIC desa untuk konfirmasi perpanjangan sebelum jatuh tempo."
			nextAction := pgDate(today.AddDate(0, 0, 3+rng.Intn(10)))
			if _, err := q.UpdateCSRenewalAction(ctx, db.UpdateCSRenewalActionParams{
				RenewalStage:          &stage,
				RenewalRisk:           &risk,
				RenewalActionPlan:     &plan,
				RenewalNextActionDate: nextAction,
				RenewalOwner:          owner,
				UpdatedBy:             owner,
				ID:                    s.ID,
			}); err != nil {
				return ids, fmt.Errorf("renewal action sub #%d: %w", s.ID, err)
			}
		}
	}

	// ---- Active — end_date jauh (baseline ARR, sebagian sudah "Renewed") ----
	for i := 0; i < farActiveCount; i++ {
		acc := accounts[seq%len(accounts)]
		planID := plans[seq%len(plans)]
		s, err := createSubscription(ctx, q, tenantID, seq, "Active", rng, owner, acc.ID, planID,
			today.AddDate(-1, -6, 0), today.AddDate(1, rng.Intn(6), 0))
		if err != nil {
			return ids, fmt.Errorf("subscription baseline #%d: %w", i+1, err)
		}
		ids = append(ids, s.ID)
		seq++
	}

	// ---- Churned — campur Voluntary/Involuntary, pola createChurned/seedsubs ----
	churnSpecs := []struct {
		typ, reason string
		winBack     bool
	}{
		{"Voluntary", "Budget", true},
		{"Voluntary", "Competitor", false},
		{"Voluntary", "Dissatisfaction", true},
		{"Involuntary", "No Adoption", false},
		{"Involuntary", "Change of Leadership", true},
		{"Involuntary", "Feature Gap", false},
	}
	for i, c := range churnSpecs {
		acc := accounts[seq%len(accounts)]
		planID := plans[seq%len(plans)]
		s, err := createSubscription(ctx, q, tenantID, seq, "Churned", rng, owner, acc.ID, planID,
			today.AddDate(-1, 0, 0), pgTimeZero())
		if err != nil {
			return ids, fmt.Errorf("subscription churn #%d: %w", i+1, err)
		}
		lost, err := num(fmt.Sprintf("%d.00", 300_000+rng.Intn(1_700_000)))
		if err != nil {
			return ids, err
		}
		reason, typ := c.reason, c.typ
		winBack := c.winBack
		if err := q.ChurnSubscription(ctx, db.ChurnSubscriptionParams{
			Status:           "Churned",
			CancellationDate: pgDate(today.AddDate(0, 0, -(5 + rng.Intn(60)))),
			ChurnReason:      &reason,
			ChurnType:        &typ,
			ChurnNotes:       ptr("Dicatat otomatis oleh seeddemo untuk keperluan demo dasbor Churn."),
			LostValueMrr:     lost,
			WinBackEligible:  &winBack,
			UpdatedBy:        owner,
			ID:               s.ID,
		}); err != nil {
			return ids, fmt.Errorf("churn sub #%d: %w", s.ID, err)
		}
		ids = append(ids, s.ID)
		seq++
	}

	// ---- Trial — end_date = masa percobaan mendatang, belum ada MRR tetap ----
	for i := 0; i < trialCount; i++ {
		acc := accounts[seq%len(accounts)]
		planID := plans[seq%len(plans)]
		s, err := createSubscription(ctx, q, tenantID, seq, "Trial", rng, owner, acc.ID, planID,
			today.AddDate(0, 0, -rng.Intn(10)), today.AddDate(0, 0, 14+rng.Intn(16)))
		if err != nil {
			return ids, fmt.Errorf("subscription trial #%d: %w", i+1, err)
		}
		ids = append(ids, s.ID)
		seq++
	}

	// ---- Expired — end_date sudah lewat, tak diperpanjang (belum tercatat churn) ----
	for i := 0; i < expiredCount; i++ {
		acc := accounts[seq%len(accounts)]
		planID := plans[seq%len(plans)]
		s, err := createSubscription(ctx, q, tenantID, seq, "Expired", rng, owner, acc.ID, planID,
			today.AddDate(-1, -3, 0), today.AddDate(0, 0, -(10+rng.Intn(40))))
		if err != nil {
			return ids, fmt.Errorf("subscription expired #%d: %w", i+1, err)
		}
		ids = append(ids, s.ID)
		seq++
	}

	// ---- PendingApproval — upsell/renewal menunggu keputusan manager (00014) ----
	for i := 0; i < pendingApprovalCount; i++ {
		acc := accounts[seq%len(accounts)]
		planID := plans[seq%len(plans)]
		s, err := createSubscription(ctx, q, tenantID, seq, "PendingApproval", rng, owner, acc.ID, planID,
			today.AddDate(0, 0, -3), today.AddDate(0, 0, 10+rng.Intn(15)))
		if err != nil {
			return ids, fmt.Errorf("subscription pending-approval #%d: %w", i+1, err)
		}
		ids = append(ids, s.ID)
		seq++
	}

	// ---- Cancelled — dibatalkan langsung (beda dari Churned: tanpa jejak reason) ----
	for i := 0; i < cancelledCount; i++ {
		acc := accounts[seq%len(accounts)]
		planID := plans[seq%len(plans)]
		s, err := createSubscription(ctx, q, tenantID, seq, "Cancelled", rng, owner, acc.ID, planID,
			today.AddDate(0, -2, 0), today.AddDate(0, 0, -(1+rng.Intn(20))))
		if err != nil {
			return ids, fmt.Errorf("subscription cancelled #%d: %w", i+1, err)
		}
		ids = append(ids, s.ID)
		seq++
	}

	return ids, nil
}

// createSubscription = pembungkus CreateSubscription dgn variasi billing/
// payment/renewal wajar per status, pola createSub dari cmd/seedsubs.
func createSubscription(ctx context.Context, q *db.Queries, tenantID int64, seq int, status string, rng *rand.Rand, owner *int64, accountID, planID int64, start, end time.Time) (db.Subscription, error) {
	code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntitySubscription)
	if err != nil {
		return db.Subscription{}, fmt.Errorf("kode langganan: %w", err)
	}

	mrrBase := 300_000 + rng.Intn(2_700_000)
	mrr, err := num(fmt.Sprintf("%d.00", mrrBase))
	if err != nil {
		return db.Subscription{}, err
	}
	arr, err := num(fmt.Sprintf("%d.00", mrrBase*12))
	if err != nil {
		return db.Subscription{}, err
	}
	discount, err := num(fmt.Sprintf("%d.00", rng.Intn(4)*5)) // 0/5/10/15%
	if err != nil {
		return db.Subscription{}, err
	}

	billing := pick(rng, []string{"Monthly", "Quarterly", "Annual", "Multi-year"})
	seats := intn32(rng, 1, 6)

	var endDate pgtype.Date
	if !end.IsZero() {
		endDate = pgDate(end)
	}

	var paymentStatus *string
	var renewalStatus, renewalType *string
	autoRenew := false
	switch status {
	case "Active", "PendingApproval":
		paymentStatus = ptr(pick(rng, []string{"Paid", "Paid", "Pending"}))
		autoRenew = rng.Intn(100) < 60
		if end.Before(time.Now().AddDate(0, 0, 40)) && !end.IsZero() {
			renewalStatus = ptr(weightedPick(rng, []weighted[string]{
				{"Upcoming", 5}, {"In Progress", 3}, {"At Risk", 2},
			}))
		} else {
			renewalStatus = ptr("Renewed")
		}
		renewalType = ptr(pick(rng, []string{"Auto", "Manual", "Upsell"}))
	case "Expired", "Cancelled":
		paymentStatus = ptr(pick(rng, []string{"Paid", "Partial"}))
		renewalStatus = ptr("Not Renewed")
	case "Trial":
		paymentStatus = ptr("Pending")
	case "Churned":
		paymentStatus = ptr(pick(rng, []string{"Paid", "Overdue"}))
	}

	var approvalStatus *string
	if status == "PendingApproval" {
		approvalStatus = ptr("Pending")
	}

	var subOwner *int64
	if rng.Intn(100) < 70 {
		subOwner = owner
	}

	s, err := q.CreateSubscription(ctx, db.CreateSubscriptionParams{
		TenantID:           tenantID,
		EntityCode:         &code,
		SubscriptionOwner:  subOwner,
		AccountID:          accountID,
		PlanID:             planID,
		Status:             status,
		ApprovalStatus:     approvalStatus,
		StartDate:          pgDate(start),
		EndDate:            endDate,
		BillingCycle:       &billing,
		AutoRenew:          autoRenew,
		ContractTermMonths: ptr(intn32(rng, 6, 36)),
		Mrr:                mrr,
		Arr:                arr,
		QuantitySeats:      &seats,
		DiscountPct:        discount,
		PaymentStatus:      paymentStatus,
		RenewalStatus:      renewalStatus,
		RenewalType:        renewalType,
		CreatedBy:          owner,
	})
	if err != nil {
		return db.Subscription{}, fmt.Errorf("buat langganan #%d (%s): %w", seq, status, err)
	}
	return s, nil
}

// pgTimeZero mengembalikan time.Time nol — dipakai sbg penanda "belum ada
// end_date pasti" sebelum ChurnSubscription mengisi cancellation_date sendiri.
func pgTimeZero() time.Time { return time.Time{} }
