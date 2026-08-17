package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// all_activities_page.go — HALAMAN daftar Activities lintas-context (M7).
// Berbeda dari sales_activities_page.go (context='sales' saja), halaman ini
// menampilkan SEMUA aktivitas (sales+cs+general) — cermin menu "Activities"
// top-level di sidebar (di luar grup Sales).
//
// Gate: reuse canViewSalesActivity (crm:sales_activity read). Siapa pun yang
// boleh lihat Sales Activities juga boleh lihat tampilan lintas-context ini.
// TODO(activities): ganti ke crm:activities read saat permission pass M9 dijalankan.

// AllActivitiesList — GET /w/{workspace}/activity-log. Daftar semua aktivitas
// (lintas-context) berkeyset + F3 ownership.
func (h *Handler) AllActivitiesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSalesActivity(ctx) {
		h.renderAllActivitiesForbidden(w, r)
		return
	}

	filter := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	cursorAt, cursorID := pageCursor(r)

	rows, err := h.q(ctx).ListAllActivities(ctx, db.ListAllActivitiesParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("all-activities: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("all-activities: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.ActivityRow, 0, len(shown))
	for _, a := range shown {
		items = append(items, activityRowView(a, names))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Aktivitas", "/activity-log",
		panel.AllActivitiesList(panel.AllActivitiesListView{
			Base:       base,
			CanWrite:   canWriteSalesActivity(ctx),
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Msg:        activitiesMsg(r.URL.Query().Get("ok")),
			Items:      items,
			NextCursor: nextCursor,
		}))
}

// renderAllActivitiesForbidden — 403 + penjelasan bagi anggota tanpa izin.
func (h *Handler) renderAllActivitiesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Aktivitas", "/activity-log",
		panel.SalesForbidden("Aktivitas"))
}
