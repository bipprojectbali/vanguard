package handler

import (
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_timeline_view.go — pemetaan baris engagement → item linimasa terpadu
// & helper tsTime, dipisah dari accounts_timeline.go (ukuran file). Perakitan
// linimasa tetap di file induk; di sini hanya view-mapping satu baris. Perilaku
// identik; hanya organisasi file yang berubah.

// engagementTimelineItemView memetakan satu baris engagement → item linimasa
// terpadu. Source="cs" → chip CS; TypeLabel + StatusBadgeClass dipakai view
// karena peta jenis/status engagement berbeda dari activity. Waktu = scheduled_at
// di zona workspace (gotcha #14), konsisten dgn pengurutan ListEngagements.
func engagementTimelineItemView(e db.ListEngagementsByAccountRow) panel.ActivityTimelineItem {
	statusLabel, statusBadge := engagementStatusLabel(e.Status)
	owner := ""
	if e.OwnerName != nil {
		owner = *e.OwnerName
	}
	return panel.ActivityTimelineItem{
		ID:               e.ID,
		Subject:          e.Subject,
		Date:             fmtLocal(e.ScheduledAt),
		Status:           statusLabel,
		StatusBadgeClass: "badge " + statusBadge,
		Owner:            owner,
		Source:           "cs",
		TypeLabel:        engagementTypeLabel(e.EngagementType),
	}
}

// tsTime menormalkan pgtype.Timestamptz → time.Time untuk merge-sort. Baris tak
// valid (mustahil di praktik: kedua kolom NOT NULL) jatuh ke zero-time → terurut
// paling akhir, tak pernah panik.
func tsTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}
