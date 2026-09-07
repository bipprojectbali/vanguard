package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_kpi.go — 4 KPI header dasbor Renewals (BL-94). Satu query
// agregat (RenewalKPIs) di-scope ownership SAMA dgn ListRenewals; nilai diformat di
// sini (view murni-data). Renewal Rate memakai definisi Report 8.4
// (ratePct(renewed_past, due_past)) atas kohort 12 bln terakhir. Dipisah dari
// subscriptions_renewals.go demi ambang Route/Handler (150 baris).

// renewalKPI menjalankan query agregat + memformat 4 kartu KPI. Gagal → error
// dipropagasi; pemanggil memilih fail-soft (render tabel tanpa KPI, KPI bukan
// data kritis untuk membaca status renewal).
func (h *Handler) renewalKPI(ctx context.Context, filter db.SubscriptionsListFilter, uid int64, today pgtype.Date) (panel.RenewalKPIs, error) {
	k, err := h.q(ctx).RenewalKPIs(ctx, db.RenewalKPIsParams{
		Today:    today,
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
	})
	if err != nil {
		return panel.RenewalKPIs{}, err
	}
	return renewalKPIView(k), nil
}

// renewalKPIView memformat baris agregat → kartu KPI. Rate = diperpanjang / jatuh
// tempo (12 bln); ratePct fail-soft "0%" bila belum ada kohort (pembagi nol).
func renewalKPIView(k db.RenewalKPIsRow) panel.RenewalKPIs {
	return panel.RenewalKPIs{
		Due30:       strconv.FormatInt(k.Due30, 10),
		Grace:       strconv.FormatInt(k.Grace, 10),
		AutoRenew:   strconv.FormatInt(k.AutoActive, 10),
		RenewalRate: ratePct(k.RenewedPast12m, k.DuePast12m),
	}
}
