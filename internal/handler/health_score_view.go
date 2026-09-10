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

// healthRowToView mengonversi ListHealthScoresRow ke panel.HealthScoreRowView.
// uid dioper untuk href akun — satu-satunya caller butuh uid dari session.
func healthRowToView(r db.ListHealthScoresRow, slug string, tz *time.Location, _ int64) panel.HealthScoreRowView {
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

// healthKPIsToView memetakan hasil agregat DB → KPI header (angka + sub-teks)
// dan dua panel dasbor BL-96 (Sebaran + Komposisi/Arah). Semua persen & lebar
// bar dihitung DI SINI (view murni-data). Panel dikosongkan (placeholder) bila
// belum ada desa terskor (Scored=0) agar bar 0% dari COALESCE tak menyesatkan.
func healthKPIsToView(k db.CountHealthScoreKPIsRow) (panel.HealthScoreKPIs, panel.HealthDashPanels) {
	kpi := panel.HealthScoreKPIs{
		Total:      k.Total,
		Healthy:    k.Healthy,
		AtRisk:     k.AtRisk,
		Critical:   k.Critical,
		HealthySub: strconv.Itoa(pctOf(k.Healthy, k.Total)) + "% dari binaan",
	}
	if k.Scored == 0 {
		kpi.AvgScoreSub = "belum ada skor"
	} else {
		kpi.AvgScoreSub = "rata skor " + strconv.Itoa(roundToInt(k.AvgScore))
	}

	panels := panel.HealthDashPanels{Scored: k.Scored > 0}
	if panels.Scored {
		panels.Distribution = []panel.HealthBarView{
			healthDistBar("Sehat", k.Healthy, k.Total, "bg-success"),
			healthDistBar("Berisiko", k.AtRisk, k.Total, "bg-warning"),
			healthDistBar("Kritis", k.Critical, k.Total, "bg-error"),
		}
		panels.Composition = []panel.HealthBarView{
			healthCompBar("Adopsi", k.AvgAdoption),
			healthCompBar("Engagement", k.AvgEngagement),
			healthCompBar("Support", k.AvgSupport),
			healthCompBar("Sentimen", k.AvgSentiment),
		}
		// Arah Pergerakan: porsi relatif terhadap desa yang PUNYA tren (bukan
		// total) — desa tanpa tren (snapshot pertama, BL-25) tak ikut basis.
		trendBase := k.TrendImproving + k.TrendStable + k.TrendDeclining
		panels.Movement = []panel.HealthBarView{
			healthDistBar("Membaik", k.TrendImproving, trendBase, "bg-success"),
			healthDistBar("Stabil", k.TrendStable, trendBase, "bg-base-300"),
			healthDistBar("Menurun", k.TrendDeclining, trendBase, "bg-error"),
		}
	}
	return kpi, panels
}

// healthDistBar — bar distribusi/tren: lebar = porsi count/total, nilai "N · P%".
func healthDistBar(label string, count, total int64, color string) panel.HealthBarView {
	p := pctOf(count, total)
	return panel.HealthBarView{
		Label: label,
		Value: strconv.FormatInt(count, 10) + " · " + strconv.Itoa(p) + "%",
		Pct:   p,
		Color: color,
	}
}

// healthCompBar — bar komposisi: lebar & nilai = rata komponen (skala 0–100),
// dijepit 0..100 agar lebar bar tak melampaui trek.
func healthCompBar(label string, avg float64) panel.HealthBarView {
	v := roundToInt(avg)
	if v > 100 {
		v = 100
	}
	return panel.HealthBarView{
		Label: label,
		Value: strconv.Itoa(v),
		Pct:   v,
		Color: "bg-primary",
	}
}

// pctOf = persen bulat count/total; total ≤ 0 → 0 (hindari bagi nol).
func pctOf(count, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(float64(count)/float64(total)*100 + 0.5)
}

// roundToInt membulatkan float non-negatif ke int terdekat (avg skor ≥ 0).
func roundToInt(f float64) int {
	if f < 0 {
		return 0
	}
	return int(f + 0.5)
}

// healthTableSubtitle — subteks tabel BL-96. Urutan dipertahankan created_at
// DESC (keputusan B, keyset BL-7 tak diubah) → "dari terbaru", BUKAN "skor
// terendah" seperti mockup (yang butuh ganti kolom cursor).
func healthTableSubtitle(total int64) string {
	return "Diurutkan dari terbaru · " + strconv.FormatInt(total, 10) + " desa binaan"
}
