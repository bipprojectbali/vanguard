package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// subscriptions_detail_rows.go — helper pemetaan baris (rantai renewal, item
// paket) & resolusi label plan untuk detail langganan di subscriptions_detail.go.

// renewalChainView memuat riwayat rantai renewal (lama→baru) untuk kartu di
// detail. Best-effort (mirror dealQuotesPreview): gagal query → nil + log, detail
// tetap terbaca. ARR tiap periode disamarkan mengikuti kebijakan yang sama.
func (h *Handler) renewalChainView(ctx context.Context, id int64, businessRole string, canARR bool) []panel.SubChainRow {
	rows, err := h.q(ctx).ListRenewalChain(ctx, id)
	if err != nil {
		h.Log.Error("subscriptions: renewal chain", "err", err)
		return nil
	}
	out := make([]panel.SubChainRow, 0, len(rows))
	for _, c := range rows {
		out = append(out, panel.SubChainRow{
			ID:     c.ID,
			IsThis: c.ID == id,
			Status: c.Status,
			MRR:    maskARR(formatRupiah(c.Mrr), businessRole),
			ARR:    maskSubscriptionARR(formatRupiah(c.Arr), canARR),
			Start:  dateStr(c.StartDate),
			End:    dateStr(c.EndDate),
		})
	}
	return out
}

// subItemRows memetakan baris subscription_items → baris tabel item detail (BL-88
// PR2b). Nilai komersial disamarkan F4: MRR & Subtotal ikut kebijakan maskARR (br),
// ARR ikut kapabilitas ARR (maskSubscriptionARR). PlanName NULL (plan terhapus /
// item tanpa plan) → "—" via view (orDash). Kuantitas & harga satuan bukan sensitif.
func subItemRows(items []db.ListSubscriptionItemsWithPlanRow, businessRole string, canARR bool) []panel.SubItemRow {
	out := make([]panel.SubItemRow, 0, len(items))
	for _, it := range items {
		name := ""
		if it.PlanName != nil {
			name = *it.PlanName
		}
		out = append(out, panel.SubItemRow{
			PlanName:  name,
			Quantity:  strconv.FormatInt(int64(it.Quantity), 10),
			UnitPrice: formatRupiah(it.UnitPrice),
			Subtotal:  maskARR(formatRupiah(it.Subtotal), businessRole),
			MRR:       maskARR(formatRupiah(it.Mrr), businessRole),
			ARR:       maskSubscriptionARR(formatRupiah(it.Arr), canARR),
		})
	}
	return out
}

// planLabel meresolusi nama plan untuk detail (nama saja). id NULL (BL-88 PR2b
// langganan multi-paket → parent plan_id NULL) → "—". Gagal baca → "Plan #<id>"
// cadangan (bukan 500): detail langganan tetap terbaca.
func (h *Handler) planLabel(ctx context.Context, id *int64) string {
	if id == nil {
		return "—"
	}
	p, err := h.q(ctx).GetPlan(ctx, *id)
	if err != nil {
		return "Plan #" + strconv.FormatInt(*id, 10)
	}
	return p.PlanName
}
