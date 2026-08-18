package handler

import (
	"net/http"
	"strconv"
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
	{"due", "Jatuh Tempo"},
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

	cursorAt, cursorID := pageCursor(r)
	now := time.Now().In(appTZ)
	today := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
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
	shown, nextCursor := splitPage(rows, func(s db.ListRenewalsRow) (pgtype.Timestamptz, int64) {
		return s.CreatedAt, s.ID
	})
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(s, now, br))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Renewals", "/subscriptions/renewals",
		panel.RenewalsList(panel.RenewalsView{
			Base:       base,
			Window:     window,
			Windows:    renewalWindowOptions(),
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Items:      items,
			NextCursor: nextCursor,
		}))
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

// renewalRowView memetakan satu baris → baris tabel Renewals. Days Left dihitung
// relatif "hari ini" (zona waktu app). Prev→Current = previous_value → MRR,
// keduanya nilai komersial → maskARR (F4, diperbaiki audit FLS M9-1). Kolom
// mengikuti wireframe 5.2 (tanpa kolom pemilik).
func renewalRowView(s db.ListRenewalsRow, now time.Time, businessRole string) panel.RenewalRow {
	return panel.RenewalRow{
		ID:          s.ID,
		Village:     s.VillageName,
		Plan:        s.PlanName,
		RenewalDate: dateStr(s.EndDate),
		DaysLeft:    daysLeftLabel(now, s.EndDate),
		Type:        deref(s.RenewalType),
		Status:      s.Status,
		PrevValue:   maskARR(formatRupiah(s.PreviousValue), businessRole),
		CurrentMRR:  maskARR(formatRupiah(s.Mrr), businessRole),
	}
}

// daysLeftLabel = selisih hari (kalender) end_date terhadap hari ini, diformat.
// Keduanya dinormalkan ke tanggal sipil (UTC midnight) agar bebas jam/zona →
// selisih bulat hari. Query sudah menjamin end_date terisi, tapi tetap fail-soft.
func daysLeftLabel(now time.Time, end pgtype.Date) string {
	if !end.Valid {
		return "—"
	}
	a := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(end.Time.Year(), end.Time.Month(), end.Time.Day(), 0, 0, 0, 0, time.UTC)
	d := int(b.Sub(a).Hours() / 24)
	switch {
	case d > 0:
		return strconv.Itoa(d) + " hari"
	case d == 0:
		return "Hari ini"
	default:
		return strconv.Itoa(-d) + " hari telat"
	}
}
