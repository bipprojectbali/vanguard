package handler

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// reports_subscriptions_filter.go — filter interaktif Subscription Report
// (BL-52): Periode + Paket yang memotong 4 KPI + ke-5 panel + CSV serentak.
// Sama pola BL-49 (Sales) / BL-50 (CS) / BL-51 (Support): query param
// (?period=&plan=&start=&end=) → form GET native (bukan Datastar; lolos CSP
// gotcha #16, bookmarkable). SATU sumber angka: parse di sini, oper ke
// reportsSubscriptionsData (HTML) & lewat view yang sama ke CSV → CSV tersaring
// identik HTML. Preset periode & rentang di-share dgn reports_sales_filter.go
// (normalizeSalesPeriod/salesPresetRange/salesCustomRange/salesTS, package sama)
// — vokabuler waktu identik, tak diduplikasi.
//
// Semantik Periode SENGAJA per-panel (kolom waktu berbeda, bukan seragam;
// pemetaan di queries/reports_subscriptions.sql & reports.sql): pergerakan MRR
// baru/ekspansi/kontraksi by start_date, churn by cancellation_date; Renewal
// (kartu + tabel bulanan) by end_date; Churn (breakdown + KPI) by
// cancellation_date. Panel SNAPSHOT (MRR/ARR berjalan, Revenue-by-Plan, Aging) =
// nilai SEKARANG tanpa dimensi waktu → Periode TAK diterapkan (catatan UI
// menjelaskan). Bila Periode kosong, pergerakan MRR jatuh ke jendela "bulan ini"
// (perilaku BL-47).
//
// Paket = plan_id DITURUNKAN DARI DATA (langganan dalam cakupan pemakai), bukan
// daftar plans penuh. Di-AND DI ATAS scope F3 (menyempit, tak melebarkan): plan
// asing → nol baris (F3 menang). Dropdown selalu dirender (plan tak owner-
// spesifik; own-scope pun bisa punya beberapa paket).

// subscriptionReportFilter = filter aktif terparse. Start/End = batas [start,end)
// (Valid=false → tak menyaring). PlanID nil → semua paket.
type subscriptionReportFilter struct {
	Period      string
	Start       pgtype.Timestamptz
	End         pgtype.Timestamptz
	PlanID      *int64
	customStart string // echo mentah YYYY-MM-DD untuk input date (walau invalid)
	customEnd   string
}

// parseSubscriptionReportFilter membaca ?period=/?plan=/?start=/?end= dari
// request. Preset tak dikenal → Semua (aman). Custom butuh start & end valid +
// start≤end; jika tidak, jatuh ke Semua (nilai mentah tetap di-echo ke input).
func parseSubscriptionReportFilter(r *http.Request) subscriptionReportFilter {
	qv := r.URL.Query()
	f := subscriptionReportFilter{Period: normalizeSalesPeriod(qv.Get("period"))}

	now := time.Now().In(appTZ)
	switch f.Period {
	case salesPeriodMonth, salesPeriodQuarter, salesPeriodYear:
		start, end := salesPresetRange(f.Period, now)
		f.Start = salesTS(start)
		f.End = salesTS(end)
	case salesPeriodCustom:
		f.customStart = strings.TrimSpace(qv.Get("start"))
		f.customEnd = strings.TrimSpace(qv.Get("end"))
		if start, end, ok := salesCustomRange(f.customStart, f.customEnd); ok {
			f.Start = salesTS(start)
			f.End = salesTS(end)
		} else {
			f.Period = salesPeriodCustom
		}
	}

	if id, err := strconv.ParseInt(qv.Get("plan"), 10, 64); err == nil && id > 0 {
		f.PlanID = &id
	}
	return f
}

// queryString merakit "period=…&plan=…&start=…&end=…" (ter-encode, tanpa leading
// ?/&) untuk ditempel ke tautan Export CSV agar CSV tersaring identik. Kosong
// bila tak ada filter aktif.
func (f subscriptionReportFilter) queryString() string {
	vals := url.Values{}
	if f.Period != salesPeriodAll {
		vals.Set("period", f.Period)
	}
	if f.Period == salesPeriodCustom {
		if f.customStart != "" {
			vals.Set("start", f.customStart)
		}
		if f.customEnd != "" {
			vals.Set("end", f.customEnd)
		}
	}
	if f.PlanID != nil {
		vals.Set("plan", strconv.FormatInt(*f.PlanID, 10))
	}
	return vals.Encode()
}

// buildSubscriptionFilterView merakit sub-view filter (dropdown Periode + Paket).
// Paket options diturunkan dari DATA (ReportSubscriptionPlans) dalam cakupan
// pemakai; selalu dirender (paket tak owner-spesifik). Menerima uid & filter
// scope F3 yang SAMA dgn data (SubscriptionsListFilterFor).
func (h *Handler) buildSubscriptionFilterView(
	ctx context.Context, f subscriptionReportFilter, sf db.SubscriptionsListFilter, uid int64,
) (panel.SubscriptionReportFilterView, error) {
	fv := panel.SubscriptionReportFilterView{
		PeriodValue: f.Period,
		Periods:     salesPeriodOptions(f.Period),
		CustomStart: f.customStart,
		CustomEnd:   f.customEnd,
		QueryString: f.queryString(),
	}
	if f.PlanID != nil {
		fv.PlanValue = strconv.FormatInt(*f.PlanID, 10)
	}

	plans, err := h.q(ctx).ReportSubscriptionPlans(ctx, db.ReportSubscriptionPlansParams{
		ScopeAll: sf.ScopeAll, IsOwn: sf.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.SubscriptionReportFilterView{}, err
	}
	opts := make([]panel.SalesFilterOption, 0, len(plans)+1)
	opts = append(opts, panel.SalesFilterOption{Value: "", Label: "Semua Paket", Selected: f.PlanID == nil})
	for _, p := range plans {
		id := strconv.FormatInt(p.PlanID, 10)
		opts = append(opts, panel.SalesFilterOption{
			Value:    id,
			Label:    p.PlanName,
			Selected: fv.PlanValue == id,
		})
	}
	fv.Plans = opts
	return fv, nil
}
