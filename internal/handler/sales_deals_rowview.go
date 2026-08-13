package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_deals_rowview.go — pembantu presentasi kecil bersama (gerbang 403,
// pemetaan baris, hitung win rate) dipakai list & pipeline Deal. Dipecah dari
// sales_deals_page.go semata untuk file health.

// renderDealsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderDealsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Deals", "/deals", panel.SalesForbidden("Deals"))
}

// dealRowView memetakan satu deal → baris/kartu + F4 (amount tersamar untuk
// Support). Owner diresolusi dari peta anggota.
func dealRowView(d db.Deal, names map[int64]string, businessRole string) panel.DealRow {
	return panel.DealRow{
		ID:            d.ID,
		EntityCode:    deref(d.EntityCode),
		DealName:      d.DealName,
		Stage:         d.Stage,
		Amount:        maskARR(formatRupiah(d.Amount), businessRole),
		Probability:   probabilityStr(d.Probability),
		ExpectedClose: dateStr(d.ExpectedCloseDate),
		Owner:         ownerName(d.DealOwner, names),
	}
}

// winRate = won/(won+lost) sebagai persen bulat. Nol deal tertutup → "—" (tak ada
// dasar hitung; menyajikan 0% akan menyesatkan).
func winRate(won, lost int64) string {
	total := won + lost
	if total == 0 {
		return "—"
	}
	return strconv.FormatInt(won*100/total, 10) + "%"
}
