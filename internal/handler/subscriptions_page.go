package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_page.go — HALAMAN baca daftar Subscription Lists (Modul 5).
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
	// Default tab = Active (BL-20): menu bernama "Subscription Lists" tetap
	// mendarat pada langganan Active, bukan SEMUA status. Bedakan tak-ada-param
	// (landing murni → default Active) dari pilih-Semua eksplisit (?status=all →
	// tak menyaring). Tanpa pembedaan ini, memetakan "" ke Active membuat tab
	// "Semua" tak terjangkau.
	statusSel := r.URL.Query().Get("status")
	if !r.URL.Query().Has("status") {
		statusSel = "Active"
	}
	statusFilter := statusSel // yang disaring di query
	if statusSel == panel.SubStatusAll {
		statusFilter = "" // "Semua" eksplisit → tak menyaring status
	}
	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas F3+status, tak melebarkan.
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, err := h.q(ctx).ListSubscriptions(ctx, db.ListSubscriptionsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		StatusFilter:    statusFilter,
		Search:          query,
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
	h.renderWorkspaceShell(w, r, "Subscription Lists", "/subscriptions",
		panel.SubList(panel.SubListView{
			Base:         base,
			StatusFilter: statusSel, // penanda pilihan (Active/…/all) utk tab & tautan
			Statuses:     subscriptionStatuses,
			Query:        query,
			Err:          wsErrMsg(r.URL.Query().Get("err")),
			Items:        items,
			NextCursor:   nextCursor,
		}))
}

// renderSubscriptionsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderSubscriptionsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Subscription Lists", "/subscriptions",
		panel.SalesForbidden("Subscription Lists"))
}

// subRowView memetakan satu baris daftar → baris tabel + F4. ARR: hanya
// admin/manager (maskSubscriptionARR, spec M5-4). MRR: kebijakan umum
// canSeeARR/maskARR (skema.md §9, "MRR/ARR/amount disembunyikan dari Support")
// — SEMUA role kecuali Support, beda dari ARR yang juga mengecualikan
// sales/csm. Diperbaiki audit FLS M9-1 (sebelumnya MRR sengaja tanpa masking,
// kontra skema.md §9). Owner diresolusi dari peta anggota.
func subRowView(s db.ListSubscriptionsRow, names map[int64]string, businessRole string) panel.SubRow {
	return panel.SubRow{
		ID:         s.ID,
		EntityCode: deref(s.EntityCode),
		Village:    s.VillageName,
		Plan:       s.PlanName,
		Status:     s.Status,
		MRR:        maskARR(formatRupiah(s.Mrr), businessRole),
		ARR:        maskSubscriptionARR(formatRupiah(s.Arr), businessRole),
		Start:      dateStr(s.StartDate),
		End:        dateStr(s.EndDate),
		Owner:      ownerName(s.SubscriptionOwner, names),
	}
}
