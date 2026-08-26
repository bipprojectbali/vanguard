package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs.go — customer_success 1:1 dgn account (UNIQUE non-partial, maks 1 baris
// per desa). Cakupan ~75% dari 40 desa (30 baris): 16 Healthy/8 At-Risk/
// 4 Critical/2 NULL (dinilai tapi belum dikategorikan) — 10 desa SISANYA
// sengaja tanpa baris sama sekali (bucket "Belum Dinilai" di health report).

func seedCustomerSuccess(ctx context.Context, q *db.Queries, tenantID int64, rng *rand.Rand, accounts []accountInfo) (int, error) {
	specs := []struct {
		status string
		count  int
	}{
		{"Healthy", 16}, {"At-Risk", 8}, {"Critical", 4}, {"", 2}, // "" = dinilai tanpa health_status
	}

	count := 0
	idx := 0
	today := time.Now()
	for _, spec := range specs {
		for i := 0; i < spec.count; i++ {
			if idx >= len(accounts) {
				return count, nil
			}
			acc := accounts[idx]
			idx++

			scores := scoresFor(rng, spec.status)
			var healthStatus *string
			if spec.status != "" {
				healthStatus = ptr(spec.status)
			}
			overall := int16((int(scores.adoption) + int(scores.engagement) + int(scores.support) + int(scores.sentiment)) / 4)

			lifecycle := lifecycleFor(spec.status, rng)
			onboarding, kickoff, targetGoLive, actualGoLive, progress := onboardingFor(rng, today)

			lastLogin := pgDate(today.AddDate(0, 0, -rng.Intn(30)))
			activeUsers := intn32(rng, 1, 25)
			loginFreq := loginFrequencyFor(spec.status, rng)
			adoptionRate, err := num(fmt.Sprintf("%d.00", 30+rng.Intn(65)))
			if err != nil {
				return count, err
			}
			usageTrend := usageTrendFor(spec.status, rng)
			features := pick(rng, []string{
				"Presensi Perangkat Desa, Pelaporan APBDes",
				"Pelaporan APBDes, Surat Menyurat",
				"Presensi, Pengaduan Warga, Pelaporan APBDes",
				"Surat Menyurat, Arsip Digital",
			})

			if _, err := q.CreateCustomerSuccess(ctx, db.CreateCustomerSuccessParams{
				TenantID:             tenantID,
				AccountID:            acc.ID,
				OverallHealthScore:   &overall,
				HealthStatus:         healthStatus,
				AdoptionScore:        &scores.adoption,
				EngagementScore:      &scores.engagement,
				SupportScore:         &scores.support,
				SentimentScore:       &scores.sentiment,
				ScoreTrend:           scoreTrendFor(spec.status, rng),
				HealthLastCalculated: pgtype.Timestamptz{Time: today.AddDate(0, 0, -rng.Intn(5)), Valid: true},
				LifecycleStage:       &lifecycle,
				StageEntryDate:       pgDate(today.AddDate(0, 0, -rng.Intn(180))),
				OnboardingStatus:     &onboarding,
				KickoffDate:          kickoff,
				TargetGoLiveDate:     targetGoLive,
				ActualGoLiveDate:     actualGoLive,
				OnboardingProgress:   &progress,
				LastLoginDate:        lastLogin,
				ActiveUsers:          &activeUsers,
				LoginFrequency:       &loginFreq,
				FeatureAdoptionRate:  adoptionRate,
				KeyFeaturesUsed:      &features,
				UsageTrend:           usageTrend,
			}); err != nil {
				return count, fmt.Errorf("customer_success desa #%d: %w", acc.ID, err)
			}
			count++
		}
	}
	return count, nil
}

type csScores struct{ adoption, engagement, support, sentiment int16 }

// scoresFor menghasilkan 4 skor komponen berkorelasi status — Healthy tinggi,
// Critical rendah, "" (belum dikategorikan) tengah-tengah/campur.
func scoresFor(rng *rand.Rand, status string) csScores {
	rangeFor := func(lo, hi int32) int16 { return int16(intn32(rng, lo, hi)) }
	switch status {
	case "Healthy":
		return csScores{rangeFor(70, 95), rangeFor(65, 95), rangeFor(70, 100), rangeFor(65, 95)}
	case "At-Risk":
		return csScores{rangeFor(40, 65), rangeFor(35, 60), rangeFor(30, 65), rangeFor(35, 60)}
	case "Critical":
		return csScores{rangeFor(10, 35), rangeFor(10, 35), rangeFor(5, 40), rangeFor(10, 35)}
	default:
		return csScores{rangeFor(40, 80), rangeFor(40, 80), rangeFor(40, 80), rangeFor(40, 80)}
	}
}

func scoreTrendFor(status string, rng *rand.Rand) *string {
	switch status {
	case "Healthy":
		return ptr(pick(rng, []string{"Improving", "Stable", "Stable"}))
	case "Critical":
		return ptr(pick(rng, []string{"Declining", "Declining", "Stable"}))
	case "At-Risk":
		return ptr(pick(rng, []string{"Declining", "Stable"}))
	default:
		return nil
	}
}

func lifecycleFor(status string, rng *rand.Rand) string {
	if status == "Critical" {
		return pick(rng, []string{"Retention", "Adoption"})
	}
	return pick(rng, []string{"Onboarding", "Adoption", "Retention", "Renewal", "Advocacy"})
}

func onboardingFor(rng *rand.Rand, today time.Time) (status string, kickoff, targetGoLive, actualGoLive pgtype.Date, progress int16) {
	status = weightedPick(rng, []weighted[string]{
		{"Completed", 6}, {"In Progress", 2}, {"Not Started", 1}, {"Stalled", 1},
	})
	kickoff = pgDate(today.AddDate(0, 0, -(60 + rng.Intn(200))))
	targetGoLive = pgDate(today.AddDate(0, 0, -(30 + rng.Intn(150))))
	switch status {
	case "Completed":
		progress = 100
		actualGoLive = pgDate(today.AddDate(0, 0, -(20 + rng.Intn(140))))
	case "In Progress":
		progress = int16(intn32(rng, 30, 80))
	case "Stalled":
		progress = int16(intn32(rng, 10, 40))
	default:
		progress = 0
	}
	return
}

func loginFrequencyFor(status string, rng *rand.Rand) string {
	switch status {
	case "Healthy":
		return pick(rng, []string{"Daily", "Weekly"})
	case "Critical":
		return pick(rng, []string{"Rarely", "Inactive"})
	case "At-Risk":
		return pick(rng, []string{"Weekly", "Monthly", "Rarely"})
	default:
		return pick(rng, []string{"Daily", "Weekly", "Monthly"})
	}
}

func usageTrendFor(status string, rng *rand.Rand) *string {
	switch status {
	case "Healthy":
		return ptr(pick(rng, []string{"Increasing", "Stable"}))
	case "Critical":
		return ptr("Decreasing")
	case "At-Risk":
		return ptr(pick(rng, []string{"Decreasing", "Stable"}))
	default:
		return ptr("Stable")
	}
}
