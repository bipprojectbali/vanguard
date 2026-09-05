package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_deals_delete.go — aksi soft-delete Deal, dipisah dari
// sales_deals_stage.go (ukuran file). Perilaku identik; hanya organisasi file
// yang berubah.

// DealDelete — POST /w/{workspace}/deals/{id}/delete. Soft-delete (reversibel di
// DB via deleted_at; jejak audit mencatat siapa & kapan). FK leads.converted_deal_id
// tak putus (baris tetap ada).
func (h *Handler) DealDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedDeal(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteDeal(ctx, db.SoftDeleteDealParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("deals: delete", "err", err)
		wsRedirect(w, r, "/deals/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "deal.delete", session.TenantID(ctx), map[string]string{
		"deal_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/deals", "deleted")
}
