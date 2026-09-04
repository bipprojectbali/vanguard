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

// reports_sales_filter.go — filter interaktif Sales Report (BL-49): Periode +
// Tim(owner) yang memotong SEMUA panel + KPI + CSV serentak. Filter = query
// param (?period=&owner=&start=&end=) → form GET native (bukan Datastar; lolos
// CSP gotcha #16, bookmarkable). SATU sumber angka: parse di sini, oper ke
// reportsSalesData (HTML) & reportsSalesCSV (CSV) → CSV tersaring identik HTML.
//
// Semantik Periode SENGAJA per-panel (bukan created_at seragam): Forecast
// memotong expected_close_date (forward-looking); Pipeline/Win-Loss/Activity/
// Lead memotong created_at (kohort masuk). Pemetaan kolom ada di query SQL
// masing-masing — di sini hanya batas [start,end) yang dihitung.
//
// Owner filter di-AND DI ATAS scope F3 (menyempit, tak melebarkan): sales
// own-scope memilih owner lain → nol baris (F3 menang). Dropdown owner hanya
// dirender untuk pemakai scope_all (handler), diturunkan dari DATA
// (ReportSalesOwners) bukan daftar anggota.

// Preset periode. "" = Semua (tanpa saring). "custom" = rentang bebas via
// ?start=&end= (YYYY-MM-DD di appTZ).
const (
	salesPeriodAll     = ""
	salesPeriodMonth   = "month"
	salesPeriodQuarter = "quarter"
	salesPeriodYear    = "year"
	salesPeriodCustom  = "custom"
)

// salesReportFilter = filter aktif terparse. Start/End = batas [start,end) untuk
// query (Valid=false → tak menyaring). OwnerID nil → semua owner.
type salesReportFilter struct {
	Period      string
	Start       pgtype.Timestamptz
	End         pgtype.Timestamptz
	OwnerID     *int64
	customStart string // echo mentah YYYY-MM-DD untuk input date (walau invalid)
	customEnd   string
}

// parseSalesReportFilter membaca ?period=/?owner=/?start=/?end= dari request.
// Preset tak dikenal → Semua (aman). Custom butuh start & end valid + start≤end;
// jika tidak, jatuh ke Semua (tapi nilai mentah tetap di-echo ke input date).
func parseSalesReportFilter(r *http.Request) salesReportFilter {
	qv := r.URL.Query()
	f := salesReportFilter{Period: normalizeSalesPeriod(qv.Get("period"))}

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
			// Rentang tak lengkap/terbalik: pertahankan pilihan "custom" +
			// echo nilai, tapi jangan menyaring (Start/End tetap invalid).
			f.Period = salesPeriodCustom
		}
	}

	if id, err := strconv.ParseInt(qv.Get("owner"), 10, 64); err == nil && id > 0 {
		f.OwnerID = &id
	}
	return f
}

// normalizeSalesPeriod memetakan input ke preset dikenal; selain itu → Semua.
func normalizeSalesPeriod(p string) string {
	switch p {
	case salesPeriodMonth, salesPeriodQuarter, salesPeriodYear, salesPeriodCustom:
		return p
	default:
		return salesPeriodAll
	}
}

// salesPresetRange menghitung [start,end) untuk preset relatif `now` (di appTZ).
// end eksklusif (awal periode berikutnya). Batas dibangun di appTZ agar "bulan
// ini" mengikuti kalender lokal operator; disimpan sebagai instant (timestamptz)
// yang dibandingkan Postgres apa adanya.
func salesPresetRange(period string, now time.Time) (start, end time.Time) {
	loc := now.Location()
	switch period {
	case salesPeriodMonth:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 1, 0)
	case salesPeriodQuarter:
		q := (int(now.Month()) - 1) / 3 // 0..3
		startMonth := time.Month(q*3 + 1)
		start = time.Date(now.Year(), startMonth, 1, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 3, 0)
	case salesPeriodYear:
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
		end = start.AddDate(1, 0, 0)
	}
	return start, end
}

// salesCustomRange parse dua tanggal YYYY-MM-DD (di appTZ) → [start, end) dengan
// end = hari `endStr` + 1 hari (inklusif tanggal akhir). ok=false bila salah
// satu kosong/invalid atau start > end.
func salesCustomRange(startStr, endStr string) (start, end time.Time, ok bool) {
	if startStr == "" || endStr == "" {
		return time.Time{}, time.Time{}, false
	}
	s, err1 := time.ParseInLocation("2006-01-02", startStr, appTZ)
	e, err2 := time.ParseInLocation("2006-01-02", endStr, appTZ)
	if err1 != nil || err2 != nil || s.After(e) {
		return time.Time{}, time.Time{}, false
	}
	return s, e.AddDate(0, 0, 1), true
}

// salesTS membungkus instant jadi pgtype.Timestamptz valid (narg period_*).
func salesTS(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// queryString merakit "period=…&owner=…&start=…&end=…" (ter-encode, tanpa
// leading ?/&) untuk ditempel ke tautan Export CSV agar CSV tersaring identik.
// Kosong bila tak ada filter aktif.
func (f salesReportFilter) queryString() string {
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
	if f.OwnerID != nil {
		vals.Set("owner", strconv.FormatInt(*f.OwnerID, 10))
	}
	return vals.Encode()
}

// buildSalesFilterView merakit sub-view filter (dropdown Periode + Tim). Owner
// dropdown HANYA untuk pemakai scope_all (deal.ScopeAll) — sales own-scope cuma
// melihat dirinya, dropdown menyesatkan. Owner options diturunkan dari DATA
// (ReportSalesOwners) dalam cakupan pemakai; TAK disaring Periode agar owner
// terpilih selalu tampil.
func (h *Handler) buildSalesFilterView(
	ctx context.Context, f salesReportFilter, deal db.DealsListFilter, uid int64,
) (panel.SalesReportFilterView, error) {
	fv := panel.SalesReportFilterView{
		PeriodValue: f.Period,
		Periods:     salesPeriodOptions(f.Period),
		ShowOwner:   deal.ScopeAll,
		CustomStart: f.customStart,
		CustomEnd:   f.customEnd,
		QueryString: f.queryString(),
	}
	if f.OwnerID != nil {
		fv.OwnerValue = strconv.FormatInt(*f.OwnerID, 10)
	}
	if !deal.ScopeAll {
		return fv, nil // dropdown owner tak dirender → jangan query
	}

	owners, err := h.q(ctx).ReportSalesOwners(ctx, db.ReportSalesOwnersParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.SalesReportFilterView{}, err
	}
	opts := make([]panel.SalesFilterOption, 0, len(owners)+1)
	opts = append(opts, panel.SalesFilterOption{Value: "", Label: "Semua Tim", Selected: f.OwnerID == nil})
	for _, o := range owners {
		id := strconv.FormatInt(o.OwnerID, 10)
		opts = append(opts, panel.SalesFilterOption{
			Value:    id,
			Label:    ownerLabel(o.OwnerName, o.OwnerEmail),
			Selected: fv.OwnerValue == id,
		})
	}
	fv.Owners = opts
	return fv, nil
}

// salesPeriodOptions membangun opsi dropdown Periode dengan selected sesuai
// pilihan aktif.
func salesPeriodOptions(selected string) []panel.SalesFilterOption {
	defs := []struct{ val, label string }{
		{salesPeriodAll, "Semua Waktu"},
		{salesPeriodMonth, "Bulan Ini"},
		{salesPeriodQuarter, "Kuartal Ini"},
		{salesPeriodYear, "Tahun Ini"},
		{salesPeriodCustom, "Kustom"},
	}
	opts := make([]panel.SalesFilterOption, 0, len(defs))
	for _, d := range defs {
		opts = append(opts, panel.SalesFilterOption{Value: d.val, Label: d.label, Selected: d.val == selected})
	}
	return opts
}

// ownerLabel = nama tampilan owner (nama > email bila nama kosong).
func ownerLabel(name *string, email string) string {
	if name != nil {
		if n := strings.TrimSpace(*name); n != "" {
			return n
		}
	}
	return email
}
