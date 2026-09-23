package handler

import (
	"net/http"
	"strings"
	"time"

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
//
// Derivasi kolom Paket & Masa Berlaku/Status (subPlanDisplay, subDerivedStatus,
// subStatusLifecycleClass) di subscriptions_status.go. Row mapping
// (subListRow + subListRowFromXxx + subRowView) di subscriptions_row.go;
// forbidden render & subSortableColumns di subscriptions_meta.go; fungsi sort
// per-kolom di subscriptions_sort_a.go & subscriptions_sort_b.go (dipecah
// krn ambang File Health).

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
	canARR := canSeeARR(ctx)

	// now/today di zona waktu app: now menurunkan Status DERIVASI per-baris
	// (BL-95, ambang SAMA dgn Renewals BL-94); today (tanggal sipil UTC) menyaring
	// KPI (MRR baru bln ini, churn 30 hari) secara deterministik utk test.
	now := time.Now().In(appTZ)
	today := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

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

	// sort/dir (BL-157a + lanjutan): whitelist 6 kolom sortable (subSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !subSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var items []panel.SubRow
	var nextCursor string
	var ok bool
	switch sortCol {
	case "village":
		items, nextCursor, ok = h.subsSortByVillage(w, r, ctx, filter, uid, statusFilter, query, dir, names, canARR, now)
		if !ok {
			return
		}
	case "status":
		items, nextCursor, ok = h.subsSortByStatus(w, r, ctx, filter, uid, statusFilter, query, dir, names, canARR, now)
		if !ok {
			return
		}
	case "plan":
		items, nextCursor, ok = h.subsSortByPlan(w, r, ctx, filter, uid, statusFilter, query, dir, names, canARR, now)
		if !ok {
			return
		}
	case "mrr":
		items, nextCursor, ok = h.subsSortByMrr(w, r, ctx, filter, uid, statusFilter, query, dir, names, canARR, now)
		if !ok {
			return
		}
	case "renewal":
		items, nextCursor, ok = h.subsSortByRenewal(w, r, ctx, filter, uid, statusFilter, query, dir, names, canARR, now)
		if !ok {
			return
		}
	case "csm":
		items, nextCursor, ok = h.subsSortByCsm(w, r, ctx, filter, uid, statusFilter, query, dir, names, canARR, now)
		if !ok {
			return
		}
	default:
		items, nextCursor, ok = h.subsSortDefault(w, r, ctx, filter, uid, statusFilter, query, cursorAt, cursorID, names, canARR, now)
		if !ok {
			return
		}
	}

	// KPI header (BL-95): satu query agregat di-scope ownership sama. Gagal →
	// tetap render tabel (fail-soft; KPI bukan data kritis untuk baca daftar).
	kpi, kpiErr := h.subscriptionListKPI(ctx, filter, uid, today)
	if kpiErr != nil {
		h.Log.Error("subscriptions: list kpi", "err", kpiErr)
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Subscription Lists", "/subscriptions",
		panel.SubList(panel.SubListView{
			Base:         base,
			StatusFilter: statusSel, // penanda pilihan (Active/…/all) utk tab & tautan
			Statuses:     subscriptionStatuses,
			Query:        query,
			KPIs:         kpi,
			Err:          wsErrMsg(r.URL.Query().Get("err")),
			Items:        items,
			NextCursor:   nextCursor,
			After:        r.URL.Query().Get("after"),
			Trail:        pageTrail(r),
			Sort:         sortCol,
			Dir:          dir,
		}))
}
