package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_cs_filter.go — filter interaktif Customer Success Report (BL-50):
// Periode + Segmen yang memotong 3 KPI + ke-5 panel + CSV serentak. Sama pola
// BL-49 (Sales): query param (?period=&segment=&start=&end=) → form GET native
// (bukan Datastar; lolos CSP gotcha #16, bookmarkable). SATU sumber angka: parse
// di sini, oper ke reportsCSData (HTML) & lewat view yang sama ke CSV → CSV
// tersaring identik HTML. Preset periode & rentang di-share dgn reports_sales_
// filter.go (normalizeSalesPeriod/salesPresetRange/salesCustomRange/salesTS,
// package sama) — vokabuler waktu identik, tak diduplikasi.
//
// Semantik Periode SENGAJA per-panel (kolom waktu berbeda, bukan created_at
// seragam): Onboarding memotong kickoff_date (kohort mulai); Engagement memotong
// scheduled_at; Retention(churn)/Alasan Churn memotong cancellation_date.
// Health, Adoption, & Retention Rate = SNAPSHOT (kondisi terkini) → Periode TAK
// berlaku (catatan UI di view). Pemetaan kolom ada di query SQL; di sini hanya
// batas [start,end) & nilai Segmen.
//
// Segmen = band kesehatan (overall_health_score) — CRM tak punya "segmen"
// formal; band kesehatan paling relevan CS (spec BL-50: bila tak jelas, pilih
// health band). Di-AND DI ATAS scope F3 (menyempit, tak melebarkan). Enum tetap
// → dropdown selalu dirender (tak seperti owner Sales yang bergantung scope).

// Segmen band kesehatan. "" = Semua. Selain nilai dikenal → Semua (aman).
const (
	csSegmentAll      = ""
	csSegmentHealthy  = "healthy"
	csSegmentFair     = "fair"
	csSegmentAtRisk   = "at_risk"
	csSegmentCritical = "critical"
)

// csReportFilter = filter aktif terparse. Start/End = batas [start,end)
// (Valid=false → tak menyaring). Segment "" → semua band.
type csReportFilter struct {
	Period      string
	Start       pgtype.Timestamptz
	End         pgtype.Timestamptz
	Segment     string
	customStart string // echo mentah YYYY-MM-DD untuk input date (walau invalid)
	customEnd   string
}

// parseCSReportFilter membaca ?period=/?segment=/?start=/?end= dari request.
// Preset & segmen tak dikenal → Semua (aman). Custom butuh start & end valid +
// start≤end; jika tidak, jatuh ke Semua (nilai mentah tetap di-echo ke input).
func parseCSReportFilter(r *http.Request) csReportFilter {
	qv := r.URL.Query()
	f := csReportFilter{
		Period:  normalizeSalesPeriod(qv.Get("period")),
		Segment: normalizeCSSegment(qv.Get("segment")),
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

// normalizeCSSegment memetakan input ke band dikenal; selain itu → Semua.
func normalizeCSSegment(s string) string {
	switch s {
	case csSegmentHealthy, csSegmentFair, csSegmentAtRisk, csSegmentCritical:
		return s
	default:
		return csSegmentAll
	}
}

// segmentArg = *string narg untuk query (nil = semua band).
func (f csReportFilter) segmentArg() *string {
	if f.Segment == csSegmentAll {
		return nil
	}
	s := f.Segment
	return &s
}

// queryString merakit "period=…&segment=…&start=…&end=…" (ter-encode, tanpa
// leading ?/&) untuk ditempel ke tautan Export CSV agar CSV tersaring identik.
// Kosong bila tak ada filter aktif.
func (f csReportFilter) queryString() string {
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
	if f.Segment != csSegmentAll {
		vals.Set("segment", f.Segment)
	}
	return vals.Encode()
}
