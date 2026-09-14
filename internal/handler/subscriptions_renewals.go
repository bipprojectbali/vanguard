package handler

import (
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals.go — dasbor Renewals (Menu 5.2, READ-ONLY). Menyorot
// langganan yang punya dimensi renewal (end_date terisi) lewat empat JENDELA
// (jatuh tempo / masa tenggang / sudah diperpanjang / semua). Aksi perpanjangan
// SENGAJA tidak di sini — renew/approve tetap di detail langganan (subscriptions_
// renew.go/approve.go); dasbor ini hanya baca (sejalan wireframe 5.2).
//
// Gerbang sama dengan daftar langganan: F2 crm:subscriptions read + F3 ownership
// (subscription_owner) di layer query. Tanpa ARR di sini, tapi Prev→Current
// (previous_value/MRR) tetap nilai komersial — dimasking F4 (maskARR, kebijakan
// umum kecuali Support, skema.md §9) sejak audit FLS M9-1. Keyset created_at DESC
// (reuse pageCursor/splitPage) — pengurutan "paling dekat jatuh tempo" ditunda ke slice KPI.

// renewalWindow = satu tab jendela renewal (key untuk query + label tampilan).
// Sumber SATU untuk tab view & validasi handler (view tak memutuskan enum).
type renewalWindow struct{ Key, Label string }

var renewalWindows = []renewalWindow{
	{"due", "Akan Jatuh Tempo"},
	{"grace", "Masa Tenggang"},
	{"renewed", "Diperpanjang"},
	{"all", "Semua"},
}

const defaultRenewalWindow = "due"

// SubscriptionRenewals — GET /w/{slug}/subscriptions/renewals. Dasbor read-only
// jatuh tempo & status renewal, ter-scope kepemilikan (F3) + filter jendela.
func (h *Handler) SubscriptionRenewals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	window := normalizeRenewalWindow(r.URL.Query().Get("window"))
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	now := time.Now().In(appTZ)
	today := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

	// sort/dir (BL-157g): whitelist 5 kolom sortable (renewalSortableColumns).
	// Status & Sisa Hari SENGAJA TAK sortable — Status derivasi selalu tampil
	// (bukan opsional seperti Subscriptions), tab jendela sudah jadi sumbu
	// kategorik; Sisa Hari duplikat sumbu urut Tgl Perpanjang (sama-sama end_date).
	sortCol := r.URL.Query().Get("sort")
	if !renewalSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	var items []panel.RenewalRow
	var nextCursor string
	switch sortCol {
	case "village":
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListRenewalsSortByVillage(ctx, db.ListRenewalsSortByVillageParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			WindowFilter: window,
			Today:        today,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: renewals sort village", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(s db.ListRenewalsSortByVillageRow) (string, int64) {
			return s.VillageName, s.ID
		})
		nextCursor = nc
		items = make([]panel.RenewalRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, renewalRowView(renewalListRowFromVillageSort(s), now, br))
		}
	case "plan":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListRenewalsSortByPlan(ctx, db.ListRenewalsSortByPlanParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			WindowFilter: window,
			Today:        today,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: renewals sort plan", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListRenewalsSortByPlanRow) (string, int64, bool) {
			if s.PlanName == nil {
				return "", s.ID, true
			}
			return *s.PlanName, s.ID, false
		})
		nextCursor = nc
		items = make([]panel.RenewalRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, renewalRowView(renewalListRowFromPlanSort(s), now, br))
		}
	case "date":
		// end_date DIJAMIN terisi di dasbor ini (ListRenewals: WHERE end_date IS
		// NOT NULL) → pola non-nullable (pageCursorText), tanpa cursor_is_null.
		cursorRaw, cursorSortID, hasCursor := pageCursorText(r)
		cursorVal, code := optDate(cursorRaw)
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListRenewalsSortByDate(ctx, db.ListRenewalsSortByDateParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			WindowFilter: window,
			Today:        today,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: renewals sort date", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(s db.ListRenewalsSortByDateRow) (string, int64) {
			return dateStr(s.EndDate), s.ID
		})
		nextCursor = nc
		items = make([]panel.RenewalRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, renewalRowView(renewalListRowFromDateSort(s), now, br))
		}
	case "type":
		// Kunci cursor = renewalTypeLabel PERSIS logika tampil (COALESCE
		// renewal_type/auto_renew) — TAK PERNAH NULL (auto_renew NOT NULL) →
		// pola non-nullable walau nilainya computed.
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListRenewalsSortByType(ctx, db.ListRenewalsSortByTypeParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			WindowFilter: window,
			Today:        today,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: renewals sort type", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(s db.ListRenewalsSortByTypeRow) (string, int64) {
			return renewalTypeLabel(s.RenewalType, s.AutoRenew), s.ID
		})
		nextCursor = nc
		items = make([]panel.RenewalRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, renewalRowView(renewalListRowFromTypeSort(s), now, br))
		}
	case "mrr":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		cursorVal, code := optNumeric(cursorRaw, "")
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListRenewalsSortByMrr(ctx, db.ListRenewalsSortByMrrParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			WindowFilter: window,
			Today:        today,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: renewals sort mrr", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(s db.ListRenewalsSortByMrrRow) (string, int64, bool) {
			if !s.Mrr.Valid {
				return "", s.ID, true
			}
			return numericStr(s.Mrr), s.ID, false
		})
		nextCursor = nc
		items = make([]panel.RenewalRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, renewalRowView(renewalListRowFromMrrSort(s), now, br))
		}
	default:
		cursorAt, cursorID := pageCursor(r)
		rows, err := h.q(ctx).ListRenewals(ctx, db.ListRenewalsParams{
			CursorCreatedAt: cursorAt,
			CursorID:        cursorID,
			ScopeAll:        filter.ScopeAll,
			IsOwn:           filter.IsOwn,
			Uid:             &uid,
			WindowFilter:    window,
			Today:           today,
			PageSize:        pageSize + 1,
		})
		if err != nil {
			h.Log.Error("subscriptions: renewals", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPage(rows, func(s db.ListRenewalsRow) (pgtype.Timestamptz, int64) {
			return s.CreatedAt, s.ID
		})
		nextCursor = nc
		items = make([]panel.RenewalRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, renewalRowView(renewalListRowFromDefault(s), now, br))
		}
	}
	if items == nil {
		items = []panel.RenewalRow{}
	}

	// KPI header (BL-94): satu query agregat di-scope ownership sama. Gagal →
	// tetap render tabel (fail-soft; KPI bukan data kritis untuk baca status renewal).
	kpi, kpiErr := h.renewalKPI(ctx, filter, uid, today)
	if kpiErr != nil {
		h.Log.Error("subscriptions: renewals kpi", "err", kpiErr)
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Renewals", "/subscriptions/renewals",
		panel.RenewalsList(panel.RenewalsView{
			Base:       base,
			Window:     window,
			KPIs:       kpi,
			Windows:    renewalWindowOptions(),
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Items:      items,
			NextCursor: nextCursor,
			After:      r.URL.Query().Get("after"),
			Trail:      pageTrail(r),
			Sort:       sortCol,
			Dir:        dir,
		}))
}

// renewalSortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157g: 5 dari 7 kolom tabel Renewals). Status & Sisa Hari SENGAJA absen
// (lihat komentar di SubscriptionRenewals). ?sort= di luar daftar ini
// diperlakukan seolah absen (jatuh ke default created_at DESC), TAK error.
var renewalSortableColumns = map[string]bool{
	"village": true,
	"plan":    true,
	"date":    true,
	"type":    true,
	"mrr":     true,
}

// normalizeRenewalWindow memetakan ?window= ke salah satu key sah; nilai asing/
// kosong → defaultRenewalWindow (jendela jatuh tempo).
func normalizeRenewalWindow(v string) string {
	for _, wnd := range renewalWindows {
		if wnd.Key == v {
			return v
		}
	}
	return defaultRenewalWindow
}

// renewalWindowOptions menyalin daftar jendela ke tipe view (handler pemilik enum).
func renewalWindowOptions() []panel.RenewalWindow {
	out := make([]panel.RenewalWindow, 0, len(renewalWindows))
	for _, wnd := range renewalWindows {
		out = append(out, panel.RenewalWindow{Key: wnd.Key, Label: wnd.Label})
	}
	return out
}
