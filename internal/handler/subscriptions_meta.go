package handler

import (
	"net/http"

	"go_starter/internal/ui/pages/panel"
)

// subscriptions_meta.go — forbidden render & whitelist sort utk SubscriptionsList
// (dipindah dari subscriptions_page.go krn ambang File Health). Berkas TERPISAH
// dari subscriptions_view.go krn subscriptions_view.go sudah dekat ambang
// (gerbang F2/F4 crm:subscriptions) — menyatukan akan melewati 150 baris.

// renderSubscriptionsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderSubscriptionsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Subscription Lists", "/subscriptions",
		panel.SalesForbidden("Subscription Lists"))
}

// subSortableColumns = whitelist kolom yang boleh diminta lewat ?sort= (BL-157a
// + lanjutan: seluruh 6 kolom tabel Subscription Lists). ?sort= di luar
// daftar ini diperlakukan seolah absen (jatuh ke default created_at DESC),
// TAK error.
var subSortableColumns = map[string]bool{
	"village": true,
	"plan":    true,
	"mrr":     true,
	"status":  true,
	"renewal": true,
	"csm":     true,
}
