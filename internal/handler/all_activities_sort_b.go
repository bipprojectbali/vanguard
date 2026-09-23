package handler

import (
	"context"
	"fmt"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// all_activities_sort_b.go — sumbu status/date untuk feed terpadu
// (buildUnifiedActivityFeed, all_activities_unified.go). Dipisah krn ambang
// File Health (Route/Handler 150 baris); badan tiap fungsi = isi case switch
// semula APA ADANYA, hanya dibungkus wrapper bertipe yang mengembalikan
// entries + error alih-alih menulis langsung ke slice bersama.

// subCursorAsTimestamptz mengubah sub-cursor sumbu Tanggal (val = UnixNano
// 19-digit via subCursorTimeVal) balik jadi pgtype.Timestamptz untuk param
// CursorVal ListAllActivitiesSortByDate/ListEngagementsFeedSortByDate.
// !hasCursor → nilai tak dipakai query (predikat keyset dilewati); kembalikan
// zero value.
func subCursorAsTimestamptz(c genSubCursor) pgtype.Timestamptz {
	if !c.hasCursor {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Unix(0, parseSubCursorTimeVal(c.val)).UTC(), Valid: true}
}

func (h *Handler) unifiedFeedByStatus(
	ctx context.Context,
	dc dualCursorGen,
	uid int64,
	query, useDir string,
	actFilter db.ActivitiesListFilter,
	engFilter db.EngagementsListFilter,
	engGate bool,
	names map[int64]string,
) ([]feedEntry, error) {
	entries := make([]feedEntry, 0, pageSize*2)
	actRows, err := h.q(ctx).ListAllActivitiesSortByStatus(ctx, db.ListAllActivitiesSortByStatusParams{
		HasCursor: dc.act.hasCursor, Dir: useDir, CursorIsNull: dc.act.isNull, CursorVal: dc.act.val, CursorID: dc.act.id,
		ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("all-activities: list activities sort status: %w", err)
	}
	for _, a := range actRows {
		val, isNull := "", a.Status == nil
		if !isNull {
			val = *a.Status
		}
		entries = append(entries, feedEntry{id: a.ID, source: "sales", row: activityRowView(a, names), sortVal: val, sortNull: isNull})
	}
	if engGate {
		// engagements.status NOT NULL (beda dari activities.status) — selalu
		// non-null, tapi query & param tetap simetris (tanpa CursorIsNull, lihat
		// ListEngagementsFeedSortByStatusParams — mirror ListActivitiesSortByKind
		// bukan ...ByOwner).
		engRows, err := h.q(ctx).ListEngagementsFeedSortByStatus(ctx, db.ListEngagementsFeedSortByStatusParams{
			HasCursor: dc.eng.hasCursor, Dir: useDir, CursorVal: dc.eng.val, CursorID: dc.eng.id,
			ScopeAll: engFilter.ScopeAll, IsOwn: engFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, fmt.Errorf("all-activities: list engagements sort status: %w", err)
		}
		for _, e := range engRows {
			entries = append(entries, feedEntry{
				id: e.ID, source: "cs",
				row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
				sortVal: e.Status,
			})
		}
	}
	return entries, nil
}

func (h *Handler) unifiedFeedByDate(
	ctx context.Context,
	dc dualCursorGen,
	uid int64,
	query, useDir string,
	actFilter db.ActivitiesListFilter,
	engFilter db.EngagementsListFilter,
	engGate bool,
	names map[int64]string,
) ([]feedEntry, error) {
	entries := make([]feedEntry, 0, pageSize*2)
	actRows, err := h.q(ctx).ListAllActivitiesSortByDate(ctx, db.ListAllActivitiesSortByDateParams{
		HasCursor: dc.act.hasCursor, Dir: useDir, CursorVal: subCursorAsTimestamptz(dc.act), CursorID: dc.act.id,
		ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("all-activities: list activities sort date: %w", err)
	}
	for _, a := range actRows {
		entries = append(entries, feedEntry{
			id: a.ID, source: "sales", row: activityRowView(a, names),
			sortVal: subCursorTimeVal(tsTime(a.CreatedAt).UnixNano()),
		})
	}
	if engGate {
		engRows, err := h.q(ctx).ListEngagementsFeedSortByDate(ctx, db.ListEngagementsFeedSortByDateParams{
			HasCursor: dc.eng.hasCursor, Dir: useDir, CursorVal: subCursorAsTimestamptz(dc.eng), CursorID: dc.eng.id,
			ScopeAll: engFilter.ScopeAll, IsOwn: engFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, fmt.Errorf("all-activities: list engagements sort date: %w", err)
		}
		for _, e := range engRows {
			entries = append(entries, feedEntry{
				id: e.ID, source: "cs",
				row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
				sortVal: subCursorTimeVal(tsTime(e.CreatedAt).UnixNano()),
			})
		}
	}
	return entries, nil
}
