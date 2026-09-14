package handler

import (
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// health_score_row.go — adapter baris Health Score (BL-157i). healthRowToView
// memetakan field YANG DIPAKAI view, diekstrak dari DELAPAN struct sqlc
// berbeda (db.ListHealthScoresRow & tujuh db.ListHealthScoresSortByXRow — satu
// query = satu struct meski SELECT sama persis) agar logika mapping TAK
// diduplikasi per query — mirror renewalListRow (subscriptions_renewals_row.go).

// healthListRow = field yang dipakai healthRowToView, diekstrak dari sqlc row
// struct manapun (default atau salah satu dari tujuh varian sort).
type healthListRow struct {
	ID                 int64
	AccountName        string
	OverallHealthScore *int16
	HealthStatus       *string
	AdoptionScore      *int16
	EngagementScore    *int16
	SupportScore       *int16
	ScoreTrend         *string
	RenewalEndDate     pgtype.Date
}

func healthListRowFromDefault(s db.ListHealthScoresRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}

func healthListRowFromVillageSort(s db.ListHealthScoresSortByVillageRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}

func healthListRowFromScoreSort(s db.ListHealthScoresSortByScoreRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}

func healthListRowFromAdoptionSort(s db.ListHealthScoresSortByAdoptionRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}

func healthListRowFromEngagementSort(s db.ListHealthScoresSortByEngagementRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}

func healthListRowFromSupportSort(s db.ListHealthScoresSortBySupportRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}

func healthListRowFromTrendSort(s db.ListHealthScoresSortByTrendRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}

func healthListRowFromRenewalSort(s db.ListHealthScoresSortByRenewalRow) healthListRow {
	return healthListRow{
		ID: s.ID, AccountName: s.AccountName, OverallHealthScore: s.OverallHealthScore,
		HealthStatus: s.HealthStatus, AdoptionScore: s.AdoptionScore, EngagementScore: s.EngagementScore,
		SupportScore: s.SupportScore, ScoreTrend: s.ScoreTrend, RenewalEndDate: s.RenewalEndDate,
	}
}
