package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_leads_page.go — HALAMAN daftar Lead (dengan tab) + gerbang forbidden
// bersama. Detail ada di sales_leads_detail.go (dipecah lebih lanjut untuk file
// health, package sama). Aksi ada di sales_leads.go/sales_leads_update.go/
// sales_leads_helpers.go. Dipisah karena keduanya tumbuh dengan aturan berbeda:
// halaman soal APA yang boleh DILIHAT & oleh SIAPA (F2 read + F3 ownership + F4
// masking), aksi soal apa yang boleh DIUBAH (F2 write). Meniru accounts_page.go.
//
// leadSortableColumns/renderLeadsForbidden/leadRowView di sales_leads_view.go;
// fungsi sort per-kolom di sales_leads_sort_a.go, sales_leads_sort_b.go &
// sales_leads_sort_c.go (dipecah krn ambang File Health).

// leadTab menerjemahkan ?tab= → dua sumbu filter ListLeads yang ORTOGONAL dari
// ownership: mine_only (paksa lead_owner=uid walau aktor ScopeAll) & status_filter.
// Tab default "" = semua yang boleh dilihat aktor dalam cakupan F3-nya.
func leadTab(tab string) (mineOnly bool, statusFilter string) {
	switch tab {
	case "my":
		return true, ""
	case "unqualified":
		return false, "Unqualified"
	default: // "all" / ""
		return false, ""
	}
}

// LeadsList — GET /w/{workspace}/leads. Daftar lead, keyset + filter kepemilikan
// F3 + tab. Bukan pemegang peran CRM → 403 + penjelasan (BUKAN 404: penerimanya
// terbukti anggota workspace).
func (h *Handler) LeadsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewLeads(ctx) {
		h.renderLeadsForbidden(w, r)
		return
	}

	filter := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	tab := r.URL.Query().Get("tab")
	mineOnly, statusFilter := leadTab(tab)
	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas F3+tab, tak melebarkan.
	// Trim agar spasi belaka ≡ tak mencari (query kosong lolos predikat SQL).
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// sort/dir (BL-157b): whitelist 7 kolom sortable (leadSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !leadSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("leads: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	canARR := canSeeARR(ctx)

	var shown []db.Lead
	var nextCursor string
	var ok bool
	switch sortCol {
	case "name":
		shown, nextCursor, ok = h.leadsSortByName(w, r, ctx, filter, uid, mineOnly, statusFilter, query, dir)
		if !ok {
			return
		}
	case "status":
		shown, nextCursor, ok = h.leadsSortByStatus(w, r, ctx, filter, uid, mineOnly, statusFilter, query, dir)
		if !ok {
			return
		}
	case "code":
		shown, nextCursor, ok = h.leadsSortByCode(w, r, ctx, filter, uid, mineOnly, statusFilter, query, dir)
		if !ok {
			return
		}
	case "source":
		shown, nextCursor, ok = h.leadsSortBySource(w, r, ctx, filter, uid, mineOnly, statusFilter, query, dir)
		if !ok {
			return
		}
	case "rating":
		shown, nextCursor, ok = h.leadsSortByRating(w, r, ctx, filter, uid, mineOnly, statusFilter, query, dir)
		if !ok {
			return
		}
	case "value":
		shown, nextCursor, ok = h.leadsSortByValue(w, r, ctx, filter, uid, mineOnly, statusFilter, query, dir)
		if !ok {
			return
		}
	case "owner":
		shown, nextCursor, ok = h.leadsSortByOwner(w, r, ctx, filter, uid, mineOnly, statusFilter, query, dir, names)
		if !ok {
			return
		}
	default:
		shown, nextCursor, ok = h.leadsSortDefault(w, r, ctx, filter, uid, mineOnly, statusFilter, query)
		if !ok {
			return
		}
	}

	items := make([]panel.LeadRow, 0, len(shown))
	for _, l := range shown {
		items = append(items, leadRowView(l, names, canARR))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Leads", "/leads", panel.LeadsList(panel.LeadsListView{
		Base:       base,
		Items:      items,
		Tab:        tab,
		Query:      query,
		CanWrite:   canWriteLeads(ctx),
		HideMyTab:  filter.IsOwn, // BL-1: cakupan 'own' → "Semua" ≡ "Lead Saya"
		NextCursor: nextCursor,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Msg:        leadsMsg(r.URL.Query().Get("ok")),
		Sort:       sortCol,
		Dir:        dir,
	}))
}

// leadSortableColumns/renderLeadsForbidden/leadRowView di sales_leads_view.go.
