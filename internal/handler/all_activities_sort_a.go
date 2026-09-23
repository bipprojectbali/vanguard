package handler

import (
	"context"
	"fmt"

	"go_starter/internal/db"
)

// all_activities_sort_a.go — sumbu kind/subject/owner untuk feed terpadu
// (buildUnifiedActivityFeed, all_activities_unified.go). Dipisah krn ambang
// File Health (Route/Handler 150 baris); badan tiap fungsi = isi case switch
// semula APA ADANYA, hanya dibungkus wrapper bertipe yang mengembalikan
// entries + error alih-alih menulis langsung ke slice bersama.

func (h *Handler) unifiedFeedByKind(
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
	actRows, err := h.q(ctx).ListAllActivitiesSortByKind(ctx, db.ListAllActivitiesSortByKindParams{
		HasCursor: dc.act.hasCursor, Dir: useDir, CursorVal: dc.act.val, CursorID: dc.act.id,
		ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("all-activities: list activities sort kind: %w", err)
	}
	for _, a := range actRows {
		entries = append(entries, feedEntry{id: a.ID, source: "sales", row: activityRowView(a, names), sortVal: a.Kind})
	}
	if engGate {
		engRows, err := h.q(ctx).ListEngagementsFeedSortByType(ctx, db.ListEngagementsFeedSortByTypeParams{
			HasCursor: dc.eng.hasCursor, Dir: useDir, CursorVal: dc.eng.val, CursorID: dc.eng.id,
			ScopeAll: engFilter.ScopeAll, IsOwn: engFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, fmt.Errorf("all-activities: list engagements sort type: %w", err)
		}
		for _, e := range engRows {
			entries = append(entries, feedEntry{
				id: e.ID, source: "cs",
				row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
				sortVal: e.EngagementType,
			})
		}
	}
	return entries, nil
}

func (h *Handler) unifiedFeedBySubject(
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
	actRows, err := h.q(ctx).ListAllActivitiesSortBySubject(ctx, db.ListAllActivitiesSortBySubjectParams{
		HasCursor: dc.act.hasCursor, Dir: useDir, CursorVal: dc.act.val, CursorID: dc.act.id,
		ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("all-activities: list activities sort subject: %w", err)
	}
	for _, a := range actRows {
		entries = append(entries, feedEntry{id: a.ID, source: "sales", row: activityRowView(a, names), sortVal: a.Subject})
	}
	if engGate {
		engRows, err := h.q(ctx).ListEngagementsFeedSortBySubject(ctx, db.ListEngagementsFeedSortBySubjectParams{
			HasCursor: dc.eng.hasCursor, Dir: useDir, CursorVal: dc.eng.val, CursorID: dc.eng.id,
			ScopeAll: engFilter.ScopeAll, IsOwn: engFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, fmt.Errorf("all-activities: list engagements sort subject: %w", err)
		}
		for _, e := range engRows {
			entries = append(entries, feedEntry{
				id: e.ID, source: "cs",
				row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
				sortVal: e.Subject,
			})
		}
	}
	return entries, nil
}

func (h *Handler) unifiedFeedByOwner(
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
	actRows, err := h.q(ctx).ListAllActivitiesSortByOwner(ctx, db.ListAllActivitiesSortByOwnerParams{
		HasCursor: dc.act.hasCursor, Dir: useDir, CursorIsNull: dc.act.isNull, CursorVal: dc.act.val, CursorID: dc.act.id,
		ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("all-activities: list activities sort owner: %w", err)
	}
	for _, a := range actRows {
		// Kunci sort Pemilik = nama/email resolusi peta anggota (ownerName),
		// PERSIS yang ditampilkan — sama disiplin dgn ActivitiesList (BL-157j).
		val, isNull := "", a.OwnerID == nil
		if !isNull {
			val = ownerName(a.OwnerID, names)
		}
		entries = append(entries, feedEntry{id: a.ID, source: "sales", row: activityRowView(a, names), sortVal: val, sortNull: isNull})
	}
	if engGate {
		engRows, err := h.q(ctx).ListEngagementsFeedSortByOwner(ctx, db.ListEngagementsFeedSortByOwnerParams{
			HasCursor: dc.eng.hasCursor, Dir: useDir, CursorIsNull: dc.eng.isNull, CursorVal: dc.eng.val, CursorID: dc.eng.id,
			ScopeAll: engFilter.ScopeAll, IsOwn: engFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, fmt.Errorf("all-activities: list engagements sort owner: %w", err)
		}
		for _, e := range engRows {
			val, isNull := "", e.OwnerName == nil
			if !isNull {
				val = *e.OwnerName
			}
			entries = append(entries, feedEntry{
				id: e.ID, source: "cs",
				row:      engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
				sortVal:  val,
				sortNull: isNull,
			})
		}
	}
	return entries, nil
}
