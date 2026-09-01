package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// plans_page.go — HALAMAN baca katalog Plans & Pricing. Aksi ada di plans.go /
// plans_status.go. Dipisah karena halaman tumbuh dengan aturan LIHAT (F2 read),
// aksi dengan aturan TULIS (F2 write). Meniru sales_deals_page.go.
//
// Katalog master per-workspace (ListPlansAll, TERMASUK pensiun) → keyset
// (created_at DESC, id DESC) lewat ?after=, TANPA F3: RLS satu-satunya
// pengurung. Harga diformat di sini (formatRupiah) → view murni-data.

// PlansList — GET /w/{workspace}/plans. Katalog terpaginate (terbaru dulu).
// Bukan pemegang peran CRM (read) → 403 + penjelasan.
func (h *Handler) PlansList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewPlans(ctx) {
		h.renderPlansForbidden(w, r)
		return
	}
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListPlansAll(ctx, db.ListPlansAllParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("plans: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(p db.Plan) (pgtype.Timestamptz, int64) {
		return p.CreatedAt, p.ID
	})
	items := make([]panel.PlanRow, 0, len(shown))
	for _, p := range shown {
		items = append(items, planRowView(p))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Plans & Pricing", "/plans", panel.PlanList(panel.PlanListView{
		Base:       base,
		CanWrite:   canWritePlans(ctx),
		Err:        plansErrMsg(r.URL.Query().Get("err")),
		Msg:        plansMsg(r.URL.Query().Get("ok")),
		Items:      items,
		NextCursor: nextCursor,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
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
