package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// activities_timeline.go — pembantu timeline aktivitas reusable untuk halaman
// detail entitas (Account, Deal, Contact, Ticket). Dipisah agar tiap detail handler
// tak perlu menduplikasi logika muat + petakan; meniru pola dealQuotesPreview.
//
// Gate izin tidak ada di sini — handler detail entitas masing-masing yang
// bertanggung jawab (siapa pun yang bisa lihat entitasnya dapat timelinenya).

// activityTimelineLimit = jumlah aktivitas terbaru yang ditampilkan di kartu
// timeline detail entitas. Daftar penuh ada di /activities.
const activityTimelineLimit = 5

// activitiesTimelineFor memuat aktivitas terbaru untuk satu entitas target
// dan mengembalikan data siap-render panel.ActivityTimelineView.
// Best-effort: gagal muat names → nama pemilik "—" (detail entitas tetap terbaca).
// Gagal muat activities → slice kosong (kartu tampil dengan pesan kosong).
func (h *Handler) activitiesTimelineFor(
	ctx context.Context,
	base, targetType string,
	targetID int64,
	canWrite bool,
) panel.ActivityTimelineView {
	at, id := firstPageCursor()
	rows, err := h.q(ctx).ListActivitiesByTarget(ctx, db.ListActivitiesByTargetParams{
		TargetType:      targetType,
		TargetID:        targetID,
		CursorCreatedAt: at,
		CursorID:        id,
		PageSize:        activityTimelineLimit + 1,
	})
	if err != nil {
		h.Log.Error("activities_timeline: list", "target_type", targetType, "target_id", targetID, "err", err)
		rows = nil
	}

	shown, nextCursor := splitPage(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("activities_timeline: members", "err", err)
		names = nil
	}

	items := make([]panel.ActivityTimelineItem, 0, len(shown))
	for _, a := range shown {
		items = append(items, activityTimelineItemView(a, names))
	}

	return panel.ActivityTimelineView{
		Base:       base,
		TargetType: targetType,
		TargetID:   targetID,
		CanWrite:   canWrite,
		Items:      items,
		NextCursor: nextCursor,
	}
}

// activityTimelineItemView memetakan satu baris DB → ActivityTimelineItem.
// Date = waktu lokal (gotcha #14). Status kosong ("") untuk kind tanpa status
// (call/chat/note) — view menampilkan "—".
func activityTimelineItemView(a db.Activity, names map[int64]string) panel.ActivityTimelineItem {
	return panel.ActivityTimelineItem{
		ID:      a.ID,
		Kind:    a.Kind,
		Subject: a.Subject,
		Date:    fmtLocal(a.CreatedAt),
		Status:  deref(a.Status),
		Owner:   ownerName(a.OwnerID, names),
	}
}
