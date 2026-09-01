package handler

import (
	"context"
	"encoding/json"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
	g "maragu.dev/gomponents"
)

// dashboard.go — Beranda ruang kerja (Modul 1, tasks.md M1-1/M1-2): kartu KPI
// (ARR total, pipeline per-stage, distribusi health, renewal jatuh tempo) +
// dua chart ECharts vendored (CSP-safe via <script type=application/json>,
// dirender charts.js). Query di queries/dashboard.sql; distribusi health
// REUSE CountHealthScoreKPIs (health_score.sql) via helper health_score_view.go
// — tak dituliskan ulang. F2 gate di dashboard_view.go; F3 ownership pakai
// filter modul ASAL tiap tabel (deals → DealsListFilter, subscriptions →
// SubscriptionsListFilter, health → AccountsListFilter lewat healthScoreListParams)
// — sumber SATU, bukan duplikat logic scope. F4: ARR pakai kebijakan
// subscriptions (canSeeSubscriptionARR/maskSubscriptionARR) BUKAN kebijakan
// deals — spec M1-3 "manager cross-team, ARR terbatas" cocok dgn Admin+Manager
// saja. Health TAK di-mask (kebijakan sengaja, lihat fls.go).

// dashboardHome merakit body Beranda: fail-soft (pola sama dgn notifBadge di
// shell_nav.go) — tanpa izin ATAU gagal query, jatuh ke Placeholder biasa.
// Dashboard hanya angka ringkas; halaman gagal render jauh lebih buruk
// daripada kartu KPI yang hilang sesekali.
func (h *Handler) dashboardHome(ctx context.Context) g.Node {
	greeting := "Selamat datang di " + session.TenantName(ctx) + "."
	if !canViewDashboard(ctx) {
		return panel.Placeholder("Beranda", greeting)
	}
	view, err := h.dashboardData(ctx)
	if err != nil {
		h.Log.Error("dashboard: build", "err", err)
		return panel.Placeholder("Beranda", greeting)
	}
	return panel.DashboardBody(view)
}

// dashboardData menjalankan keempat agregasi (empat round-trip kecil, rule 13
// — bukan hitung di Go atas seluruh baris) & merakit view-model siap-render.
func (h *Handler) dashboardData(ctx context.Context) (panel.DashboardView, error) {
	dataScope := session.BusinessDataScope(ctx)
	businessRole := session.BusinessRole(ctx)
	uid := session.UserID(ctx)
	q := h.q(ctx)

	subsFilter := db.SubscriptionsListFilterFor(dataScope)

	arrTotal, err := q.DashboardARRTotal(ctx, db.DashboardARRTotalParams{
		ScopeAll: subsFilter.ScopeAll, IsOwn: subsFilter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.DashboardView{}, err
	}

	// BL-11: chart Pipeline per-Stage bersumber Deals — gerbangi pada
	// canViewDeals (crm:deals read, bukan cek role khusus). Role tanpa izin
	// (CS pasca-BL-11, Support) melewati query ini (hemat 1 round-trip);
	// PipelineChart tetap "" → kartu tak dirender (dashboard.go view).
	pipelineChart := ""
	if canViewDeals(ctx) {
		dealsFilter := db.DealsListFilterFor(dataScope)
		stages, err := q.DashboardPipelineByStage(ctx, db.DashboardPipelineByStageParams{
			ScopeAll: dealsFilter.ScopeAll, IsOwn: dealsFilter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashboardView{}, err
		}
		pipelineChart = h.marshalChart(pipelineChartOption(stages))
	}

	now := time.Now().In(appTZ)
	today := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
	due, err := q.DashboardRenewalsDue(ctx, db.DashboardRenewalsDueParams{
		Today: today, ScopeAll: subsFilter.ScopeAll, IsOwn: subsFilter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.DashboardView{}, err
	}

	health, err := q.CountHealthScoreKPIs(ctx, healthScoreKPIParams(healthScoreListParams(ctx)))
	if err != nil {
		return panel.DashboardView{}, err
	}

	return panel.DashboardView{
		ARRTotal:       maskSubscriptionARR(formatRupiah(arrTotal), businessRole),
		PipelineChart:  pipelineChart,
		HealthChart:    h.marshalChart(healthChartOption(health)),
		HealthTotal:    health.Total,
		HealthScored:   health.Scored,
		RenewalsDue:    due.DueCount,
		RenewalsDueARR: maskSubscriptionARR(formatRupiah(due.DueArr), businessRole),
	}, nil
}

// marshalChart men-marshal option ECharts → JSON siap-tanam; gagal → di-log +
// "{}" agar charts.js tak crash (pola sama dgn buildChart di dev_logs_build.go).
func (h *Handler) marshalChart(opt map[string]any) string {
	b, err := json.Marshal(opt)
	if err != nil {
		h.Log.Error("dashboard: marshal chart", "err", err)
		return "{}"
	}
	return string(b)
}
