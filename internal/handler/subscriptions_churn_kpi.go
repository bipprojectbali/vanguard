package handler

import (
	"context"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_churn_kpi.go — KPI dasbor, filter tipe, & pemetaan baris untuk
// dasbor Churn (Menu 5.2/5.4) di subscriptions_churn_page.go.

// churnTypeTab = satu tab tipe churn (key untuk query + label tampilan). Sumber SATU
// untuk tab view & normalisasi handler; key ” = semua tipe (mencerminkan
// subs_churn_type_chk: Voluntary/Involuntary).
type churnTypeTab struct{ Key, Label string }

var churnTypeTabs = []churnTypeTab{
	{"", "Semua"},
	{"Voluntary", "Sukarela"},
	{"Involuntary", "Terpaksa"},
}

// churnKPIs merakit 4 kartu KPI dasbor Churn. Jendela waktu TETAP per label mockup
// (bukan tab periode, keputusan user 7 Sep): Churn Rate & Desa Churn = 30 hari
// terakhir, Churned MRR = bulan berjalan, Avg Tenure = seluruh riwayat (period NULL).
// SELALU global (semua tipe churn) — tab Tipe hanya menyaring tabel + CSV. F3
// ownership diwariskan dari filter/uid. REUSE ReportRetention/ReportChurnAge apa adanya.
func (h *Handler) churnKPIs(ctx context.Context, filter db.SubscriptionsListFilter, uid int64, canARR bool) (panel.ChurnKPIs, error) {
	now := time.Now().In(appTZ)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, appTZ)
	start30 := salesTS(today.AddDate(0, 0, -30))
	endTom := salesTS(today.AddDate(0, 0, 1)) // eksklusif; sertakan churn hari ini
	monStart, monEnd := salesPresetRange(salesPeriodMonth, now)

	// Churn Rate & Desa Churn (30 hari): active = snapshot (tak dibatasi waktu),
	// churned = cancellation_date dalam [start30, endTom).
	ret, err := h.q(ctx).ReportRetention(ctx, db.ReportRetentionParams{
		PeriodStart: start30, PeriodEnd: endTom,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ChurnKPIs{}, err
	}
	// Churned MRR (bulan berjalan): SUM lost_value_mrr atas kohort churn bulan ini.
	mon, err := h.q(ctx).ReportChurnAge(ctx, db.ReportChurnAgeParams{
		PeriodStart: salesTS(monStart), PeriodEnd: salesTS(monEnd),
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ChurnKPIs{}, err
	}
	// Avg Tenure (seluruh riwayat): period NULL (pgtype zero → sqlc narg NULL).
	life, err := h.q(ctx).ReportChurnAge(ctx, db.ReportChurnAgeParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ChurnKPIs{}, err
	}

	return panel.ChurnKPIs{
		ChurnRate:       ratePct(ret.Churned, ret.Active+ret.Churned),
		ChurnedMRR:      maskARR(formatRupiah(mon.LostValue), canARR),
		VillagesChurned: strconv.FormatInt(ret.Churned, 10),
		VillagesActive:  strconv.FormatInt(ret.Active, 10),
		AvgTenure:       tenureMonthsStr(life.AvgAgeDays, life.AgedCount),
	}, nil
}

// tenureMonthsStr memformat rata umur (hari, dari ReportChurnAge) → "N bln".
// count ≤ 0 (tak ada kohort ber-tanggal lengkap) → "—". 30.44 = rata hari/bulan.
func tenureMonthsStr(days pgtype.Numeric, count int64) string {
	if count <= 0 {
		return "—"
	}
	f, err := days.Float64Value()
	if err != nil || !f.Valid {
		return "—"
	}
	return strconv.FormatInt(int64(f.Float64/30.44+0.5), 10) + " bln"
}

// subTenureMonthsStr = masa langganan satu baris (cancellation_date − start_date)
// dalam bulan, "N bln". Salah satu tanggal kosong / negatif → "—". Dihitung di
// handler agar view tetap murni-data (BL-92 kolom Tenure).
func subTenureMonthsStr(start, end pgtype.Date) string {
	if !start.Valid || !end.Valid {
		return "—"
	}
	days := end.Time.Sub(start.Time).Hours() / 24
	if days < 0 {
		return "—"
	}
	return strconv.FormatInt(int64(days/30.44+0.5), 10) + " bln"
}

// normalizeChurnTypeFilter memetakan ?type= ke salah satu key sah; nilai asing →
// ” (semua tipe). Key ” sendiri sah (tab "Semua").
func normalizeChurnTypeFilter(v string) string {
	for _, t := range churnTypeTabs {
		if t.Key == v {
			return v
		}
	}
	return ""
}

// churnTypeFilterOptions menyalin daftar tab ke tipe view (handler pemilik enum).
func churnTypeFilterOptions() []panel.ChurnType {
	out := make([]panel.ChurnType, 0, len(churnTypeTabs))
	for _, t := range churnTypeTabs {
		out = append(out, panel.ChurnType{Key: t.Key, Label: t.Label})
	}
	return out
}

// churnRowView memetakan satu baris → baris tabel Churn. MRR Hilang = lost_value_
// mrr, nilai komersial → maskARR (F4). CSM = nama pemilik langganan dari peta
// anggota. Tenure = cancellation_date − start_date (BL-92). Kolom mengikuti
// wireframe 5.2/5.4.
func churnRowView(s db.ListChurnedRow, names map[int64]string, canARR bool) panel.ChurnRow {
	return panel.ChurnRow{
		ID:        s.ID,
		Village:   s.VillageName,
		Plan:      subPlanDisplay(s.PlanName, s.ItemCount),
		LostMRR:   maskARR(formatRupiah(s.LostValueMrr), canARR),
		Reason:    deref(s.ChurnReason),
		Type:      deref(s.ChurnType),
		ChurnDate: dateStr(s.CancellationDate),
		Tenure:    subTenureMonthsStr(s.StartDate, s.CancellationDate),
		CSM:       ownerName(s.SubscriptionOwner, names),
	}
}
