package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// all_activities_unified.go — linimasa TERPADU untuk halaman Activities GLOBAL
// (/activity-log, BL-41). Menaikkan gagasan BL-31 (detail Account) ke feed
// lintas-desa berpaginasi: menggabungkan dua sumber-kebenaran terpisah dalam
// SATU daftar kronologis —
//   - activities (jalur Sales/umum, tabel activities)   → ListAllActivities
//   - engagements (jalur Customer Success, tabel sendiri) → ListEngagementsFeed
//
// Nol duplikasi data (OPSI 2 BL-31): masing-masing tabel tetap sumber-kebenarannya;
// feed ini hanya MEMBACA gabungan (baris CS read-only — CRUD-nya di modul CS).
//
// Tiga keputusan yang menyatukannya:
//
//  1. GATED UNION (BL-41 #1). Lengan CS hanya dimuat bila aktor boleh melihat
//     modul Engagements (canViewEngagements — crm:engagements) ATAU platform.
//     Sales/Support tak punya crm:engagements → lengan CS tak pernah ikut, jadi
//     tak ada kebocoran baris CS ke halaman /activity-log mereka.
//
//  2. SUMBU WAKTU DISAMAKAN (BL-41 #3). Kedua sumber diurut created_at DESC
//     ("kapan DICATAT"). engagements.scheduled_at (bisa jauh ke depan) SENGAJA
//     tak dipakai di feed ini — mencampur created_at vs scheduled_at dalam satu
//     ORDER BY bikin "terbaru" tak jujur. ListEngagementsFeed (query terpisah)
//     meng-keyset created_at; indeks 00037 melayaninya.
//
//  3. PAGINASI KEYSET LINTAS-TABEL via CURSOR KOMPOSIT (bagian tersulit, beda
//     dari BL-31 yang tak berpaginasi). Tak ada satu keyset yang bisa menjangkau
//     dua tabel; jadi tiap sumber di-keyset sendiri, di-over-fetch pageSize+1,
//     lalu di-merge-sort & dipotong di handler. Cursor halaman = GABUNGAN dua
//     sub-cursor (satu per sumber) yang dikemas jadi satu token yang lolos
//     ui.validTrailToken (`<digit..>_<digit..>`), sehingga mengalir apa adanya
//     lewat mesin jejak BL-7 (pager dua-arah). Lihat encodeDualCursor.
//
// F3 ownership per-sumber (mekanismenya beda, tak bisa berbagi satu filter):
// activities menyaring owner_id (ActivitiesListFilter); engagements menyaring
// via kolom accounts account_owner/assigned_csm/backup_csm (EngagementsListFilter).
// RLS h.q(ctx) mengurung tenant di bawah keduanya. Platform → ScopeAll dua sumber.

// buildUnifiedActivityFeed menjalankan satu halaman feed terpadu: query dua
// sumber (lengan CS gated), merge-sort created_at DESC, potong pageSize, dan
// hitung cursor komposit halaman berikutnya. Mengembalikan baris siap-render +
// nextCursor ("" = halaman terakhir). names dioper agar pemetaan owner activity
// tak query anggota dua kali.
func (h *Handler) buildUnifiedActivityFeed(
	ctx context.Context,
	r *http.Request,
	names map[int64]string,
) ([]panel.ActivityRow, string, error) {
	dc := decodeDualCursor(r.URL.Query().Get("after"))
	uid := session.UserID(ctx)
	platform := isPlatformRole(session.Role(ctx))
	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas F3, tak melebarkan.
	// activities → subject; engagements → subject + village_name.
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// ── Lengan Sales/umum: activities ──────────────────────────────────────
	// Platform tanpa business_role → ScopeAll agar tak nol baris.
	actFilter := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	if platform {
		actFilter = db.ActivitiesListFilter{ScopeAll: true}
	}
	acts, err := h.q(ctx).ListAllActivities(ctx, db.ListAllActivitiesParams{
		CursorCreatedAt: dc.act.at,
		CursorID:        dc.act.id,
		ScopeAll:        actFilter.ScopeAll,
		IsOwn:           actFilter.IsOwn,
		Uid:             &uid,
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		return nil, "", fmt.Errorf("all-activities: list activities: %w", err)
	}

	// ── Lengan CS: engagements (F2-GATED) ──────────────────────────────────
	// Kunci "gated union" (BL-41 #1): tanpa gerbang ini, Sales/Support yang
	// membuka /activity-log akan melihat baris CS. Platform tembus (operator
	// sistem butuh visibilitas penuh), sejajar canViewAllActivities.
	var engs []db.ListEngagementsFeedRow
	if platform || canViewEngagements(ctx) {
		engFilter := db.EngagementsListFilterFor(session.BusinessDataScope(ctx))
		if platform {
			engFilter = db.EngagementsListFilter{ScopeAll: true}
		}
		engs, err = h.q(ctx).ListEngagementsFeed(ctx, db.ListEngagementsFeedParams{
			CursorCreatedAt: dc.eng.at,
			CursorID:        dc.eng.id,
			ScopeAll:        engFilter.ScopeAll,
			IsOwn:           engFilter.IsOwn,
			Uid:             &uid,
			Search:          query,
			PageSize:        pageSize + 1,
		})
		if err != nil {
			return nil, "", fmt.Errorf("all-activities: list engagements feed: %w", err)
		}
	}

	// ── Gabung + urut created_at DESC (id DESC, lalu source sebagai tie-break
	// deterministik lintas-tabel di mana id bisa bertumbukan) ───────────────
	entries := make([]feedEntry, 0, len(acts)+len(engs))
	for _, a := range acts {
		entries = append(entries, feedEntry{
			at: tsTime(a.CreatedAt), id: a.ID, source: "sales",
			row: activityRowView(a, names),
			cur: subCursor{at: a.CreatedAt, id: a.ID},
		})
	}
	for _, e := range engs {
		entries = append(entries, feedEntry{
			at: tsTime(e.CreatedAt), id: e.ID, source: "cs",
			row: engagementFeedRowView(e),
			cur: subCursor{at: e.CreatedAt, id: e.ID},
		})
	}

	items, nextCursor := pageEntries(entries, dc)
	return items, nextCursor, nil
}
