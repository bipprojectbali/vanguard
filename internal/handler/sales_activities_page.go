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

	cursorAt, cursorID := pageCursor(r)
	base := wsPath(slugFromRequest(r), "")
	// q = pencarian bebas (BL-6): MEMPERSEMPIT subject di atas F3/target, tak melebar.
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// Cek ?target=type:id — dari tautan "Lihat semua" di kartu timeline entitas.
	rawTarget := r.URL.Query().Get("target")
	targetType, targetID, hasTarget := parseActivityTarget(rawTarget)

	var rows []db.Activity
	var targetLabel string
	var err error

	if hasTarget {
		// Mode filter per-entitas: tampilkan semua aktivitas target ini lintas-context.
		rows, err = h.q(ctx).ListActivitiesByTarget(ctx, db.ListActivitiesByTargetParams{
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
		// Label best-effort: gagal lookup → nama fallback "Tipe #id".
		targetLabel = h.targetLabel(ctx, targetType, targetID)
	} else {
		// Mode normal: daftar Sales Activities ber-F3 ownership.
		filter := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
		uid := session.UserID(ctx)
		rows, err = h.q(ctx).ListActivities(ctx, db.ListActivitiesParams{
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
			Query:        query,
			TargetFilter: targetFilter,
			TargetLabel:  targetLabel,
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
