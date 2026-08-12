package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_activities_page.go — HALAMAN baca Sales Activity (4.4): daftar berkeyset &
// detail. Aksi ada di sales_activities.go; form GET & opsi picker di
// sales_activities_new.go. Dipisah karena halaman tumbuh dengan aturan LIHAT (F2
// read + F3 ownership) & aksi dengan aturan TULIS. Meniru sales_deals_page.go.

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

// ActivityDetail — GET /w/{workspace}/activities/{id}. Satu aktivitas. Ownership
// diputuskan di sini (loadOwnedActivity: filter.Allows) — di luar cakupan → 404.
// Nama target & kontak diresolusi best-effort (satu query masing-masing, bukan
// N+1); gagal → label cadangan berbasis id, bukan 500.
func (h *Handler) ActivityDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSalesActivity(ctx) {
		h.renderActivitiesForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadOwnedActivity(w, r, id)
	if !ok {
		return
	}
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("activities: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, a.Subject, "/activities",
		panel.ActivityDetail(h.activityDetailView(ctx, base, a, names)))
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

// activityDetailView merakit detail lengkap. Nama target (& kontak untuk Call)
// diresolusi best-effort (baris di luar tenant/terhapus → label cadangan, tak
// menggagalkan halaman). Field per-kind diisi hanya untuk kind terkait.
func (h *Handler) activityDetailView(ctx context.Context, base string, a db.Activity, names map[int64]string) panel.ActivityDetailView {
	v := panel.ActivityDetailView{
		Base:         base,
		ID:           a.ID,
		Kind:         a.Kind,
		Subject:      a.Subject,
		TargetType:   a.TargetType,
		TargetID:     a.TargetID,
		TargetLabel:  h.targetLabel(ctx, a.TargetType, a.TargetID),
		Owner:        ownerName(a.OwnerID, names),
		Created:      fmtLocal(a.CreatedAt),
		Updated:      fmtLocal(a.UpdatedAt),
		Status:       deref(a.Status),
		TaskStatuses: taskStatusOptions,
		Notes:        deref(a.Notes),
		CanWrite:     canWriteSalesActivity(ctx),
	}
	switch a.Kind {
	case "task":
		v.DueDate = dateStr(a.DueDate)
		v.Priority = deref(a.Priority)
	case "call":
		if a.ContactID != nil {
			v.ContactLabel = h.contactLabel(ctx, *a.ContactID)
		}
		v.Direction = deref(a.Direction)
		v.ActivityAt = fmtLocal(a.ActivityAt)
		v.Duration = int32PtrStr(a.DurationMin)
		v.CallResult = deref(a.CallResult)
	case "note":
		v.Body = deref(a.Body)
	}
	return v
}

// targetLabel meresolusi nama target polimorfik (deal/account/contact) untuk
// tampilan detail. Reuse accountLabel/contactLabel (sales_deals_page.go). Gagal /
// tipe tak dikenal → label cadangan berbasis id (bukan 500).
func (h *Handler) targetLabel(ctx context.Context, targetType string, id int64) string {
	idStr := strconv.FormatInt(id, 10)
	switch targetType {
	case "deal":
		d, err := h.q(ctx).GetDeal(ctx, id)
		if err != nil {
			return "Deal #" + idStr
		}
		return d.DealName
	case "account":
		return h.accountLabel(ctx, id)
	case "contact":
		return h.contactLabel(ctx, id)
	default:
		return targetType + " #" + idStr
	}
}

// int32PtrStr memformat *int32 opsional untuk tampilan/prefill: nil → "".
func int32PtrStr(n *int32) string {
	if n == nil {
		return ""
	}
	return strconv.FormatInt(int64(*n), 10)
}
