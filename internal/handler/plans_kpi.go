package handler

import (
	"context"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// plans_kpi.go — komputasi 4 KPI header katalog Plans & Pricing (BL-93): Paket
// Aktif, Paket Nonaktif, Harga Terendah, Harga Tertinggi. Dipisah dari
// plans_page.go agar aturan harga (annualisasi) punya rumah & test sendiri.
//
// Harga min/max DINORMALISASI ke tahunan (keputusan BL-93): base_price siklus
// Monthly dikali 12 agar apple-to-apple dengan Annual sebelum dibandingkan —
// angka mentah lintas-siklus menyesatkan (Rp 150rb/bln tampak < Rp 750rb/thn
// padahal setara Rp 1,8jt/thn). Dihitung atas plan AKTIF saja (ListPlans; plan
// nonaktif tak dijual → tak masuk rentang harga jual). Nilai KPI = angka tahunan
// setara; label = "NamaPlan / thn". Plan tanpa base_price / siklus tak dikenal
// dilewati (tak bisa dinormalisasi dengan jujur).

// billingMonthly/billingAnnual = cermin billingFrequencyOptions (plans_form.go).
// Named-const, bukan literal telanjang, agar annualisasi terikat enum yang sama.
const (
	billingMonthly = "Monthly"
	billingAnnual  = "Annual"
)

// annualMultiplier mengembalikan pengali annualisasi untuk satu siklus tagih dan
// apakah siklus dikenal. NULL/tak dikenal → (0,false) → plan dilewati dari min/max.
func annualMultiplier(billing string) (int64, bool) {
	switch billing {
	case billingMonthly:
		return 12, true
	case billingAnnual:
		return 1, true
	default:
		return 0, false
	}
}

// rupiahInt mengekstrak nilai bulat rupiah dari NUMERIC(15,2) (buang pecahan).
// NULL/tak terurai → (0,false). Dipakai annualisasi min/max; format desimal tak
// relevan untuk KPI ringkas.
func rupiahInt(n pgtype.Numeric) (int64, bool) {
	s := numericStr(n)
	if s == "" {
		return 0, false
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// planCatalogKPIView merakit 4 KPI dari agregat count + daftar plan aktif. Min/max
// harga dinormalisasi ke tahunan (lihat catatan berkas). Katalog tanpa harga
// dikenali → HasPrice=false (view menampilkan "—").
func planCatalogKPIView(stats db.PlanCatalogStatsRow, active []db.Plan) panel.PlanKPIView {
	v := panel.PlanKPIView{
		ActiveCount:   stats.ActiveCount,
		InactiveCount: stats.InactiveCount,
	}
	var lo, hi int64
	var loName, hiName string
	found := false
	for _, p := range active {
		mult, ok := annualMultiplier(deref(p.BillingFrequency))
		if !ok {
			continue
		}
		base, ok := rupiahInt(p.BasePrice)
		if !ok {
			continue
		}
		annual := base * mult
		if !found || annual < lo {
			lo, loName = annual, p.PlanName
		}
		if !found || annual > hi {
			hi, hiName = annual, p.PlanName
		}
		found = true
	}
	if found {
		v.HasPrice = true
		v.LowestPrice = "Rp " + groupThousands(strconv.FormatInt(lo, 10))
		v.LowestSub = loName + " / thn"
		v.HighestPrice = "Rp " + groupThousands(strconv.FormatInt(hi, 10))
		v.HighestSub = hiName + " / thn"
	}
	return v
}

// planKPI mengambil agregat count + daftar plan aktif, lalu merakit KPI header.
// Fail-soft: galat query → KPI nol/kosong (katalog tetap terkelola; KPI bukan
// data kritis untuk mengelola plan). Scope tenant lewat RLS (h.q).
func (h *Handler) planKPI(ctx context.Context) panel.PlanKPIView {
	stats, err := h.q(ctx).PlanCatalogStats(ctx)
	if err != nil {
		h.Log.Error("plans: kpi stats", "err", err)
		return panel.PlanKPIView{}
	}
	active, err := h.q(ctx).ListPlans(ctx)
	if err != nil {
		h.Log.Error("plans: kpi active plans", "err", err)
		return planCatalogKPIView(stats, nil)
	}
	return planCatalogKPIView(stats, active)
}
