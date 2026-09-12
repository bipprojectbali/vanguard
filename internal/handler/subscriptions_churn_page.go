package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_churn_page.go — dasbor Churn (Menu 5.2/5.4, READ-ONLY). Menyorot
// langganan yang telah berhenti (status Cancelled/Churned) + nilai MRR yang hilang.
// Aksi churn SENGAJA tidak di sini — ditandai dari detail langganan (subscriptions_
// churn.go); dasbor ini hanya baca (sejalan wireframe 5.2/5.4).
//
// Gerbang sama dengan daftar langganan: F2 crm:subscriptions read + F3 ownership
// (subscription_owner) di layer query. MRR Hilang = lost_value_mrr, nilai komersial
// → maskARR (F4, diperbaiki audit FLS M9-1). Keyset created_at DESC (reuse
// pageCursor/splitPage).
//
// BL-92: didesain ulang jadi dasbor (permintaan user 7 Sep, wireframe). Tambahan
// banner peringatan (token warning), 4 kartu KPI berjendela TETAP per label mockup
// (bukan tab periode) & SELALU global (tab Tipe hanya menyaring tabel + CSV), kolom
// Tenure (cancellation_date − start_date, bulan), serta ekspor CSV read-only.
// KPI REUSE fungsi baca agregat Report 8.4 (ReportRetention/ReportChurnAge) apa adanya.
//
// KPI dasbor, filter tipe (churnTypeTab), & pemetaan baris (churnRowView) di
// subscriptions_churn_kpi.go.

// SubscriptionChurnList — GET /w/{slug}/subscriptions/churn. Dasbor read-only
// langganan berhenti, ter-scope kepemilikan (F3) + filter tipe churn opsional.
func (h *Handler) SubscriptionChurnList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	typeFilter := normalizeChurnTypeFilter(r.URL.Query().Get("type"))
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListChurned(ctx, db.ListChurnedParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		TypeFilter:      typeFilter,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: churn list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(s db.ListChurnedRow) (pgtype.Timestamptz, int64) {
		return s.CreatedAt, s.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: churn members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.ChurnRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, churnRowView(s, names, br))
	}

	// KPI global (semua tipe) — tab Tipe hanya menyaring tabel, bukan kartu (BL-92).
	kpis, err := h.churnKPIs(ctx, filter, uid, br)
	if err != nil {
		h.Log.Error("subscriptions: churn kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Churn", "/subscriptions/churn",
		panel.ChurnList(panel.ChurnView{
			Base:       base,
			Type:       typeFilter,
			Types:      churnTypeFilterOptions(),
			KPIs:       kpis,
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Items:      items,
			NextCursor: nextCursor,
			After:      r.URL.Query().Get("after"),
			Trail:      pageTrail(r),
		}))
}

// churnKPIs, tenureMonthsStr, subTenureMonthsStr, normalizeChurnTypeFilter,
// churnTypeFilterOptions, churnRowView di subscriptions_churn_kpi.go.
