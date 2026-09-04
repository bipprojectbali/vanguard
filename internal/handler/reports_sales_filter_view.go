package handler

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// reports_sales_filter_view.go — perakitan sub-view filter Sales Report (BL-49)
// & serialisasi query, dipisah dari reports_sales_filter.go (ukuran file). Model
// filter + parsing tetap di file induk; di sini hanya queryString (tautan Export)
// dan dropdown Periode/Tim. Perilaku identik; hanya organisasi file yang berubah.

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
