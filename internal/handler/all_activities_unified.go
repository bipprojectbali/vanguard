package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// all_activities_unified.go — linimasa TERPADU untuk halaman Activities GLOBAL
// (/activity-log, BL-41). Menaikkan gagasan BL-31 (detail Account) ke feed
// lintas-desa berpaginasi: menggabungkan dua sumber-kebenaran terpisah dalam
// SATU daftar terurut —
//   - activities (jalur Sales/umum, tabel activities)   → ListAllActivitiesSortBy*
//   - engagements (jalur Customer Success, tabel sendiri) → ListEngagementsFeedSortBy*
//
// Nol duplikasi data (OPSI 2 BL-31): masing-masing tabel tetap sumber-kebenarannya;
// feed ini hanya MEMBACA gabungan (baris CS read-only — CRUD-nya di modul CS).
//
// Empat keputusan yang menyatukannya:
//
//  1. GATED UNION (BL-41 #1). Lengan CS hanya dimuat bila aktor boleh melihat
//     modul Engagements (canViewEngagements — crm:engagements) ATAU platform.
//     Sales/Support tak punya crm:engagements → lengan CS tak pernah ikut, jadi
//     tak ada kebocoran baris CS ke halaman /activity-log mereka.
//
//  2. SUMBU URUT PILIHAN USER (BL-157k). Sebelumnya satu-satunya sumbu adalah
//     created_at DESC ("kapan DICATAT"); kini lima sumbu (Jenis/Subjek/Pemilik/
//     Status/Tanggal) — sama seperti Sales Activities (BL-157j) — berlaku
//     JUGA di feed gabungan ini, lewat pasangan query SortBy* pada KEDUA
//     lengan. engagements.scheduled_at (bisa jauh ke depan) TETAP tak dipakai
//     sebagai sumbu Tanggal — mencampur created_at vs scheduled_at dalam satu
//     ORDER BY bikin "terbaru" tak jujur (keputusan asli BL-41 #3, tak berubah).
//     Tanpa ?sort= eksplisit, feed jatuh ke sumbu Tanggal arah desc — SATU
//     mekanisme (ListAllActivitiesSortByDate/ListEngagementsFeedSortByDate),
//     bukan jalur ListAllActivities/ListEngagementsFeed polos yang terpisah.
//
//  3. PAGINASI KEYSET LINTAS-TABEL via CURSOR KOMPOSIT (bagian tersulit, beda
//     dari BL-31 yang tak berpaginasi). Tak ada satu keyset yang bisa menjangkau
//     dua tabel; jadi tiap sumber di-keyset sendiri, di-over-fetch pageSize+1,
//     lalu di-merge-sort & dipotong di handler. Cursor halaman = GABUNGAN dua
//     sub-cursor (satu per sumber) yang dikemas jadi satu token yang lolos
//     ui.validTrailToken (`<hexdigits>_<decimaldigits>`), sehingga mengalir apa
//     adanya lewat mesin jejak BL-7 (pager dua-arah). Sejak BL-157k sub-cursor
//     membawa NILAI SUMBU AKTIF (bukan cuma created_at) — lihat
//     all_activities_cursor.go (genSubCursor/dualCursorGen).
//
//  4. MERGE-SORT DI GO, BUKAN SQL (keterbatasan collation). Tiap sisi sudah
//     terurut benar oleh SQL-nya sendiri; hanya INTERLEAVE lintas-sisi yang
//     dilakukan Go (lessEntry, all_activities_feed.go) via perbandingan string
//     byte-wise. Postgres bisa pakai collation locale-aware; Go selalu
//     byte-wise (≈ collation "C") — pada karakter tak-biasa (aksen, dsb.)
//     interleaving lintas-sumber SECARA TEORITIS bisa berbeda urutan dari satu
//     query SQL gabungan hipotetis. Tak ada modul lain yang perlu memecahkan
//     ini (tak ada yang merge-sort dua stream ter-SQL-urut di sisi klien) —
//     dicatat, bukan diperbaiki, di luar cakupan BL-157k.
//
// F3 ownership per-sumber (mekanismenya beda, tak bisa berbagi satu filter):
// activities menyaring owner_id (ActivitiesListFilterFor); engagements menyaring
// via kolom accounts account_owner/assigned_csm/backup_csm (EngagementsListFilterFor).
// RLS h.q(ctx) mengurung tenant di bawah keduanya. Platform → ScopeAll dua sumber.

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

// buildUnifiedActivityFeed menjalankan satu halaman feed terpadu pada sumbu
// sortCol/dir (BL-157k): query dua sumber lewat pasangan SortBy* yang cocok
// (lengan CS gated), merge-sort pada sumbu itu, potong pageSize, dan hitung
// cursor komposit halaman berikutnya. sortCol kosong/tak dikenal → sumbu
// Tanggal arah desc (default lama, kini lewat mekanisme SortBy* yang sama).
// Mengembalikan baris siap-render + nextCursor ("" = halaman terakhir). names
// dioper agar pemetaan owner activity tak query anggota dua kali.
func (h *Handler) buildUnifiedActivityFeed(
	ctx context.Context,
	r *http.Request,
	names map[int64]string,
	sortCol, dir string,
) ([]panel.ActivityRow, string, error) {
	axis, useDir := sortCol, dir
	if axis == "" {
		axis, useDir = "date", "desc"
	}

	dc := decodeDualCursorGen(r.URL.Query().Get("after"))
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

	// ── Lengan CS: engagements (F2-GATED) ──────────────────────────────────
	// Kunci "gated union" (BL-41 #1): tanpa gerbang ini, Sales/Support yang
	// membuka /activity-log akan melihat baris CS. Platform tembus (operator
	// sistem butuh visibilitas penuh), sejajar canViewAllActivities.
	engGate := platform || canViewEngagements(ctx)
	engFilter := db.EngagementsListFilter{}
	if engGate {
		engFilter = db.EngagementsListFilterFor(session.BusinessDataScope(ctx))
		if platform {
			engFilter = db.EngagementsListFilter{ScopeAll: true}
		}
	}

	entries := make([]feedEntry, 0, pageSize*2)

	switch axis {
	case "kind":
		actRows, err := h.q(ctx).ListAllActivitiesSortByKind(ctx, db.ListAllActivitiesSortByKindParams{
			HasCursor: dc.act.hasCursor, Dir: useDir, CursorVal: dc.act.val, CursorID: dc.act.id,
			ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, "", fmt.Errorf("all-activities: list activities sort kind: %w", err)
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
				return nil, "", fmt.Errorf("all-activities: list engagements sort type: %w", err)
			}
			for _, e := range engRows {
				entries = append(entries, feedEntry{
					id: e.ID, source: "cs",
					row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
					sortVal: e.EngagementType,
				})
			}
		}

	case "subject":
		actRows, err := h.q(ctx).ListAllActivitiesSortBySubject(ctx, db.ListAllActivitiesSortBySubjectParams{
			HasCursor: dc.act.hasCursor, Dir: useDir, CursorVal: dc.act.val, CursorID: dc.act.id,
			ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, "", fmt.Errorf("all-activities: list activities sort subject: %w", err)
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
				return nil, "", fmt.Errorf("all-activities: list engagements sort subject: %w", err)
			}
			for _, e := range engRows {
				entries = append(entries, feedEntry{
					id: e.ID, source: "cs",
					row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
					sortVal: e.Subject,
				})
			}
		}

	case "owner":
		actRows, err := h.q(ctx).ListAllActivitiesSortByOwner(ctx, db.ListAllActivitiesSortByOwnerParams{
			HasCursor: dc.act.hasCursor, Dir: useDir, CursorIsNull: dc.act.isNull, CursorVal: dc.act.val, CursorID: dc.act.id,
			ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, "", fmt.Errorf("all-activities: list activities sort owner: %w", err)
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
				return nil, "", fmt.Errorf("all-activities: list engagements sort owner: %w", err)
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

	case "status":
		actRows, err := h.q(ctx).ListAllActivitiesSortByStatus(ctx, db.ListAllActivitiesSortByStatusParams{
			HasCursor: dc.act.hasCursor, Dir: useDir, CursorIsNull: dc.act.isNull, CursorVal: dc.act.val, CursorID: dc.act.id,
			ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, "", fmt.Errorf("all-activities: list activities sort status: %w", err)
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
				return nil, "", fmt.Errorf("all-activities: list engagements sort status: %w", err)
			}
			for _, e := range engRows {
				entries = append(entries, feedEntry{
					id: e.ID, source: "cs",
					row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
					sortVal: e.Status,
				})
			}
		}

	default: // "date"
		actRows, err := h.q(ctx).ListAllActivitiesSortByDate(ctx, db.ListAllActivitiesSortByDateParams{
			HasCursor: dc.act.hasCursor, Dir: useDir, CursorVal: subCursorAsTimestamptz(dc.act), CursorID: dc.act.id,
			ScopeAll: actFilter.ScopeAll, IsOwn: actFilter.IsOwn, Uid: &uid, Search: query, PageSize: pageSize + 1,
		})
		if err != nil {
			return nil, "", fmt.Errorf("all-activities: list activities sort date: %w", err)
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
				return nil, "", fmt.Errorf("all-activities: list engagements sort date: %w", err)
			}
			for _, e := range engRows {
				entries = append(entries, feedEntry{
					id: e.ID, source: "cs",
					row:     engagementFeedRowCore(e.ID, e.AccountID, e.Subject, e.EngagementType, e.Status, e.CreatedAt, e.OwnerName),
					sortVal: subCursorTimeVal(tsTime(e.CreatedAt).UnixNano()),
				})
			}
		}
	}

	items, nextCursor := pageEntries(entries, dc, useDir)
	return items, nextCursor, nil
}
