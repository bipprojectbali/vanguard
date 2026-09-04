package handler

import (
	"context"

	"go_starter/internal/ui/pages/panel"
)

// reports_cs_filter_view.go — perakit sub-view filter CS Report, dipisah dari
// reports_cs_filter.go (ukuran file). Perilaku identik; hanya organisasi file
// yang berubah.

// buildCSFilterView merakit sub-view filter (dropdown Periode + Segmen). Kedua
// dropdown selalu dirender (enum tetap; segmen tak bergantung scope). Tak butuh
// ctx/DB (beda dgn Sales yang query daftar owner) — tapi menerima ctx demi
// keselarasan tanda-tangan bila kelak butuh.
func (h *Handler) buildCSFilterView(_ context.Context, f csReportFilter) panel.CSReportFilterView {
	return panel.CSReportFilterView{
		PeriodValue:  f.Period,
		Periods:      salesPeriodOptions(f.Period),
		SegmentValue: f.Segment,
		Segments:     csSegmentOptions(f.Segment),
		CustomStart:  f.customStart,
		CustomEnd:    f.customEnd,
		QueryString:  f.queryString(),
	}
}

// csSegmentOptions membangun opsi dropdown Segmen (band kesehatan) dgn selected
// sesuai pilihan aktif.
func csSegmentOptions(selected string) []panel.SalesFilterOption {
	defs := []struct{ val, label string }{
		{csSegmentAll, "Semua Segmen"},
		{csSegmentHealthy, "Sehat (80–100)"},
		{csSegmentFair, "Cukup (60–79)"},
		{csSegmentAtRisk, "Berisiko (40–59)"},
		{csSegmentCritical, "Kritis (<40)"},
	}
	opts := make([]panel.SalesFilterOption, 0, len(defs))
	for _, d := range defs {
		opts = append(opts, panel.SalesFilterOption{Value: d.val, Label: d.label, Selected: d.val == selected})
	}
	return opts
}
