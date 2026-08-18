package handler

import (
	"context"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_subscriptions_export.go — loop keyset LENGKAP untuk CSV Subscription
// Report, dipisah dari reports_subscriptions.go (yang sudah dekat batas 150
// baris "route/handler" & concern beda: render satu halaman vs kumpulkan
// SEMUA baris). "No silent caps" (CLAUDE.md #13): export yang diam-diam
// terpotong ke halaman pertama adalah bug tersembunyi, bukan fitur.

// paginateAll mengumpulkan SELURUH baris lewat keyset (bukan cuma page
// pertama) — generik atas tipe baris apa pun yang punya (created_at, id).
// fetch dipanggil ulang dengan cursor baris terakhir sampai halaman < pageSize
// (tanda halaman terakhir, pola sama dgn splitPage). Mulai dari
// firstPageCursor() (sentinel infinity), BUKAN zero-value — query keyset
// membandingkan "< cursor", jadi zero-value (Valid:false) meloloskan NOL baris.
func paginateAll[T any](
	fetch func(cursorAt pgtype.Timestamptz, cursorID int64) ([]T, error),
	keyOf func(T) (pgtype.Timestamptz, int64),
) ([]T, error) {
	var all []T
	cursorAt, cursorID := firstPageCursor()
	for {
		page, err := fetch(cursorAt, cursorID)
		if err != nil {
			return nil, err
		}
		hasMore := len(page) > pageSize
		if hasMore {
			page = page[:pageSize]
		}
		all = append(all, page...)
		if !hasMore || len(page) == 0 {
			break
		}
		cursorAt, cursorID = keyOf(page[len(page)-1])
	}
	return all, nil
}

// reportsSubscriptionsExportRows merakit header+baris CSV untuk section aktif,
// mengumpulkan SEMUA baris (paginateAll) — F3 ownership SAMA dgn tampilan HTML.
func (h *Handler) reportsSubscriptionsExportRows(ctx context.Context, section string) ([]string, [][]string, error) {
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	q := h.q(ctx)

	if section == reportSectionChurn {
		names, err := h.memberNameMap(ctx)
		if err != nil {
			return nil, nil, err
		}
		all, err := paginateAll(
			func(cursorAt pgtype.Timestamptz, cursorID int64) ([]db.ListChurnedRow, error) {
				return q.ListChurned(ctx, db.ListChurnedParams{
					CursorCreatedAt: cursorAt, CursorID: cursorID,
					ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
					PageSize: pageSize + 1,
				})
			},
			func(s db.ListChurnedRow) (pgtype.Timestamptz, int64) { return s.CreatedAt, s.ID },
		)
		if err != nil {
			return nil, nil, err
		}
		header := []string{"Desa", "Paket", "MRR Hilang", "Alasan", "Tipe", "Tgl Churn", "CSM"}
		rows := make([][]string, 0, len(all))
		for _, s := range all {
			v := churnRowView(s, names, br)
			rows = append(rows, []string{v.Village, v.Plan, v.LostMRR, v.Reason, v.Type, v.ChurnDate, v.CSM})
		}
		return header, rows, nil
	}

	now := time.Now().In(appTZ)
	today := reportTodayDate(now)
	all, err := paginateAll(
		func(cursorAt pgtype.Timestamptz, cursorID int64) ([]db.ListRenewalsRow, error) {
			return q.ListRenewals(ctx, db.ListRenewalsParams{
				CursorCreatedAt: cursorAt, CursorID: cursorID,
				ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
				WindowFilter: "due", Today: today, PageSize: pageSize + 1,
			})
		},
		func(s db.ListRenewalsRow) (pgtype.Timestamptz, int64) { return s.CreatedAt, s.ID },
	)
	if err != nil {
		return nil, nil, err
	}
	header := []string{"Desa", "Paket", "Tgl Perpanjang", "Sisa Hari", "Jenis", "Status", "Prev", "Kini"}
	rows := make([][]string, 0, len(all))
	for _, s := range all {
		v := renewalRowView(s, now, br)
		rows = append(rows, []string{v.Village, v.Plan, v.RenewalDate, v.DaysLeft, v.Type, v.Status, v.PrevValue, v.CurrentMRR})
	}
	return header, rows, nil
}
