package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// canViewHealthScore: crm:health read (CSM, Sales, Manager, Admin, Support).
func canViewHealthScore(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:health", "read")
}

// renderHealthScoreForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderHealthScoreForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Customer Health Score", "/health-scores",
		panel.SalesForbidden("Customer Health Score"))
}

// healthScoreListParams merakit parameter ListHealthScores dari session.
// Pola: AccountsListFilterFor → IsOwn → KEDUA flag SQL true (union kepemilikan)
// — identik dengan accountsListParams / contactsListParams.
// Support (ScopeNone) → semua flag false → 0 baris (fail-closed).
func healthScoreListParams(ctx context.Context) db.ListHealthScoresParams {
	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	af := db.AccountsListFilterFor(dataScope)
	isOwn := af.IsOwn
	return db.ListHealthScoresParams{
		ScopeAll: af.ScopeAll,
		IsCsm:    isOwn,
		IsSales:  isOwn,
		Uid:      &uid,
	}
}

// healthScoreKPIParams merakit CountHealthScoreKPIsParams dari ListHealthScoresParams.
func healthScoreKPIParams(p db.ListHealthScoresParams) db.CountHealthScoreKPIsParams {
	return db.CountHealthScoreKPIsParams{
		ScopeAll: p.ScopeAll,
		IsCsm:    p.IsCsm,
		IsSales:  p.IsSales,
		Uid:      p.Uid,
	}
}

// healthScoreStatus mengembalikan label + kelas badge dari health_status.
func healthScoreStatus(status *string) (label, badge string) {
	if status == nil {
		return "—", "badge-ghost"
	}
	switch *status {
	case "Healthy":
		return "Sehat", "badge-success"
	case "At-Risk":
		return "Berisiko", "badge-warning"
	case "Critical":
		return "Kritis", "badge-error"
	default:
		return *status, "badge-ghost"
	}
}

// healthScoreTrend mengembalikan label tren singkat.
func healthScoreTrend(trend *string) string {
	if trend == nil {
		return "—"
	}
	switch *trend {
	case "Improving":
		return "↑ Naik"
	case "Stable":
		return "→ Stabil"
	case "Declining":
		return "↓ Turun"
	default:
		return *trend
	}
}

// healthScoreDaysInStage menghitung hari sejak stage_entry_date.
func healthScoreDaysInStage(d pgtype.Date, tz *time.Location) string {
	if !d.Valid {
		return "—"
	}
	loc := time.UTC
	if tz != nil {
		loc = tz
	}
	now := time.Now().In(loc)
	t := time.Date(int(d.Time.Year()), d.Time.Month(), int(d.Time.Day()), 0, 0, 0, 0, loc)
	days := int(now.Sub(t).Hours() / 24)
	if days < 0 {
		return "—"
	}
	if days == 0 {
		return "Hari ini"
	}
	return strconv.FormatInt(int64(days), 10) + " hari"
}

// healthScoreStr menampilkan skor sebagai string atau "—" bila nil.
func healthScoreStr(s *int16) string {
	if s == nil {
		return "—"
	}
	return strconv.FormatInt(int64(*s), 10)
}

// healthRowToView mengonversi ListHealthScoresRow ke panel.HealthScoreRowView.
// uid dioper untuk href akun — satu-satunya caller butuh uid dari session.
func healthRowToView(r db.ListHealthScoresRow, slug string, tz *time.Location, _ int64) panel.HealthScoreRowView {
	statusLabel, statusBadge := healthScoreStatus(r.HealthStatus)
	return panel.HealthScoreRowView{
		ID:          r.ID,
		AccountName: r.AccountName,
		Score:       healthScoreStr(r.OverallHealthScore),
		StatusLabel: statusLabel,
		StatusBadge: statusBadge,
		Adoption:    healthScoreStr(r.AdoptionScore),
		Engagement:  healthScoreStr(r.EngagementScore),
		Support:     healthScoreStr(r.SupportScore),
		Sentiment:   healthScoreStr(r.SentimentScore),
		Trend:       healthScoreTrend(r.ScoreTrend),
		DaysInStage: healthScoreDaysInStage(r.StageEntryDate, tz),
		HrefDetail:  wsPath(slug, "/accounts/"+strconv.FormatInt(r.ID, 10)+"/customer-success"),
	}
}
