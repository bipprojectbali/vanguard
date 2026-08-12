package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_page.go — HALAMAN baca daftar Active Subscriptions (Modul 5).
// GET-only slice (M5-3b): renew/churn/create menyusul di slice berikutnya.
// Meniru sales_deals_page.go (dealsTable): gerbang F2 read → F3 ownership di
// layer query → F4 masking ARR di baris. Keyset (created_at DESC, id DESC) +
// filter status opsional lewat ?status=.

// SubscriptionsList — GET /w/{workspace}/subscriptions. Daftar langganan
// ter-scope kepemilikan (F3) + filter status opsional. Bukan pemegang peran CRM
// (read) → 403 + penjelasan.
func (h *Handler) SubscriptionsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	cursorAt, cursorID := pageCursor(r)
	statusFilter := r.URL.Query().Get("status")
	rows, err := h.q(ctx).ListSubscriptions(ctx, db.ListSubscriptionsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		StatusFilter:    statusFilter,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(s db.ListSubscriptionsRow) (pgtype.Timestamptz, int64) {
		return s.CreatedAt, s.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(s, names, br))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Active Subscriptions", "/subscriptions",
		panel.SubList(panel.SubListView{
			Base:         base,
			StatusFilter: statusFilter,
			Statuses:     subscriptionStatuses,
			Err:          wsErrMsg(r.URL.Query().Get("err")),
			Items:        items,
			NextCursor:   nextCursor,
		}))
}

// renderSubscriptionsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderSubscriptionsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Active Subscriptions", "/subscriptions",
		panel.SalesForbidden("Active Subscriptions"))
}

// subRowView memetakan satu baris daftar → baris tabel + F4 (ARR disamarkan untuk
// non-manager). MRR terlihat semua viewer; ARR hanya admin/manager. Owner
// diresolusi dari peta anggota.
func subRowView(s db.ListSubscriptionsRow, names map[int64]string, businessRole string) panel.SubRow {
	return panel.SubRow{
		ID:         s.ID,
		EntityCode: deref(s.EntityCode),
		Village:    s.VillageName,
		Plan:       s.PlanName,
		Status:     s.Status,
		MRR:        formatRupiah(s.Mrr),
		ARR:        maskSubscriptionARR(formatRupiah(s.Arr), businessRole),
		Start:      dateStr(s.StartDate),
		End:        dateStr(s.EndDate),
		Owner:      ownerName(s.SubscriptionOwner, names),
	}
}
