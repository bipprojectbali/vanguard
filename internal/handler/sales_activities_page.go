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
//
// Fungsi sort per-kolom mode normal (kind/subject/owner di sort_a.go,
// status/date/default di sort_b.go), whitelist kolom sortable,
// renderActivitiesForbidden & activityRowView (sales_activities_view.go)
// dipisah krn ambang File Health yang sama.

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
		var ok bool
		switch sortCol {
		case "kind":
			shown, nextCursor, ok = h.activitiesSortByKind(w, r, ctx, filter, uid, query, dir)
		case "subject":
			shown, nextCursor, ok = h.activitiesSortBySubject(w, r, ctx, filter, uid, query, dir)
		case "owner":
			shown, nextCursor, ok = h.activitiesSortByOwner(w, r, ctx, filter, uid, query, dir, names)
		case "status":
			shown, nextCursor, ok = h.activitiesSortByStatus(w, r, ctx, filter, uid, query, dir)
		case "date":
			shown, nextCursor, ok = h.activitiesSortByDate(w, r, ctx, filter, uid, query, dir)
		default:
			shown, nextCursor, ok = h.activitiesSortDefault(w, r, ctx, filter, uid, query)
		}
		if !ok {
			return
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

// activitySortableColumns, renderActivitiesForbidden, activityRowView — lihat
// sales_activities_view.go.
