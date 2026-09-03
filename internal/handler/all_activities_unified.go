package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
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

// subCursor = posisi keyset (created_at, id) satu sumber. Salinan bentuk yang
// dipakai pageCursor, tapi eksplisit agar cursor komposit merakit dua darinya.
type subCursor struct {
	at pgtype.Timestamptz
	id int64
}

// dualCursor = pasangan sub-cursor (activities + engagements) untuk satu posisi
// halaman feed terpadu. Tiap sumber maju independen: sumber yang tak menyumbang
// baris ke halaman ini biarkan sub-cursornya tak berubah (halaman berikut mulai
// dari titik yang sama untuk sumber itu).
type dualCursor struct {
	act subCursor
	eng subCursor
}

// firstSubCursor = sub-cursor halaman pertama: (created_at, id) maksimum
// (Infinity, MaxInt64) sehingga semua baris lolos syarat keyset `< cursor`.
// Sejajar firstPageCursor (dev_users.go) tapi dibungkus subCursor.
func firstSubCursor() subCursor {
	at, id := firstPageCursor()
	return subCursor{at: at, id: id}
}

// firstDualCursor = posisi awal feed (kedua sumber di halaman pertama).
func firstDualCursor() dualCursor {
	return dualCursor{act: firstSubCursor(), eng: firstSubCursor()}
}

// subFieldSentinel menandai sub-cursor "di awal" (firstSubCursor) dalam token.
// Panjang 1 → mustahil bentrok dengan pengkodean baris nyata (selalu ≥20 char:
// 19 digit nano + ≥1 digit id).
const subFieldSentinel = "0"

// subFieldNanoWidth = lebar zero-pad UnixNano dalam token. MaxInt64 = 19 digit;
// nano baris nyata (pasca-1970) selalu positif & muat 19 digit, jadi lebar tetap
// membuat pemisahan nano|id deterministik (potong di indeks 19).
const subFieldNanoWidth = 19

// encodeSubCursor mengemas satu sub-cursor jadi field digit-murni untuk token.
// firstSubCursor / tak-valid → sentinel "0". Baris nyata → 19-digit-nano + id.
func encodeSubCursor(c subCursor) string {
	if !c.at.Valid || c.at.InfinityModifier != pgtype.Finite {
		return subFieldSentinel
	}
	nano := c.at.Time.UTC().UnixNano()
	if nano < 0 {
		// Mustahil di praktik (created_at selalu pasca-1970); jaga token tetap
		// digit-murni bila terjadi anomali data.
		return subFieldSentinel
	}
	return fmt.Sprintf("%0*d%d", subFieldNanoWidth, nano, c.id)
}

// decodeSubCursor membalik encodeSubCursor. Sentinel/rusak/kependekan → halaman
// pertama sub-sumber (fail-open ke awal, sejalan filosofi pageCursor: masukan
// URL yang bisa disunting tak boleh menggagalkan render).
func decodeSubCursor(field string) subCursor {
	if len(field) < subFieldNanoWidth+1 {
		return firstSubCursor()
	}
	nano, err1 := strconv.ParseInt(field[:subFieldNanoWidth], 10, 64)
	id, err2 := strconv.ParseInt(field[subFieldNanoWidth:], 10, 64)
	if err1 != nil || err2 != nil {
		return firstSubCursor()
	}
	return subCursor{
		at: pgtype.Timestamptz{Time: time.Unix(0, nano).UTC(), Valid: true},
		id: id,
	}
}

// encodeDualCursor merakit token halaman = "<fieldAct>_<fieldEng>". Tepat satu
// underscore, kedua sisi digit-murni → lolos ui.validTrailToken, jadi mengalir
// lewat jejak BL-7 tanpa perlakuan khusus.
func encodeDualCursor(c dualCursor) string {
	return encodeSubCursor(c.act) + cursorSep + encodeSubCursor(c.eng)
}

// decodeDualCursor membaca ?after= jadi dualCursor. Kosong/rusak → halaman
// pertama (kedua sumber dari awal).
func decodeDualCursor(raw string) dualCursor {
	if raw == "" {
		return firstDualCursor()
	}
	actField, engField, ok := strings.Cut(raw, cursorSep)
	if !ok {
		return firstDualCursor()
	}
	return dualCursor{act: decodeSubCursor(actField), eng: decodeSubCursor(engField)}
}

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
	return items, nextCursor, nil
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
