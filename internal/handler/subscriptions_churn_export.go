package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_churn_export.go — ekspor CSV read-only dasbor Churn (BL-92). Adapter
// TIPIS: baca ulang ListChurned (jalur SAMA dgn tabel HTML) → CSV. F2 crm:subscriptions
// read + F3 ownership + F4 masking IDENTIK halaman. Tipe aktif (?type=) menyaring baris
// PERSIS tabel. Tanpa aksi tulis. Iterasi keyset menyeluruh (tanpa batas diam) agar CSV
// memuat SEMUA baris ter-scope, bukan hanya satu halaman.

// SubscriptionChurnExport — GET /w/{slug}/subscriptions/churn/export?type=…
func (h *Handler) SubscriptionChurnExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	typeFilter := normalizeChurnTypeFilter(r.URL.Query().Get("type"))
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	all, err := h.allChurned(ctx, filter, uid, typeFilter)
	if err != nil {
		h.Log.Error("subscriptions: churn export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: churn export members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	headers := []string{"Desa", "Paket", "MRR Hilang", "Alasan", "Tipe", "Tgl Churn", "Tenure", "CS"}
	rows := make([][]string, 0, len(all))
	for _, s := range all {
		v := churnRowView(s, names, br)
		rows = append(rows, []string{
			v.Village, v.Plan, v.LostMRR, v.Reason, v.Type, v.ChurnDate, v.Tenure, v.CSM,
		})
	}
	if err := writeCSV(w, "subscription-churn", headers, rows); err != nil {
		h.Log.Error("subscriptions: churn export write", "err", err)
	}
}

// allChurned menarik SELURUH baris churn ter-scope via iterasi keyset (created_at
// DESC), bukan satu halaman — CSV tak boleh diam-diam terpotong. Batas keras
// churnExportMaxPages mencegah loop tak berujung bila cursor rusak.
func (h *Handler) allChurned(ctx context.Context, filter db.SubscriptionsListFilter, uid int64, typeFilter string) ([]db.ListChurnedRow, error) {
	const churnExportMaxPages = 10_000 // guard: 10k × pageSize baris (batas atas waras)
	cursorAt, cursorID := firstPageCursor()
	out := make([]db.ListChurnedRow, 0)
	for page := 0; page < churnExportMaxPages; page++ {
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
			return nil, err
		}
		shown, next := splitPage(rows, func(s db.ListChurnedRow) (pgtype.Timestamptz, int64) {
			return s.CreatedAt, s.ID
		})
		out = append(out, shown...)
		if next == "" || len(shown) == 0 {
			break
		}
		last := shown[len(shown)-1]
		cursorAt, cursorID = last.CreatedAt, last.ID
	}
	return out, nil
}
