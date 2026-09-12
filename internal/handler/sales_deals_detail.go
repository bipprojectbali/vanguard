package handler

import (
	"errors"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_deals_detail.go — HALAMAN detail satu Deal. Dipisah dari
// sales_deals_page.go (daftar) untuk file health — detail tumbuh dengan
// aturannya sendiri (resolusi account/kontak/quote best-effort).
//
// Rakitan view (dealQuotesPreview/dealDetailView) di sales_deals_detail_view.go.

// dealQuotesPreviewLimit membatasi jumlah quote yang ditampilkan di kartu
// pratinjau pada detail deal. Daftar penuh (berkeyset) ada di
// /deals/{id}/quotes; kartu ini hanya cuplikan teratas.
const dealQuotesPreviewLimit = 5

// DealDetail — GET /w/{workspace}/deals/{id}. Satu deal. Ownership diputuskan di
// sini (filter.Allows) — kembaran per-baris dari list. Di luar cakupan → 404.
// Nama account & kontak utama diresolusi best-effort (satu query masing-masing,
// bukan N+1); gagal → tautan berlabel kode/id, bukan 500.
func (h *Handler) DealDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	d, err := h.q(ctx).GetDeal(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("deals: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), d.DealOwner) {
		http.NotFound(w, r)
		return
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	view := h.dealDetailView(ctx, base, d, names)
	// BL-99: surface umpan balik PRG di halaman tempat form aksi berada. Tanpa ini,
	// gerbang tahap terminal (Closed Won/Lost) yang gagal — win_loss/loss_reason
	// wajib, atau langganan gagal dibuat — redirect ke sini dgn ?err tapi senyap,
	// sehingga penolakan tampak seperti "deal tak tersimpan".
	view.Err = wsErrMsg(r.URL.Query().Get("err"))
	view.Msg = dealsMsg(r.URL.Query().Get("ok"))
	h.renderWorkspaceShell(w, r, d.DealName, "/deals", panel.DealDetail(view))
}
