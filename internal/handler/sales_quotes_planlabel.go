package handler

import (
	"context"

	"go_starter/internal/db"
)

// sales_quotes_planlabel.go — resolusi label Plan (nama + harga) untuk baris item
// quote & picker plan di builder. Dipisah dari sales_quotes_detail.go semata untuk
// file health; satu concern: "plan_id → teks tampilan".

// quotePlanLabels membangun peta plan_id → label untuk baris item. Sumber utama
// ListPlans (satu query); plan yang sudah PENSIUN (is_active=false, tak muncul di
// ListPlans) diresolusi individual via GetPlan — item lama tetap ter-nama. Baris
// per-quote bounded → jumlah GetPlan susulan kecil.
func (h *Handler) quotePlanLabels(ctx context.Context, items []db.QuoteItem, plans []db.Plan) map[int64]string {
	labels := make(map[int64]string, len(plans)+len(items))
	for _, p := range plans {
		labels[p.ID] = quotePlanLabel(p)
	}
	for _, it := range items {
		if it.PlanID == nil {
			continue
		}
		if _, ok := labels[*it.PlanID]; ok {
			continue
		}
		p, err := h.q(ctx).GetPlan(ctx, *it.PlanID)
		if err != nil {
			continue // biarkan pemanggil pakai cadangan "Plan #<id>"
		}
		labels[p.ID] = quotePlanLabel(p)
	}
	return labels
}

// quotePlanLabel = label plan untuk picker/baris: "Nama — Rp harga". Harga = base
// (referensi tampilan; unit_price sebenarnya di-SNAPSHOT saat item dibuat).
func quotePlanLabel(p db.Plan) string {
	label := p.PlanName
	if price := formatRupiah(p.BasePrice); price != "" {
		label += " — " + price
	}
	return label
}
