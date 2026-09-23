package handler

import (
	"context"
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
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
//
// Fungsi per-sumbu sort (kind/subject/owner di sort_a.go, status/date di
// sort_b.go) dipisah krn ambang File Health yang sama.

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

	var entries []feedEntry
	var err error
	switch axis {
	case "kind":
		entries, err = h.unifiedFeedByKind(ctx, dc, uid, query, useDir, actFilter, engFilter, engGate, names)
	case "subject":
		entries, err = h.unifiedFeedBySubject(ctx, dc, uid, query, useDir, actFilter, engFilter, engGate, names)
	case "owner":
		entries, err = h.unifiedFeedByOwner(ctx, dc, uid, query, useDir, actFilter, engFilter, engGate, names)
	case "status":
		entries, err = h.unifiedFeedByStatus(ctx, dc, uid, query, useDir, actFilter, engFilter, engGate, names)
	default: // "date"
		entries, err = h.unifiedFeedByDate(ctx, dc, uid, query, useDir, actFilter, engFilter, engGate, names)
	}
	if err != nil {
		return nil, "", err
	}

	items, nextCursor := pageEntries(entries, dc, useDir)
	return items, nextCursor, nil
}
