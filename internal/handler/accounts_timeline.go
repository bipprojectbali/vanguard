package handler

import (
	"context"
	"sort"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_timeline.go — linimasa TERPADU (read-only) untuk detail Account (BL-31).
// Menggabungkan dua sumber-kebenaran terpisah yang selama ini tak pernah bertemu:
//   - activities (jalur Sales, activity_context='sales')
//   - engagements (jalur Customer Success, tabel sendiri)
// menjadi satu kartu kronologis per-desa. OPSI 2 BL-31: nol duplikasi data —
// masing-masing tabel tetap sumber-kebenarannya; kartu ini hanya MEMBACA gabungan.
//
// Gated union (BL-31 #3):
//   - Baris engagement HANYA dimuat bila canViewEngagements(ctx) (F2 modul CS).
//     Sales/Support tak punya crm:engagements → sumber CS tak pernah ikut,
//     jadi tak ada kebocoran data CS ke peran non-CS.
//   - F3 ownership: pemanggil (AccountDetail) sudah menolak 404 desa di luar
//     cakupan SEBELUM sampai sini; kepemilikan engagement mengikuti kolom akun
//     yang sama (account_owner/assigned_csm/backup_csm) — "boleh lihat desa" ⇒
//     "boleh lihat engagement-nya". Konsisten dgn timeline activity yang memang
//     di-gate di level entitas (lihat activities_timeline.go).
//   - RLS h.q(ctx) mengisolasi tenant di bawah semuanya.
//
// Paginasi (BL-31 #2, opsi b): kartu hanya menampilkan activityTimelineLimit
// baris terbaru. Ambil (limit+1) per-sumber lalu merge-sort di handler dan
// potong — tak perlu keyset lintas-tabel (kunci waktu beda: activities.created_at
// vs engagements.scheduled_at). Waktu tampil di zona workspace (gotcha #14).

// timelineEntry = satu baris terpadu + kunci waktu mentah untuk merge-sort.
// item = data siap-render; at = waktu untuk pengurutan lintas-sumber.
type timelineEntry struct {
	at   time.Time
	item panel.ActivityTimelineItem
}

// accountUnifiedTimelineFor merakit linimasa terpadu Sales+CS untuk satu desa.
// names = peta user_id→nama yang sudah dirakit pemanggil (accountDetailView) —
// dioper agar tak query anggota dua kali. canWrite menyalakan tombol
// "+ Log Aktivitas" (jalur Sales; linimasa sendiri read-only).
func (h *Handler) accountUnifiedTimelineFor(
	ctx context.Context,
	base string,
	a db.Account,
	names map[int64]string,
	canWrite bool,
) panel.ActivityTimelineView {
	entries := make([]timelineEntry, 0, 2*activityTimelineLimit)

	// ── Sumber Sales: activities target=account ────────────────────────────
	at, id := firstPageCursor()
	acts, err := h.q(ctx).ListActivitiesByTarget(ctx, db.ListActivitiesByTargetParams{
		TargetType:      "account",
		TargetID:        a.ID,
		CursorCreatedAt: at,
		CursorID:        id,
		PageSize:        activityTimelineLimit + 1,
	})
	if err != nil {
		h.Log.Error("accounts_timeline: activities", "account_id", a.ID, "err", err)
		acts = nil
	}
	actShown, actNext := splitPage(acts, func(a db.Activity) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})
	for _, act := range actShown {
		item := activityTimelineItemView(act, names)
		item.Source = "sales"
		entries = append(entries, timelineEntry{at: tsTime(act.CreatedAt), item: item})
	}

	// ── Sumber CS: engagements (F2-gated) ──────────────────────────────────
	// Hanya dimuat bila aktor boleh melihat modul Engagements — kunci "gated
	// union": tanpa gerbang ini, Sales yang membuka desanya akan melihat baris CS.
	engMore := false
	if canViewEngagements(ctx) {
		engs, err := h.q(ctx).ListEngagementsByAccount(ctx, db.ListEngagementsByAccountParams{
			AccountID: a.ID,
			PageSize:  activityTimelineLimit + 1,
		})
		if err != nil {
			h.Log.Error("accounts_timeline: engagements", "account_id", a.ID, "err", err)
			engs = nil
		}
		if len(engs) > activityTimelineLimit {
			engMore = true
			engs = engs[:activityTimelineLimit]
		}
		for _, e := range engs {
			entries = append(entries, timelineEntry{
				at:   tsTime(e.ScheduledAt),
				item: engagementTimelineItemView(e),
			})
		}
	}

	// ── Merge-sort lintas-sumber (waktu turun, id turun sebagai tie-break) ──
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].at.Equal(entries[j].at) {
			return entries[i].item.ID > entries[j].item.ID
		}
		return entries[i].at.After(entries[j].at)
	})

	// "Lihat semua" muncul bila ada baris tersembunyi dari sumber MANA PUN:
	// activities masih bersambung, engagements melebihi limit, atau gabungan
	// dua sumber melebihi limit (sebagian terpotong di bawah).
	more := actNext != "" || engMore || len(entries) > activityTimelineLimit
	if len(entries) > activityTimelineLimit {
		entries = entries[:activityTimelineLimit]
	}

	items := make([]panel.ActivityTimelineItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, e.item)
	}

	// NextCursor dipakai view HANYA sebagai flag "ada lagi" → tampilkan tautan
	// "Lihat semua aktivitas". Sentinel non-kosong; nilai spesifik tak dipakai
	// (kartu terpadu tak melanjutkan keyset — daftar penuh ada di /activities).
	nextCursor := ""
	if more {
		nextCursor = "more"
	}

	return panel.ActivityTimelineView{
		Base:       base,
		TargetType: "account",
		TargetID:   a.ID,
		CanWrite:   canWrite,
		Items:      items,
		NextCursor: nextCursor,
		Title:      "Linimasa", // kartu menggabungkan sumber Sales + CS (BL-31)
	}
}
