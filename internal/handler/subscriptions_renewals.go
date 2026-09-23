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
//
// Fungsi sort per-kolom (village/plan/date di sort_a.go, type/mrr/default di
// sort_b.go) dipisah krn ambang File Health yang sama.

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
	canARR := canSeeARR(ctx)

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
	var ok bool
	switch sortCol {
	case "village":
		items, nextCursor, ok = h.renewalsSortByVillage(w, r, ctx, filter, uid, window, today, dir, now, canARR)
	case "plan":
		items, nextCursor, ok = h.renewalsSortByPlan(w, r, ctx, filter, uid, window, today, dir, now, canARR)
	case "date":
		items, nextCursor, ok = h.renewalsSortByDate(w, r, ctx, filter, uid, window, today, dir, now, canARR)
	case "type":
		items, nextCursor, ok = h.renewalsSortByType(w, r, ctx, filter, uid, window, today, dir, now, canARR)
	case "mrr":
		items, nextCursor, ok = h.renewalsSortByMrr(w, r, ctx, filter, uid, window, today, dir, now, canARR)
	default:
		items, nextCursor, ok = h.renewalsSortDefault(w, r, ctx, filter, uid, window, today, now, canARR)
	}
	if !ok {
		return
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
