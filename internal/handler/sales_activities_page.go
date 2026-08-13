package handler

import (
	"net/http"

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

// ActivitiesList — GET /w/{workspace}/activities. Daftar aktivitas Sales berkeyset
// (F3 owner_id). Bukan pemegang izin lihat → 403 + penjelasan.
func (h *Handler) ActivitiesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}
	filter := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListActivities(ctx, db.ListActivitiesParams{
		ContextFilter:   activityContextSales,
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("activities: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("activities: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.ActivityRow, 0, len(shown))
	for _, a := range shown {
		items = append(items, activityRowView(a, names))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Sales Activities", "/activities",
		panel.ActivitiesList(panel.ActivitiesListView{
			Base:       base,
			CanWrite:   canWriteSalesActivity(ctx),
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Msg:        activitiesMsg(r.URL.Query().Get("ok")),
			Items:      items,
			NextCursor: nextCursor,
		}))
}

// renderActivitiesForbidden — 403 + penjelasan bagi anggota tanpa izin
// crm:sales_activity.
func (h *Handler) renderActivitiesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Sales Activities", "/activities",
		panel.SalesForbidden("Sales Activities"))
}

// activityRowView memetakan satu aktivitas → baris tabel. Target = tipe+id (tanpa
// lookup nama → hindari N+1, rule 13). Status hanya untuk Task ("" untuk call/note
// → badge "—"). Tanggal = created_at lokal (gotcha #14: simpan UTC, tampil lokal).
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
	}
}
