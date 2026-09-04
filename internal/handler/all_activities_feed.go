package handler

import (
	"sort"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// all_activities_feed.go — entri feed, paginasi (merge-sort → potong → majukan
// cursor komposit), dan pemetaan baris CS→ActivityRow untuk feed terpadu.
// Dipisah dari all_activities_unified.go (ukuran file); logika identik.

// feedEntry = satu baris terpadu + kunci mentah untuk merge-sort & memajukan
// sub-cursor sumbernya. source membedakan asal ("sales" activities / "cs"
// engagements).
type feedEntry struct {
	at     time.Time
	id     int64
	source string
	row    panel.ActivityRow
	cur    subCursor // (created_at, id) baris ini → titik lanjut sub-cursor sumber
}

// pageEntries mengurut entries created_at DESC, memotong ke pageSize, memajukan
// tiap sub-cursor ke baris TERAKHIR yang ditampilkan dari sumbernya, lalu
// merakit cursor komposit halaman berikutnya ("" = halaman terakhir). Diekstrak
// verbatim dari buildUnifiedActivityFeed (ukuran file); logika tak berubah.
func pageEntries(entries []feedEntry, dc dualCursor) ([]panel.ActivityRow, string) {
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].at.Equal(entries[j].at) {
			return entries[i].at.After(entries[j].at)
		}
		if entries[i].id != entries[j].id {
			return entries[i].id > entries[j].id
		}
		return entries[i].source < entries[j].source
	})

	// ── Potong ke pageSize; kelebihan = penanda "masih ada" (sama pola
	// splitPage). Kombinasi ≤ pageSize ⟹ tak ada sumber yang menyentuh +1 ⟹
	// keduanya habis ⟹ tak ada lagi (lihat rasional di BL-41 tasks). ──────────
	more := len(entries) > pageSize
	if more {
		entries = entries[:pageSize]
	}

	// ── Majukan tiap sub-cursor ke baris TERAKHIR yang DITAMPILKAN dari sumber
	// itu (entries sudah urut turun → kemunculan terakhir = terkecil = titik
	// lanjut benar). Sumber tanpa baris tampil → sub-cursor tak berubah (baris
	// yang di-fetch tapi tak tampil akan di-query ulang halaman berikut). ─────
	next := dc
	for _, e := range entries {
		switch e.source {
		case "sales":
			next.act = e.cur
		case "cs":
			next.eng = e.cur
		}
	}
	nextCursor := ""
	if more {
		nextCursor = encodeDualCursor(next)
	}

	items := make([]panel.ActivityRow, 0, len(entries))
	for _, e := range entries {
		items = append(items, e.row)
	}
	return items, nextCursor
}

// engagementFeedRowView memetakan satu baris ListEngagementsFeed → ActivityRow
// bertanda Source="cs" untuk tabel Activities global. Read-only: TargetType/ID =
// desa induk (view menaut ke /accounts/{id}, bukan /activities/{id}). Kolom
// Jenis pakai TypeLabel (engagement_type), Status pakai peta engagement
// (engagementStatusLabel → label + badge daisyUI). Konteks="cs" → chip "CS".
func engagementFeedRowView(e db.ListEngagementsFeedRow) panel.ActivityRow {
	statusLabel, statusBadge := engagementStatusLabel(e.Status)
	owner := ""
	if e.OwnerName != nil {
		owner = *e.OwnerName
	}
	return panel.ActivityRow{
		ID:               e.ID,
		Subject:          e.Subject,
		TargetType:       "account",
		TargetID:         e.AccountID,
		Owner:            owner,
		Status:           statusLabel,
		StatusBadgeClass: statusBadge, // "badge-info"/…; view menambah prefiks "badge "
		Created:          fmtLocal(e.CreatedAt),
		Context:          "cs",
		Source:           "cs",
		TypeLabel:        engagementTypeLabel(e.EngagementType),
	}
}
