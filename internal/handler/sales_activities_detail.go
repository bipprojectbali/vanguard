package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_detail.go — HALAMAN detail satu Sales Activity + pembantu
// resolusi label. Dipisah dari sales_activities_page.go (daftar) untuk file
// health — detail tumbuh dengan aturannya sendiri (resolusi target/kontak
// per-kind, best-effort).

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
// tampilan detail. Reuse accountLabel/contactLabel (sales_deals_detail.go). Gagal
// / tipe tak dikenal → label cadangan berbasis id (bukan 500).
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
