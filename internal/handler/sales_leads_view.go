package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_leads_view.go — whitelist sort, gerbang forbidden & row-mapping utk
// LeadsList, dipisah dari sales_leads_page.go krn ambang File Health
// (Route/Handler 150 baris).

// leadSortableColumns = whitelist kolom yang boleh diminta lewat ?sort= (BL-157b:
// 7 kolom tabel Leads). ?sort= di luar daftar ini diperlakukan seolah absen
// (jatuh ke default created_at DESC), TAK error.
var leadSortableColumns = map[string]bool{
	"code":   true,
	"name":   true,
	"source": true,
	"status": true,
	"rating": true,
	"value":  true,
	"owner":  true,
}

// renderLeadsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM. Status
// ditulis SEBELUM body agar penolakan tak terkirim sebagai 200 palsu.
func (h *Handler) renderLeadsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Leads", "/leads", panel.SalesForbidden("Leads"))
}

// leadRowView memetakan satu baris daftar + F4 (estimated_value tersamar bagi
// pemanggil tanpa kapabilitas crm:subscriptions/arr, BL-169). Nama pemilik
// diresolusi dari peta anggota (bukan id telanjang). canARR dihitung SEKALI
// oleh pemanggil (canSeeARR(ctx)).
func leadRowView(l db.Lead, names map[int64]string, canARR bool) panel.LeadRow {
	return panel.LeadRow{
		ID:            l.ID,
		EntityCode:    deref(l.EntityCode),
		LeadName:      l.LeadName,
		ContactPerson: deref(l.ContactPerson),
		LeadSource:    deref(l.LeadSource),
		Status:        l.LeadStatus,
		Rating:        deref(l.Rating),
		EstValue:      maskARR(formatRupiah(l.EstimatedValue), canARR),
		Owner:         ownerName(l.LeadOwner, names),
	}
}
