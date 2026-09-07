package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_kpi.go — 4 KPI header halaman Subscription Lists (BL-95). Satu
// query agregat (SubscriptionListKPIs) di-scope ownership SAMA dgn ListSubscriptions;
// nilai diformat di sini (view murni-data). Meniru subscriptions_renewals_kpi.go
// (BL-94). Dipisah dari subscriptions_page.go demi ambang Route/Handler (150 baris).
//
// Masking F4: nilai Rp KPI (Total MRR/ARR) SENGAJA tak dimasking — hanya role dgn
// crm:subscriptions read yang mencapai halaman ini (canViewSubscriptions), dan
// maskARR hanya menyamarkan MRR untuk Support yang TAK punya read → tak pernah
// sampai ke sini. Agregat (bukan per-baris milik role) → tak ada sumbu ownership
// nilai untuk disamarkan.

// subscriptionListKPI menjalankan query agregat + memformat 4 kartu KPI. Gagal →
// error dipropagasi; pemanggil memilih fail-soft (render tabel tanpa KPI — KPI
// bukan data kritis untuk membaca daftar langganan), sejalan BL-94.
func (h *Handler) subscriptionListKPI(ctx context.Context, filter db.SubscriptionsListFilter, uid int64, today pgtype.Date) (panel.SubKPIs, error) {
	k, err := h.q(ctx).SubscriptionListKPIs(ctx, db.SubscriptionListKPIsParams{
		Today:    today,
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
	})
	if err != nil {
		return panel.SubKPIs{}, err
	}
	return subscriptionListKPIView(k), nil
}

// subscriptionListKPIView memformat baris agregat → kartu KPI. ChurnRate memakai
// definisi churn% app (churned / (active + churned)); ratePct fail-soft "—" bila
// belum ada langganan (pembagi nol). ARR = total_arr (MRR×12, dihitung di SQL).
func subscriptionListKPIView(k db.SubscriptionListKPIsRow) panel.SubKPIs {
	return panel.SubKPIs{
		TotalMRR:   formatRupiah(k.TotalMrr),
		NewMRR:     formatRupiah(k.NewMrr),
		ARR:        formatRupiah(k.TotalArr),
		ActiveSubs: strconv.FormatInt(k.ActiveCount, 10),
		Accounts:   strconv.FormatInt(k.TotalAccounts, 10),
		ChurnRate:  ratePct(k.Churned30, k.ActiveCount+k.Churned30),
	}
}
