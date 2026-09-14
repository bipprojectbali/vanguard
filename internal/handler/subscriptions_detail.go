package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// subscriptions_detail.go — HALAMAN baca satu langganan + riwayat rantai renewal
// (M5-3b, GET-only). Ownership (F3) diputuskan di sini atas baris (filter.Allows)
// — kembaran per-baris dari daftar; di luar cakupan → 404 (bukan 403: keberadaan
// baris pun tak diungkap). Meniru DealDetail. Renew/churn menyusul slice lain.
//
// Pemetaan baris (renewalChainView, subItemRows) & resolusi label plan
// (planLabel) di subscriptions_detail_rows.go.

// SubscriptionDetail — GET /w/{workspace}/subscriptions/{id}. Satu langganan.
// Nama plan & desa diresolusi best-effort (satu query masing-masing, bukan N+1);
// gagal → label cadangan, bukan 500.
func (h *Handler) SubscriptionDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	s, err := h.q(ctx).GetSubscription(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("subscriptions: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), s.SubscriptionOwner) {
		http.NotFound(w, r)
		return
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	title := deref(s.EntityCode)
	if title == "" {
		title = "Langganan #" + strconv.FormatInt(s.ID, 10)
	}
	view := h.subDetailView(ctx, base, s, names)
	view.Msg = subscriptionsMsg(r.URL.Query().Get("ok"))
	view.Err = wsErrMsg(r.URL.Query().Get("err"))
	h.renderWorkspaceShell(w, r, title, "/subscriptions", panel.SubDetail(view))
}
