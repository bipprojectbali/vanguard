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
)

// KPI header & dasbor (healthKPIsToView, dll.) di health_score_kpi.go.

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
		Segment:  p.Segment,
	}
}

// healthActionLabel — aksi kontekstual tabel (BL-96) diturunkan dari status
// kesehatan: Kritis butuh intervensi (Playbook), Berisiko perlu Tinjau, sisanya
// (Healthy / belum dinilai) cukup Lihat. Semua menuju detail customer-success
// yang sama (tak ada halaman playbook per-akun) — label saja yang kontekstual.
func healthActionLabel(status *string) string {
	if status == nil {
		return "Lihat"
	}
	switch *status {
	case "Critical":
		return "Playbook"
	case "At-Risk":
		return "Tinjau"
	default:
		return "Lihat"
	}
}

// healthScoreStr menampilkan skor sebagai string atau "—" bila nil.
func healthScoreStr(s *int16) string {
	if s == nil {
		return "—"
	}
	return strconv.FormatInt(int64(*s), 10)
}

// healthRowToView mengonversi healthListRow (adapter BL-157i, lihat
// health_score_row.go) ke panel.HealthScoreRowView. uid dioper untuk href
// akun — satu-satunya caller butuh uid dari session.
func healthRowToView(r healthListRow, slug string, tz *time.Location, _ int64) panel.HealthScoreRowView {
	statusLabel, statusBadge := healthScoreStatus(r.HealthStatus)
	loc := time.UTC
	if tz != nil {
		loc = tz
	}
	return panel.HealthScoreRowView{
		ID:          r.ID,
		AccountName: r.AccountName,
		Score:       healthScoreStr(r.OverallHealthScore),
		StatusLabel: statusLabel,
		StatusBadge: statusBadge,
		Adoption:    healthScoreStr(r.AdoptionScore),
		Engagement:  healthScoreStr(r.EngagementScore),
		Support:     healthScoreStr(r.SupportScore),
		Trend:       healthScoreTrend(r.ScoreTrend),
		// BL-96: "Jatuh Tempo" = sisa hari ke end_date langganan aktif terdekat
		// (renewal_end_date dari LATERAL); reuse daysLeftLabel (subscriptions).
		RenewalDue:  daysLeftLabel(time.Now().In(loc), r.RenewalEndDate),
		ActionLabel: healthActionLabel(r.HealthStatus),
		HrefDetail:  wsPath(slug, "/accounts/"+strconv.FormatInt(r.ID, 10)+"/customer-success"),
	}
}
