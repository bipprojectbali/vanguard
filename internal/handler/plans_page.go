package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// plans_page.go — HALAMAN baca katalog Plans & Pricing. Aksi ada di plans.go /
// plans_status.go. Dipisah karena halaman tumbuh dengan aturan LIHAT (F2 read),
// aksi dengan aturan TULIS (F2 write). Meniru sales_deals_page.go.
//
// Katalog master bounded per-workspace (ListPlansAll, TERMASUK pensiun) → TANPA
// keyset & TANPA F3: RLS satu-satunya pengurung. Harga diformat di sini
// (formatRupiah) → view murni-data.

// PlansList — GET /w/{workspace}/plans. Seluruh katalog (aktif dulu lalu pensiun).
// Bukan pemegang peran CRM (read) → 403 + penjelasan.
func (h *Handler) PlansList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewPlans(ctx) {
		h.renderPlansForbidden(w, r)
		return
	}
	rows, err := h.q(ctx).ListPlansAll(ctx)
	if err != nil {
		h.Log.Error("plans: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.PlanRow, 0, len(rows))
	for _, p := range rows {
		items = append(items, planRowView(p))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Plans & Pricing", "/plans", panel.PlanList(panel.PlanListView{
		Base:     base,
		CanWrite: canWritePlans(ctx),
		Err:      plansErrMsg(r.URL.Query().Get("err")),
		Msg:      plansMsg(r.URL.Query().Get("ok")),
		Items:    items,
	}))
}

// renderPlansForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderPlansForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Plans & Pricing", "/plans", panel.SalesForbidden("Plans & Pricing"))
}

// planRowView memetakan satu plan → baris tabel katalog. Harga diformat rupiah
// (NULL → ""); billing/kategori apa adanya. Tak ada F4 di katalog master (harga
// plan bukan data komersial per-desa yang perlu disamarkan).
func planRowView(p db.Plan) panel.PlanRow {
	return panel.PlanRow{
		ID:       p.ID,
		PlanName: p.PlanName,
		PlanCode: p.PlanCode,
		Category: p.PlanCategory,
		Price:    formatRupiah(p.BasePrice),
		Billing:  deref(p.BillingFrequency),
		Currency: p.Currency,
		Active:   p.IsActive,
	}
}
