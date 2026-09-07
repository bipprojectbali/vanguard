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
		items = append(items, subRowView(s, names, br, now))
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
		}))
}

// renderSubscriptionsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderSubscriptionsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Subscription Lists", "/subscriptions",
		panel.SalesForbidden("Subscription Lists"))
}

// subRowView memetakan satu baris daftar → baris tabel ramping (BL-95: 6 kolom
// Desa · Paket · MRR · Status · Renewal Date · CSM; ARR & Mulai dibuang). MRR:
// kebijakan umum maskARR (skema.md §9 — SEMUA role kecuali Support; diperbaiki
// audit FLS M9-1). Status = DERIVASI renewal (subDerivedStatus) HANYA untuk
// langganan Active; status daur hidup lain (Trial/Cancelled/…) tampil apa adanya
// (badge lifecycle) — derivasi berbasis end_date tak bermakna untuk status
// terminal. CSM (owner) diresolusi dari peta anggota.
func subRowView(s db.ListSubscriptionsRow, names map[int64]string, businessRole string, now time.Time) panel.SubRow {
	label, cls := subDerivedStatus(s.Status, s.EndDate, now)
	return panel.SubRow{
		ID:          s.ID,
		Village:     s.VillageName,
		Plan:        s.PlanName,
		Status:      label,
		StatusClass: cls,
		MRR:         maskARR(formatRupiah(s.Mrr), businessRole),
		Renewal:     dateStr(s.EndDate),
		CSM:         ownerName(s.SubscriptionOwner, names),
	}
}

// subDerivedStatus = Status kolom daftar langganan (BL-95). Untuk langganan
// Active, DERIVASI dari end_date (reuse logika BL-94 agar konsisten): lewat tempo
// → "Masa Tenggang" (error), ≤ dueSoonDays (30) → "Jatuh Tempo" (warning), selain
// itu → "Aman" (success). Status daur hidup NON-Active (Trial/PendingApproval/
// Expired/Cancelled/Churned) dikembalikan apa adanya dgn badge lifecycle
// (subStatusLabelClass) — derivasi timing renewal tak bermakna untuk status
// terminal (mis. Cancelled ber-end_date lampau ≠ "Masa Tenggang"). Mengembalikan
// label + class badge daisyUI (token semantik).
func subDerivedStatus(status string, end pgtype.Date, now time.Time) (label, badgeClass string) {
	if status != "Active" {
		return status, subStatusLifecycleClass(status)
	}
	if !end.Valid {
		return "Aman", "badge badge-success"
	}
	a := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Time.Year(), end.Time.Month(), end.Time.Day(), 0, 0, 0, 0, time.UTC)
	d := int(b.Sub(a).Hours() / 24)
	switch {
	case d < 0:
		return "Masa Tenggang", "badge badge-error"
	case d <= dueSoonDays:
		return "Jatuh Tempo", "badge badge-warning"
	default:
		return "Aman", "badge badge-success"
	}
}

// subStatusLifecycleClass = badge daisyUI (token semantik) untuk status daur hidup
// langganan NON-Active pada kolom Status daftar (BL-95). Dipindah dari
// panel.subStatusBadge saat Status jadi derivasi: view kini menerima class jadi
// dari handler. Active tak lewat sini (di-derivasi subDerivedStatus).
func subStatusLifecycleClass(status string) string {
	switch status {
	case "Trial":
		return "badge badge-info"
	case "Suspended", "PendingApproval":
		return "badge badge-warning"
	case "Expired", "Cancelled", "Churned":
		return "badge badge-error"
	default:
		return "badge badge-ghost"
	}
}
