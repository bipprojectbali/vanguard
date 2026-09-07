package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_export.go — ekspor CSV read-only dasbor Renewals (BL-94).
// Adapter TIPIS: baca ulang ListRenewals (jalur SAMA dgn tabel HTML) → CSV. F2
// crm:subscriptions read + F3 ownership + F4 masking IDENTIK halaman. Jendela aktif
// (?window=) menyaring baris PERSIS tabel. Tanpa aksi tulis. Iterasi keyset
// menyeluruh (tanpa batas diam) agar CSV memuat SEMUA baris ter-scope, bukan satu
// halaman. Meniru subscriptions_churn_export.go (pola BL-92).

// SubscriptionRenewalsExport — GET /w/{slug}/subscriptions/renewals/export?window=…
func (h *Handler) SubscriptionRenewalsExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	window := normalizeRenewalWindow(r.URL.Query().Get("window"))
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	now := time.Now().In(appTZ)
	today := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

	all, err := h.allRenewals(ctx, filter, uid, window, today)
	if err != nil {
		h.Log.Error("subscriptions: renewals export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	headers := []string{"Desa", "Paket", "Tgl Perpanjang", "Sisa Hari", "Jenis", "Status", "Nilai Sebelumnya", "MRR Kini"}
	rows := make([][]string, 0, len(all))
	for _, s := range all {
		v := renewalRowView(s, now, br)
		rows = append(rows, []string{
			v.Village, v.Plan, v.RenewalDate, v.DaysLeft, v.Type, v.Status, v.PrevValue, v.CurrentMRR,
		})
	}
	if err := writeCSV(w, "subscription-renewals", headers, rows); err != nil {
		h.Log.Error("subscriptions: renewals export write", "err", err)
	}
}

// allRenewals menarik SELURUH baris renewal ter-scope via iterasi keyset (created_at
// DESC), bukan satu halaman — CSV tak boleh diam-diam terpotong. Batas keras
// renewalExportMaxPages mencegah loop tak berujung bila cursor rusak.
func (h *Handler) allRenewals(ctx context.Context, filter db.SubscriptionsListFilter, uid int64, window string, today pgtype.Date) ([]db.ListRenewalsRow, error) {
	const renewalExportMaxPages = 10_000 // guard: 10k × pageSize baris (batas atas waras)
	cursorAt, cursorID := firstPageCursor()
	out := make([]db.ListRenewalsRow, 0)
	for page := 0; page < renewalExportMaxPages; page++ {
		rows, err := h.q(ctx).ListRenewals(ctx, db.ListRenewalsParams{
			CursorCreatedAt: cursorAt,
			CursorID:        cursorID,
			ScopeAll:        filter.ScopeAll,
			IsOwn:           filter.IsOwn,
			Uid:             &uid,
			WindowFilter:    window,
			Today:           today,
			PageSize:        pageSize + 1,
		})
		if err != nil {
			return nil, err
		}
		shown, next := splitPage(rows, func(s db.ListRenewalsRow) (pgtype.Timestamptz, int64) {
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
