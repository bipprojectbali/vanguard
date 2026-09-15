package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_activities_page.go — HALAMAN daftar Sales Activity (4.4): keyset list.
// Detail satu aktivitas ada di sales_activities_detail.go. Aksi ada di
// sales_activities.go dkk; form GET & opsi picker di sales_activities_new.go.
// Dipisah karena daftar & detail tumbuh dengan aturannya sendiri. Meniru
// sales_deals_page.go.

// ActivitiesList — GET /w/{workspace}/activities[?target=type:id]. Daftar aktivitas
// Sales berkeyset (F3 owner_id). Jika ?target= valid, filter ke entitas itu saja
// (pakai ListActivitiesByTarget, tanpa F3 — konsisten dengan timeline di detail).
// Bukan pemegang izin lihat → 403 + penjelasan.
func (h *Handler) ActivitiesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	// q = pencarian bebas (BL-6): MEMPERSEMPIT subject di atas F3/target, tak melebar.
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// Cek ?target=type:id — dari tautan "Lihat semua" di kartu timeline entitas.
	rawTarget := r.URL.Query().Get("target")
	targetType, targetID, hasTarget := parseActivityTarget(rawTarget)

	// sort/dir (BL-157j): whitelist 4 kolom sortable (activitySortableColumns).
	// Berlaku HANYA mode normal — ListActivitiesByTarget tak punya varian sort.
	// Kombinasi tak dikenal → jatuh ke default (created_at DESC), TAK error.
	sortCol := r.URL.Query().Get("sort")
	if !activitySortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("activities: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var shown []db.Activity
	var nextCursor string
	var targetLabel string

	if hasTarget {
		// Mode filter per-entitas: tampilkan semua aktivitas target ini lintas-context.
		cursorAt, cursorID := pageCursor(r)
		rows, err := h.q(ctx).ListActivitiesByTarget(ctx, db.ListActivitiesByTargetParams{
			TargetType:      targetType,
			TargetID:        targetID,
			CursorCreatedAt: cursorAt,
			CursorID:        cursorID,
			Search:          query,
			PageSize:        pageSize + 1,
		})
		if err != nil {
			h.Log.Error("activities: list by target", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPage(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
			return a.CreatedAt, a.ID
		})
		// Label best-effort: gagal lookup → nama fallback "Tipe #id".
		targetLabel = h.targetLabel(ctx, targetType, targetID)
		sortCol = ""
	} else {
		// Mode normal: daftar Sales Activities ber-F3 ownership.
		filter := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
		uid := session.UserID(ctx)
		switch sortCol {
		case "kind":
			cursorVal, cursorSortID, hasCursor := pageCursorText(r)
			rows, err := h.q(ctx).ListActivitiesSortByKind(ctx, db.ListActivitiesSortByKindParams{
				ContextFilter: activityContextSales,
				HasCursor:     hasCursor,
				Dir:           dir,
				CursorVal:     cursorVal,
				CursorID:      cursorSortID,
				ScopeAll:      filter.ScopeAll,
				IsOwn:         filter.IsOwn,
				Uid:           &uid,
				Search:        query,
				PageSize:      pageSize + 1,
			})
			if err != nil {
				h.Log.Error("activities: list sort kind", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			shown, nextCursor = splitPageText(rows, func(a db.Activity) (string, int64) {
				return a.Kind, a.ID
			})
		case "subject":
			cursorVal, cursorSortID, hasCursor := pageCursorText(r)
			rows, err := h.q(ctx).ListActivitiesSortBySubject(ctx, db.ListActivitiesSortBySubjectParams{
				ContextFilter: activityContextSales,
				HasCursor:     hasCursor,
				Dir:           dir,
				CursorVal:     cursorVal,
				CursorID:      cursorSortID,
				ScopeAll:      filter.ScopeAll,
				IsOwn:         filter.IsOwn,
				Uid:           &uid,
				Search:        query,
				PageSize:      pageSize + 1,
			})
			if err != nil {
				h.Log.Error("activities: list sort subject", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			shown, nextCursor = splitPageText(rows, func(a db.Activity) (string, int64) {
				return a.Subject, a.ID
			})
		case "owner":
			cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
			rows, err := h.q(ctx).ListActivitiesSortByOwner(ctx, db.ListActivitiesSortByOwnerParams{
				ContextFilter: activityContextSales,
				HasCursor:     hasCursor,
				Dir:           dir,
				CursorIsNull:  isNull,
				CursorVal:     cursorVal,
				CursorID:      cursorSortID,
				ScopeAll:      filter.ScopeAll,
				IsOwn:         filter.IsOwn,
				Uid:           &uid,
				Search:        query,
				PageSize:      pageSize + 1,
			})
			if err != nil {
				h.Log.Error("activities: list sort owner", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			// Kunci cursor Pemilik = nama/email resolusi peta anggota (ownerName),
			// PERSIS yang ditampilkan — bukan owner_id mentah. NULL mengikuti
			// owner_id asli (bukan string kosong hasil ownerName), sama dengan
			// kondisi NULL di kunci sort SQL (LEFT JOIN users).
			shown, nextCursor = splitPageTextNullable(rows, func(a db.Activity) (string, int64, bool) {
				if a.OwnerID == nil {
					return "", a.ID, true
				}
				return ownerName(a.OwnerID, names), a.ID, false
			})
		case "status":
			cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
			rows, err := h.q(ctx).ListActivitiesSortByStatus(ctx, db.ListActivitiesSortByStatusParams{
				ContextFilter: activityContextSales,
				HasCursor:     hasCursor,
				Dir:           dir,
				CursorIsNull:  isNull,
				CursorVal:     cursorVal,
				CursorID:      cursorSortID,
				ScopeAll:      filter.ScopeAll,
				IsOwn:         filter.IsOwn,
				Uid:           &uid,
				Search:        query,
				PageSize:      pageSize + 1,
			})
			if err != nil {
				h.Log.Error("activities: list sort status", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			shown, nextCursor = splitPageTextNullable(rows, func(a db.Activity) (string, int64, bool) {
				if a.Status == nil {
					return "", a.ID, true
				}
				return *a.Status, a.ID, false
			})
		case "date":
			cursorAt, cursorSortID, hasCursor := pageCursorTimestamp(r)
			rows, err := h.q(ctx).ListActivitiesSortByDate(ctx, db.ListActivitiesSortByDateParams{
				ContextFilter: activityContextSales,
				HasCursor:     hasCursor,
				Dir:           dir,
				CursorVal:     cursorAt,
				CursorID:      cursorSortID,
				ScopeAll:      filter.ScopeAll,
				IsOwn:         filter.IsOwn,
				Uid:           &uid,
				Search:        query,
				PageSize:      pageSize + 1,
			})
			if err != nil {
				h.Log.Error("activities: list sort date", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			shown, nextCursor = splitPageTimestamp(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
				return a.CreatedAt, a.ID
			})
		default:
			cursorAt, cursorID := pageCursor(r)
			rows, err := h.q(ctx).ListActivities(ctx, db.ListActivitiesParams{
				ContextFilter:   activityContextSales,
				CursorCreatedAt: cursorAt,
				CursorID:        cursorID,
				ScopeAll:        filter.ScopeAll,
				IsOwn:           filter.IsOwn,
				Uid:             &uid,
				Search:          query,
				PageSize:        pageSize + 1,
			})
			if err != nil {
				h.Log.Error("activities: list", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			shown, nextCursor = splitPage(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
				return a.CreatedAt, a.ID
			})
		}
	}

	items := make([]panel.ActivityRow, 0, len(shown))
	for _, a := range shown {
		items = append(items, activityRowView(a, names))
	}

	// targetFilter dikirim ke view apa adanya (string "type:id") untuk propagasi URL.
	targetFilter := ""
	if hasTarget {
		targetFilter = rawTarget
	}
	title := "Sales Activities"
	if targetLabel != "" {
		title = "Aktivitas — " + targetLabel
	}
	h.renderWorkspaceShell(w, r, title, "/activities",
		panel.ActivitiesList(panel.ActivitiesListView{
			Base:         base,
			CanWrite:     canWriteSalesActivity(ctx),
			Err:          wsErrMsg(r.URL.Query().Get("err")),
			Msg:          activitiesMsg(r.URL.Query().Get("ok")),
			Items:        items,
			NextCursor:   nextCursor,
			After:        r.URL.Query().Get("after"),
			Trail:        pageTrail(r),
			Query:        query,
			TargetFilter: targetFilter,
			TargetLabel:  targetLabel,
			Sort:         sortCol,
			Dir:          dir,
		}))
}

// activitySortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157j: 5 kolom tabel Sales Activities). ?sort= di luar daftar ini
// diperlakukan seolah absen (jatuh ke default created_at DESC), TAK error.
// Target sengaja tak masuk: komposit (tipe+id), bukan skalar tunggal.
// "date" (created_at) SUDAH jadi sumbu default (DESC) tanpa ?sort= —
// dimasukkan whitelist toh supaya user bisa FLIP ke ASC & mengarahkan panah
// aktif eksplisit di header, lewat ListActivitiesSortByDate (arah dinamis).
var activitySortableColumns = map[string]bool{
	"kind":    true,
	"subject": true,
	"owner":   true,
	"status":  true,
	"date":    true,
}

// renderActivitiesForbidden — 403 + penjelasan bagi anggota tanpa izin
// crm:sales_activity. currentPath via activityCurrentPathFromQuery (BL-161): jalur
// ini juga dicapai dari ActivityDetail (klik baris /activity-log tanpa akses
// Sales Activities, mis. CS) — tanpa penanda "?from=log", sidebar akan menyala
// item "Sales Activities" yang Disabled untuk peran itu.
func (h *Handler) renderActivitiesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Sales Activities", activityCurrentPathFromQuery(r),
		panel.SalesForbidden("Sales Activities"))
}

// activityRowView memetakan satu aktivitas → baris tabel. Target = tipe+id (tanpa
// lookup nama → hindari N+1, rule 13). Status hanya untuk Task ("" untuk call/note
// → badge "—"). Tanggal = created_at lokal (gotcha #14: simpan UTC, tampil lokal).
// Context diisi agar AllActivitiesList dapat menampilkan kolom Konteks;
// diabaikan di Sales Activities (kolom tak ada di tabel itu).
func activityRowView(a db.Activity, names map[int64]string) panel.ActivityRow {
	return panel.ActivityRow{
		ID:         a.ID,
		Kind:       a.Kind,
		Subject:    a.Subject,
		TargetType: a.TargetType,
		TargetID:   a.TargetID,
		Owner:      ownerName(a.OwnerID, names),
		Status:     deref(a.Status),
		Created:    fmtLocal(a.CreatedAt),
		Context:    deref(a.ActivityContext),
	}
}
