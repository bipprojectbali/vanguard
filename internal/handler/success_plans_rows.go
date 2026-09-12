package handler

import (
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// success_plans_rows.go — pemetaan baris, KPI, & badge Success Plan (view
// murni-data), dipisah dari success_plans_data.go (pemuatan & guard) untuk
// file health.

// successPlanRowView memetakan satu baris ListSuccessPlansRow → SuccessPlanRow view.
// BL-97: kolom mockup — "Tujuan Rencana" = objective (fallback plan_name bila
// kosong agar baris tetap teridentifikasi); Health dari skor kesehatan akun
// (reuse healthScoreStatus, BL-24). Nama Plan & Owner CS tak lagi dipetakan.
func successPlanRowView(r db.ListSuccessPlansRow, slug string) panel.SuccessPlanRow {
	objective := deref(r.Objective)
	if objective == "" {
		objective = r.PlanName
	}
	healthLabel, healthBadge := healthScoreStatus(r.HealthStatus)
	return panel.SuccessPlanRow{
		ID:          r.ID,
		AccountName: r.AccountName,
		Objective:   objective,
		StatusLabel: r.PlanStatus,
		StatusBadge: successPlanStatusBadge(r.PlanStatus),
		Progress:    int(r.Progress),
		TargetDate:  dateStr(r.TargetDate),
		HealthLabel: healthLabel,
		HealthBadge: healthBadge,
		HrefEdit:    wsPath(slug, "/success-plans/"+strconv.FormatInt(r.ID, 10)+"/edit"),
	}
}

// successPlanKPIParams merakit CountSuccessPlanKPIsParams dari filter ownership
// yang SAMA dengan ListSuccessPlans → KPI konsisten dengan daftar. today dioper
// dalam zona waktu app (batas overdue/jatuh-tempo dihitung di SQL).
func successPlanKPIParams(f db.SuccessPlansListFilter, uid int64) db.CountSuccessPlanKPIsParams {
	return db.CountSuccessPlanKPIsParams{
		Today:    pgtype.Date{Time: todayInAppTZ(), Valid: true},
		ScopeAll: f.ScopeAll,
		IsOwn:    f.IsOwn,
		Uid:      &uid,
	}
}

// successPlanKPIsToView memetakan agregat DB → kartu KPI header (angka + sub-teks).
// Kartu ke-4 "Rata Capaian" mensubstitusi "Target Tercapai" mockup (berbasis
// langkah — model belum ada); nilainya rata progress plan aktif (Active/At-Risk).
func successPlanKPIsToView(k db.CountSuccessPlanKPIsRow) panel.SuccessPlanKPIs {
	return panel.SuccessPlanKPIs{
		ActiveCount:    strconv.FormatInt(k.ActiveCount, 10),
		ActiveSub:      "di " + strconv.FormatInt(k.ActiveVillages, 10) + " desa",
		Overdue:        strconv.FormatInt(k.Overdue, 10),
		OverdueSub:     "melewati tenggat",
		DueSoon:        strconv.FormatInt(k.DueSoon, 10),
		DueSoonSub:     "perlu dikejar",
		AvgProgress:    strconv.Itoa(roundToInt(k.AvgProgress)) + "%",
		AvgProgressSub: "rata capaian",
	}
}

// successPlanTableSubtitle — subteks tabel BL-97 memakai jumlah plan aktif.
func successPlanTableSubtitle(activeCount int64) string {
	return strconv.FormatInt(activeCount, 10) + " rencana aktif · target terukur & penanggung jawab"
}

// successPlanStatusBadge memetakan status → kelas badge daisyUI.
func successPlanStatusBadge(s string) string {
	switch s {
	case "Draft":
		return "badge-ghost"
	case "Active":
		return "badge-info"
	case "Achieved":
		return "badge-success"
	case "At-Risk":
		return "badge-warning"
	case "Cancelled":
		return "badge-neutral"
	default:
		return "badge-ghost"
	}
}
