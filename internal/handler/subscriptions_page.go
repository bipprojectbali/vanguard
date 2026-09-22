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
// subStatusLifecycleClass) di subscriptions_status.go.

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
	switch sortCol {
	case "village":
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListSubscriptionsSortByVillage(ctx, db.ListSubscriptionsSortByVillageParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: list sort village", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(s db.ListSubscriptionsSortByVillageRow) (string, int64) {
			return s.VillageName, s.ID
		})
		nextCursor = nc
		items = make([]panel.SubRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, subRowView(subListRowFromVillageSort(s), names, canARR, now))
		}
	case "status":
		// Masa Berlaku diurut RAW status (Trial/Active/…, alfabetis) — keputusan
		// user, BUKAN band urgensi derivasi (subDerivedStatus). Tak nullable →
		// reuse pageCursorText/splitPageText apa adanya (pola sama "village").
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListSubscriptionsSortByStatus(ctx, db.ListSubscriptionsSortByStatusParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: list sort status", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(s db.ListSubscriptionsSortByStatusRow) (string, int64) {
			return s.Status, s.ID
		})
		nextCursor = nc
		items = make([]panel.SubRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, subRowView(subListRowFromStatusSort(s), names, canARR, now))
		}
	case "plan":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListSubscriptionsSortByPlan(ctx, db.ListSubscriptionsSortByPlanParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: list sort plan", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByPlanRow) (string, int64, bool) {
			if s.PlanName == nil {
				return "", s.ID, true
			}
			return *s.PlanName, s.ID, false
		})
		nextCursor = nc
		items = make([]panel.SubRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, subRowView(subListRowFromPlanSort(s), names, canARR, now))
		}
	case "mrr":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		// cursor MRR kanonik = teks desimal apa adanya (optNumeric ada di
		// sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
		// cursor_val bertipe numeric asli, jadi teks cursor tak perlu
		// menjaga urutan leksikal, cukup bolak-balik lossless.
		cursorVal, code := optNumeric(cursorRaw, "")
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListSubscriptionsSortByMrr(ctx, db.ListSubscriptionsSortByMrrParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: list sort mrr", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByMrrRow) (string, int64, bool) {
			if !s.Mrr.Valid {
				return "", s.ID, true
			}
			return numericStr(s.Mrr), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.SubRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, subRowView(subListRowFromMrrSort(s), names, canARR, now))
		}
	case "renewal":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		cursorVal, code := optDate(cursorRaw)
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListSubscriptionsSortByRenewal(ctx, db.ListSubscriptionsSortByRenewalParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: list sort renewal", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByRenewalRow) (string, int64, bool) {
			if !s.EndDate.Valid {
				return "", s.ID, true
			}
			return dateStr(s.EndDate), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.SubRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, subRowView(subListRowFromRenewalSort(s), names, canARR, now))
		}
	case "csm":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListSubscriptionsSortByCsm(ctx, db.ListSubscriptionsSortByCsmParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: list sort csm", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Kunci cursor CSM = nama/email resolusi peta anggota (ownerName), PERSIS
		// yang ditampilkan — bukan owner mentah. NULL-nya mengikuti
		// subscription_owner asli (bukan string kosong hasil ownerName), sama
		// dengan kondisi NULL di kunci sort SQL (LEFT JOIN users).
		shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByCsmRow) (string, int64, bool) {
			if s.SubscriptionOwner == nil {
				return "", s.ID, true
			}
			return ownerName(s.SubscriptionOwner, names), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.SubRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, subRowView(subListRowFromCsmSort(s), names, canARR, now))
		}
	default:
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
		shown, nc := splitPage(rows, func(s db.ListSubscriptionsRow) (pgtype.Timestamptz, int64) {
			return s.CreatedAt, s.ID
		})
		nextCursor = nc
		items = make([]panel.SubRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, subRowView(subListRowFromDefault(s), names, canARR, now))
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

// renderSubscriptionsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderSubscriptionsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Subscription Lists", "/subscriptions",
		panel.SalesForbidden("Subscription Lists"))
}

// subSortableColumns = whitelist kolom yang boleh diminta lewat ?sort= (BL-157a
// + lanjutan: seluruh 6 kolom tabel Subscription Lists). ?sort= di luar
// daftar ini diperlakukan seolah absen (jatuh ke default created_at DESC),
// TAK error.
var subSortableColumns = map[string]bool{
	"village": true,
	"plan":    true,
	"mrr":     true,
	"status":  true,
	"renewal": true,
	"csm":     true,
}

// subListRow = field YANG DIPAKAI subRowView, diekstrak dari DUA struct sqlc
// berbeda (db.ListSubscriptionsRow & db.ListSubscriptionsSortByVillageRow —
// satu query = satu struct meski SELECT sama persis) agar logika mapping
// (derivasi status, mask MRR F4, resolusi CSM) TAK diduplikasi per query.
type subListRow struct {
	ID                int64
	VillageName       string
	PlanName          *string
	ItemCount         int64
	Status            string
	EndDate           pgtype.Date
	Mrr               pgtype.Numeric
	SubscriptionOwner *int64
}

func subListRowFromDefault(s db.ListSubscriptionsRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromVillageSort(s db.ListSubscriptionsSortByVillageRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromStatusSort(s db.ListSubscriptionsSortByStatusRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromPlanSort(s db.ListSubscriptionsSortByPlanRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromMrrSort(s db.ListSubscriptionsSortByMrrRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromRenewalSort(s db.ListSubscriptionsSortByRenewalRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

func subListRowFromCsmSort(s db.ListSubscriptionsSortByCsmRow) subListRow {
	return subListRow{
		ID: s.ID, VillageName: s.VillageName, PlanName: s.PlanName, ItemCount: s.ItemCount,
		Status: s.Status, EndDate: s.EndDate, Mrr: s.Mrr, SubscriptionOwner: s.SubscriptionOwner,
	}
}

// subRowView memetakan satu baris daftar → baris tabel ramping (BL-95: 6 kolom
// Desa · Paket · MRR · Status · Renewal Date · CSM; ARR & Mulai dibuang). MRR:
// kebijakan umum maskARR (skema.md §9 — SEMUA role kecuali Support; diperbaiki
// audit FLS M9-1). Status = DERIVASI renewal (subDerivedStatus) HANYA untuk
// langganan Active; status daur hidup lain (Trial/Cancelled/…) tampil apa adanya
// (badge lifecycle) — derivasi berbasis end_date tak bermakna untuk status
// terminal. CSM (owner) diresolusi dari peta anggota. canARR dihitung SEKALI
// oleh pemanggil (canSeeARR(ctx)).
func subRowView(s subListRow, names map[int64]string, canARR bool, now time.Time) panel.SubRow {
	label, cls := subDerivedStatus(s.Status, s.EndDate, now)
	return panel.SubRow{
		ID:          s.ID,
		Village:     s.VillageName,
		Plan:        subPlanDisplay(s.PlanName, s.ItemCount),
		Status:      label,
		StatusClass: cls,
		MRR:         maskARR(formatRupiah(s.Mrr), canARR),
		Renewal:     dateStr(s.EndDate),
		CSM:         ownerName(s.SubscriptionOwner, names),
	}
}

// subPlanDisplay, subDerivedStatus, subStatusLifecycleClass di
// subscriptions_status.go.
