package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_support_filter.go — filter interaktif Support Report (BL-51): Periode +
// Prioritas yang memotong 3 KPI + ke-5 panel + CSV serentak. Sama pola BL-49
// (Sales) & BL-50 (CS): query param (?period=&priority=&start=&end=) → form GET
// native (bukan Datastar; lolos CSP gotcha #16, bookmarkable). SATU sumber angka:
// parse di sini, oper ke reportsSupportData (HTML) & lewat view yang sama ke CSV
// → CSV tersaring identik HTML. Preset periode & rentang di-share dgn reports_
// sales_filter.go (normalizeSalesPeriod/salesPresetRange/salesCustomRange/salesTS,
// package sama) — vokabuler waktu identik, tak diduplikasi.
//
// Semantik Periode SENGAJA per-panel (kolom waktu berbeda, bukan created_at
// seragam; pemetaan di queries/reports_support.sql): Volume Masuk by created_at &
// Selesai by resolved_at; SLA & Agen by created_at; Resolution by resolved_at
// (tiket selesai dalam jendela). DUA pengecualian: baris "bulan ini vs lalu"
// (ReportResolutionMonthCompare) inheren relatif now() → Periode TAK berlaku
// (hanya Prioritas); KB Published = snapshot tanpa dimensi waktu → tak disaring
// sama sekali (catatan UI menjelaskan keduanya).
//
// Prioritas = 3 tingkat NYATA skema (rendah/sedang/tinggi; TAK ada "Kritis" —
// mockup keliru). Di-AND DI ATAS scope F3 (menyempit, tak melebarkan). Panel
// per-prioritas (SLA, Resolution) → baris tersaring; panel agregat (Volume, KPI,
// Agen) → agregasi ulang hanya prioritas terpilih.

// Prioritas tiket. "" = Semua. Selain nilai dikenal → Semua (aman).
const (
	supportPriorityAll    = ""
	supportPriorityLow    = "rendah"
	supportPriorityMedium = "sedang"
	supportPriorityHigh   = "tinggi"
)

// supportReportFilter = filter aktif terparse. Start/End = batas [start,end)
// (Valid=false → tak menyaring). Priority "" → semua prioritas.
type supportReportFilter struct {
	Period      string
	Start       pgtype.Timestamptz
	End         pgtype.Timestamptz
	Priority    string
	customStart string // echo mentah YYYY-MM-DD untuk input date (walau invalid)
	customEnd   string
}

// parseSupportReportFilter membaca ?period=/?priority=/?start=/?end= dari request.
// Preset & prioritas tak dikenal → Semua (aman). Custom butuh start & end valid +
// start≤end; jika tidak, jatuh ke Semua (nilai mentah tetap di-echo ke input).
func parseSupportReportFilter(r *http.Request) supportReportFilter {
	qv := r.URL.Query()
	f := supportReportFilter{
		Period:   normalizeSalesPeriod(qv.Get("period")),
		Priority: normalizeSupportPriority(qv.Get("priority")),
	}

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
	return f
}

// normalizeSupportPriority memetakan input ke prioritas dikenal; selain itu → Semua.
func normalizeSupportPriority(s string) string {
	switch s {
	case supportPriorityLow, supportPriorityMedium, supportPriorityHigh:
		return s
	default:
		return supportPriorityAll
	}
}

// priorityArg = *string narg untuk query (nil = semua prioritas).
func (f supportReportFilter) priorityArg() *string {
	if f.Priority == supportPriorityAll {
		return nil
	}
	p := f.Priority
	return &p
}

// queryString merakit "period=…&priority=…&start=…&end=…" (ter-encode, tanpa
// leading ?/&) untuk ditempel ke tautan Export CSV agar CSV tersaring identik.
// Kosong bila tak ada filter aktif.
func (f supportReportFilter) queryString() string {
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
	if f.Priority != supportPriorityAll {
		vals.Set("priority", f.Priority)
	}
	return vals.Encode()
}
