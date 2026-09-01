package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_subscriptions.go — Subscription Report (Modul 8 M8-1, wireframe 8.4):
// renewal-forecast (jendela TETAP "due", reuse ListRenewals) + churn (reuse
// ListChurned), dua TAB link bookmarkable (?section=renewal|churn, gotcha
// #16). Row-view REUSE langsung (renewalRowView/churnRowView, package sama —
// subscriptions_renewals.go/subscriptions_churn_page.go) — tak ditulis ulang.
// F2 gate canViewReports; F3 ownership via SubscriptionsListFilterFor (sumber
// SAMA dgn dasbor asal). Export CSV loop SEMUA halaman keyset (rule "no
// silent caps" — export tak boleh diam-diam terpotong ke halaman pertama).

const (
	reportSectionRenewal = "renewal"
	reportSectionChurn   = "churn"
)

// normalizeReportSection memetakan ?section= ke salah satu key sah; nilai
// asing/kosong → reportSectionRenewal (tab default).
func normalizeReportSection(v string) string {
	if v == reportSectionChurn {
		return reportSectionChurn
	}
	return reportSectionRenewal
}

// ReportsSubscriptions — GET /reports/subscriptions?section=renewal|churn.
func (h *Handler) ReportsSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Subscription Report", "/reports/subscriptions")
		return
	}
	section := normalizeReportSection(r.URL.Query().Get("section"))
	view, err := h.reportsSubscriptionsPage(ctx, r, section)
	if err != nil {
		h.Log.Error("reports: subscriptions page", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Subscription Report", "/reports/subscriptions",
		panel.ReportsSubscriptionsBody(view))
}

// reportsSubscriptionsPage mengambil SATU halaman keyset (dipakai HTML —
// export loop semua halaman lewat helper terpisah di bawah).
func (h *Handler) reportsSubscriptionsPage(ctx context.Context, r *http.Request, section string) (panel.ReportsSubscriptionsView, error) {
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	cursorAt, cursorID := pageCursor(r)
	q := h.q(ctx)
	view := panel.ReportsSubscriptionsView{Section: section, After: r.URL.Query().Get("after"), Trail: pageTrail(r)}

	if section == reportSectionChurn {
		rows, err := q.ListChurned(ctx, db.ListChurnedParams{
			CursorCreatedAt: cursorAt, CursorID: cursorID,
			ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
			PageSize: pageSize + 1,
		})
		if err != nil {
			return view, err
		}
		shown, next := splitPage(rows, func(s db.ListChurnedRow) (pgtype.Timestamptz, int64) { return s.CreatedAt, s.ID })
		names, err := h.memberNameMap(ctx)
		if err != nil {
			return view, err
		}
		items := make([]panel.ChurnRow, 0, len(shown))
		for _, s := range shown {
			items = append(items, churnRowView(s, names, br))
		}
		view.ChurnItems, view.NextCursor = items, next
		return view, nil
	}

	now := time.Now().In(appTZ)
	today := reportTodayDate(now)
	rows, err := q.ListRenewals(ctx, db.ListRenewalsParams{
		CursorCreatedAt: cursorAt, CursorID: cursorID,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		WindowFilter: "due", Today: today, PageSize: pageSize + 1,
	})
	if err != nil {
		return view, err
	}
	shown, next := splitPage(rows, func(s db.ListRenewalsRow) (pgtype.Timestamptz, int64) { return s.CreatedAt, s.ID })
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(s, now, br))
	}
	view.RenewalItems, view.NextCursor = items, next
	return view, nil
}

func reportTodayDate(now time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

// ReportsSubscriptionsExport — GET /reports/subscriptions/export?section=...
// Loop SEMUA halaman keyset (bukan cuma page pertama) → CSV lengkap.
func (h *Handler) ReportsSubscriptionsExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Subscription Report", "/reports/subscriptions")
		return
	}
	section := normalizeReportSection(r.URL.Query().Get("section"))
	header, rows, err := h.reportsSubscriptionsExportRows(ctx, section)
	if err != nil {
		h.Log.Error("reports: subscriptions export", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := writeCSV(w, "subscription-report-"+section, header, rows); err != nil {
		h.Log.Error("reports: subscriptions export write", "err", err)
	}
}
