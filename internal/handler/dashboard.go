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

// dashboard.go — Beranda ruang kerja (Modul 1). BL-59 redesain: baris KPI ringkas
// GLOBAL (ARR total, renewal jatuh tempo, ARR berisiko, health dinilai) + distribusi
// health DIPERTAHANKAN di atas; di bawahnya SECTION per-domain (Sales/…) yang
// dikomposisi per-izin (dashboard_sales.go dst). Tiap domain merender bila role
// punya kapabilitas modulnya — selaras F2/F3 yang sudah ada, role kustom otomatis
// dapat section sesuai modulnya tanpa kode baru.
//
// Query global di queries/dashboard.sql; distribusi health REUSE
// CountHealthScoreKPIs (health_score.sql). F2 gate di dashboard_view.go; F3
// ownership pakai filter modul ASAL tiap tabel (subscriptions → SubscriptionsListFilter,
// health → healthScoreListParams). F4: ARR pakai kebijakan BL-58 (kapabilitas
// crm:subscriptions/arr, bukan nama role). Health TAK di-mask (kebijakan sengaja).

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

// dashboardData menjalankan agregasi GLOBAL (baris KPI ringkas + distribusi
// health) lalu mengomposisi section per-domain (BL-59). Tiap domain builder
// (dashSalesDomain, …) menggate butirnya per-kapabilitas & hanya ditambahkan
// bila punya isi — role melihat UNION section modul yang boleh diaksesnya.
func (h *Handler) dashboardData(ctx context.Context) (panel.DashboardView, error) {
	dataScope := session.BusinessDataScope(ctx)
	canARR := canSeeSubscriptionARR(ctx) // BL-58: kapabilitas, bukan nama role
	uid := session.UserID(ctx)
	q := h.q(ctx)

	subsFilter := db.SubscriptionsListFilterFor(dataScope)

	arrTotal, err := q.DashboardARRTotal(ctx, db.DashboardARRTotalParams{
		ScopeAll: subsFilter.ScopeAll, IsOwn: subsFilter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.DashboardView{}, err
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

	view := panel.DashboardView{
		ARRTotal:       maskSubscriptionARR(formatRupiah(arrTotal), canARR),
		HealthChart:    h.marshalChart(healthChartOption(health)),
		HealthTotal:    health.Total,
		HealthScored:   health.Scored,
		RenewalsDue:    due.DueCount,
		RenewalsDueARR: maskSubscriptionARR(formatRupiah(due.DueArr), canARR),
	}

	// Section per-domain (BL-59). 59a: Sales, 59b: Langganan, 59c: Customer
	// Success. 59d menyusul (Support) — tiap domain builder ditambah di sini
	// dengan pola sama.
	if d, ok, err := h.dashSalesDomain(ctx, dataScope, uid); err != nil {
		return panel.DashboardView{}, err
	} else if ok {
		view.Domains = append(view.Domains, d)
	}
	if d, ok, err := h.dashSubscriptionDomain(ctx, dataScope, uid); err != nil {
		return panel.DashboardView{}, err
	} else if ok {
		view.Domains = append(view.Domains, d)
	}
	if d, ok, err := h.dashCSDomain(ctx, dataScope, uid); err != nil {
		return panel.DashboardView{}, err
	} else if ok {
		view.Domains = append(view.Domains, d)
	}

	return view, nil
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
