package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_churn_page.go — dasbor Churn (Menu 5.2/5.4, READ-ONLY). Menyorot
// langganan yang telah berhenti (status Cancelled/Churned) + nilai MRR yang hilang.
// Aksi churn SENGAJA tidak di sini — ditandai dari detail langganan (subscriptions_
// churn.go); dasbor ini hanya baca (sejalan wireframe 5.2/5.4).
//
// Gerbang sama dengan daftar langganan: F2 crm:subscriptions read + F3 ownership
// (subscription_owner) di layer query. MRR Hilang = lost_value_mrr, nilai komersial
// → maskARR (F4, diperbaiki audit FLS M9-1). Keyset created_at DESC (reuse
// pageCursor/splitPage).

// churnTypeTab = satu tab tipe churn (key untuk query + label tampilan). Sumber SATU
// untuk tab view & normalisasi handler; key ” = semua tipe (mencerminkan
// subs_churn_type_chk: Voluntary/Involuntary).
type churnTypeTab struct{ Key, Label string }

var churnTypeTabs = []churnTypeTab{
	{"", "Semua"},
	{"Voluntary", "Sukarela"},
	{"Involuntary", "Terpaksa"},
}

// SubscriptionChurnList — GET /w/{slug}/subscriptions/churn. Dasbor read-only
// langganan berhenti, ter-scope kepemilikan (F3) + filter tipe churn opsional.
func (h *Handler) SubscriptionChurnList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	typeFilter := normalizeChurnTypeFilter(r.URL.Query().Get("type"))
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListChurned(ctx, db.ListChurnedParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		TypeFilter:      typeFilter,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: churn list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(s db.ListChurnedRow) (pgtype.Timestamptz, int64) {
		return s.CreatedAt, s.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: churn members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.ChurnRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, churnRowView(s, names, br))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Churn", "/subscriptions/churn",
		panel.ChurnList(panel.ChurnView{
			Base:       base,
			Type:       typeFilter,
			Types:      churnTypeFilterOptions(),
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Items:      items,
			NextCursor: nextCursor,
			After:      r.URL.Query().Get("after"),
			Trail:      pageTrail(r),
		}))
}

// normalizeChurnTypeFilter memetakan ?type= ke salah satu key sah; nilai asing →
// ” (semua tipe). Key ” sendiri sah (tab "Semua").
func normalizeChurnTypeFilter(v string) string {
	for _, t := range churnTypeTabs {
		if t.Key == v {
			return v
		}
	}
	return ""
}

// churnTypeFilterOptions menyalin daftar tab ke tipe view (handler pemilik enum).
func churnTypeFilterOptions() []panel.ChurnType {
	out := make([]panel.ChurnType, 0, len(churnTypeTabs))
	for _, t := range churnTypeTabs {
		out = append(out, panel.ChurnType{Key: t.Key, Label: t.Label})
	}
	return out
}

// churnRowView memetakan satu baris → baris tabel Churn. MRR Hilang = lost_value_
// mrr, nilai komersial → maskARR (F4). CSM = nama pemilik langganan dari peta
// anggota. Kolom mengikuti wireframe 5.2/5.4.
func churnRowView(s db.ListChurnedRow, names map[int64]string, businessRole string) panel.ChurnRow {
	return panel.ChurnRow{
		ID:        s.ID,
		Village:   s.VillageName,
		Plan:      s.PlanName,
		LostMRR:   maskARR(formatRupiah(s.LostValueMrr), businessRole),
		Reason:    deref(s.ChurnReason),
		Type:      deref(s.ChurnType),
		ChurnDate: dateStr(s.CancellationDate),
		CSM:       ownerName(s.SubscriptionOwner, names),
	}
}
