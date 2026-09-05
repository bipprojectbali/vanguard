package handler

import (
	"context"

	"go_starter/internal/ui/pages/panel"
)

// reports_support_filter_view.go — perakit sub-view filter Support Report,
// dipisah dari reports_support_filter.go (ukuran file). Perilaku identik; hanya
// organisasi file yang berubah.

// buildSupportFilterView merakit sub-view filter (dropdown Periode + Prioritas).
// Kedua dropdown selalu dirender (enum tetap; prioritas tak bergantung scope).
// Menerima ctx demi keselarasan tanda-tangan (tak butuh DB kini).
func (h *Handler) buildSupportFilterView(_ context.Context, f supportReportFilter) panel.SupportReportFilterView {
	return panel.SupportReportFilterView{
		PeriodValue:   f.Period,
		Periods:       salesPeriodOptions(f.Period),
		PriorityValue: f.Priority,
		Priorities:    supportPriorityOptions(f.Priority),
		CustomStart:   f.customStart,
		CustomEnd:     f.customEnd,
		QueryString:   f.queryString(),
	}
}

// supportPriorityOptions membangun opsi dropdown Prioritas dgn selected sesuai
// pilihan aktif. 3 tingkat NYATA skema (label ID sepadan ticketPriorityLabelID).
func supportPriorityOptions(selected string) []panel.SalesFilterOption {
	defs := []struct{ val, label string }{
		{supportPriorityAll, "Semua Prioritas"},
		{supportPriorityHigh, "Tinggi"},
		{supportPriorityMedium, "Sedang"},
		{supportPriorityLow, "Rendah"},
	}
	opts := make([]panel.SalesFilterOption, 0, len(defs))
	for _, d := range defs {
		opts = append(opts, panel.SalesFilterOption{Value: d.val, Label: d.label, Selected: d.val == selected})
	}
	return opts
}
